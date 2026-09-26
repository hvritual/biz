//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	notificationapp "github.com/hvritual/biz/internal/notification/application"
	"gorm.io/gorm"
)

type identityDeliveryFixture struct {
	enterprise173Fixture
	ctx        context.Context
	repository *accesspersistence.VerificationRepository
	worker     notificationapp.SecurityDeliveryWorker
}

func newIdentityDeliveryFixture(t *testing.T) identityDeliveryFixture {
	t.Helper()
	// Use the existing serial, opt-in fixture workflow; never share sender
	// counters or a pending queue between independent lifecycle scenarios.
	fixture := newEnterprise173Fixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	repository, err := accesspersistence.NewVerificationRepository(fixture.DB, fixture.Protection)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.EnsureReliableSecurityNotificationSchema(ctx); err != nil {
		t.Fatal(err)
	}
	worker := notificationapp.SecurityDeliveryWorker{
		Repository: repository, Sender: fixture.Sender,
		Policy: accessdomain.EnterpriseNotificationRetryPolicy(), WorkerID: "identity-worker-185",
	}
	if err := worker.Validate(); err != nil {
		t.Fatal(err)
	}
	return identityDeliveryFixture{enterprise173Fixture: fixture, ctx: ctx, repository: repository, worker: worker}
}

func TestEnterprise185IdentityLifecycleOutboxCommitsBeforeReliableWorkerDelivery(t *testing.T) {
	t.Run("verification-request-commits-outbox-before-provider-io", func(t *testing.T) {
		f := newIdentityDeliveryFixture(t)
		ctx, db := f.ctx, f.DB
		const user, tenant, email = "n185-life-user", "n185-life-tenant", "n185-life@example.invalid"
		const oldPassword, newPassword = "CoffeePass9A", "CoffeePass9B"
		sessionA, sessionB := f.bootstrapAccount(t, user, tenant, email, oldPassword)
		challenge, delivery, err := f.Service.SendVerificationCode(ctx, accessdomain.VerificationChallengeRequest{
			BusinessEventID: "n185-life-recovery-request", FlowID: "n185-life-recovery-flow",
			Purpose: accessdomain.VerificationPurposePasswordRecovery, UserID: user,
			Channel: accessdomain.SecurityNotificationEmail, Destination: email,
		})
		if err != nil || challenge.ChallengeID == "" || delivery.State != accessdomain.NotificationStatePending || f.Sender.Count() != 0 {
			t.Fatalf("verification queue contract failed: state=%s calls=%d err=%v", delivery.State, f.Sender.Count(), err)
		}
		result, err := f.worker.RunOnce(ctx)
		if err != nil || result.EventID != challenge.NotificationEventID || result.State != accessdomain.NotificationStateDelivered || f.Sender.Count() != 1 {
			t.Fatalf("verification worker failed: state=%s calls=%d err=%v", result.State, f.Sender.Count(), err)
		}
		message, ok := f.Sender.Message(challenge.NotificationEventID)
		if !ok || message.Secret == "" || message.Destination != email {
			t.Fatal("delivered OTP evidence missing or mismatched")
		}
		request := accessdomain.VerifyChallengeRequest{
			ChallengeID: challenge.ChallengeID, FlowID: "n185-life-recovery-flow",
			Purpose: accessdomain.VerificationPurposePasswordRecovery, UserID: user,
			Channel: accessdomain.SecurityNotificationEmail, Destination: email, Code: message.Secret,
		}

		// Compare the actual stored authority with the recovery input before any
		// mutation. Report field names/booleans, never OTPs or binding hashes.
		var stored struct {
			Purpose, UserID, TenantID, FlowID, Channel string
			DestinationHash, BindingHash, CodeHash     string
		}
		if err := db.Table("biz_verification_challenges").Select("purpose,user_id,tenant_id,flow_id,channel,destination_hash,binding_hash,code_hash").
			Where("challenge_id=?", challenge.ChallengeID).Take(&stored).Error; err != nil {
			t.Fatal(err)
		}
		destinationHash, _, err := f.Protection.DestinationHash(request.Channel, request.Destination)
		if err != nil {
			t.Fatal("could not derive recovery destination binding")
		}
		checks := map[string]bool{
			"purpose": stored.Purpose == string(request.Purpose), "user": stored.UserID == request.UserID,
			"tenant": stored.TenantID == request.TenantID, "flow": stored.FlowID == request.FlowID,
			"channel": stored.Channel == string(request.Channel), "destination_hash": stored.DestinationHash == destinationHash,
			"binding_hash": stored.BindingHash == f.Protection.BindingHash(request.Purpose, request.UserID, request.TenantID, request.FlowID, request.Channel, destinationHash),
			"code_hash":    stored.CodeHash == f.Protection.HashCode(request.ChallengeID, request.Code),
		}
		for field, matches := range checks {
			if !matches {
				t.Fatalf("recovery binding mismatch: field=%s", field)
			}
		}
		t.Log("recovery authority: purpose/user/tenant/flow/channel/destination_hash/binding_hash/code_hash all match")

		assertUnchanged := func() {
			t.Helper()
			var row struct{ ConsumedAt *time.Time }
			if err := db.Table("biz_verification_challenges").Select("consumed_at").Where("challenge_id=?", request.ChallengeID).Take(&row).Error; err != nil {
				t.Fatal(err)
			}
			var authorizations, completions int64
			if err := db.Table("biz_one_time_authorizations").Where("challenge_id=?", request.ChallengeID).Count(&authorizations).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Table("biz_security_notification_outbox").Where("business_event_id=?", "password-reset-complete/"+request.ChallengeID).Count(&completions).Error; err != nil {
				t.Fatal(err)
			}
			if row.ConsumedAt != nil || authorizations != 0 || completions != 0 || f.Sender.Count() != 1 {
				t.Fatal("failed recovery left a consumed challenge, authorization, completion or external effect")
			}
			if err := f.Store.VerifyUserPassword(ctx, user, oldPassword); err != nil {
				t.Fatal("failed recovery changed the previous password")
			}
			for _, raw := range []string{sessionA, sessionB} {
				if _, err := f.Store.AuthenticateWebSession(ctx, raw); err != nil {
					t.Fatal("failed recovery revoked a session")
				}
			}
		}
		variants := []struct {
			name   string
			change func(*accessdomain.VerifyChallengeRequest)
		}{
			{"purpose", func(r *accessdomain.VerifyChallengeRequest) { r.Purpose = accessdomain.VerificationPurposeLogin }},
			{"user", func(r *accessdomain.VerifyChallengeRequest) { r.UserID = "another-user" }},
			{"tenant", func(r *accessdomain.VerifyChallengeRequest) { r.TenantID = "another-tenant" }},
			{"flow", func(r *accessdomain.VerifyChallengeRequest) { r.FlowID = "another-flow" }},
			{"channel", func(r *accessdomain.VerifyChallengeRequest) {
				r.Channel = accessdomain.SecurityNotificationSMS
				r.Destination = "+491701234567"
			}},
			{"destination", func(r *accessdomain.VerifyChallengeRequest) { r.Destination = "other@example.invalid" }},
			{"code", func(r *accessdomain.VerifyChallengeRequest) {
				r.Code = "000000"
				if request.Code == r.Code {
					r.Code = "111111"
				}
			}},
		}
		for _, variant := range variants {
			if !t.Run("rejects-substituted-"+variant.name, func(t *testing.T) {
				changed := request
				variant.change(&changed)
				if _, err := f.Store.RecoverPasswordWithCode(ctx, f.Protection, changed, 5*time.Minute, newPassword, newPassword); !errors.Is(err, accessdomain.ErrVerificationInvalid) {
					t.Fatalf("substituted recovery input was not rejected: %v", err)
				}
			}) {
				t.Fatal("recovery binding negative failed")
			}
		}
		assertUnchanged()

		// The callback fails only the completion outbox insert, after valid
		// binding, password change, session revoke and challenge consumption.
		injected := errors.New("injected recovery completion insert failure")
		callback := "notification185:recovery-completion"
		hit := false
		if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
			if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "biz_security_notification_outbox" {
				hit = true
				tx.AddError(injected)
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, recoveryErr := f.Store.RecoverPasswordWithCode(ctx, f.Protection, request, 5*time.Minute, newPassword, newPassword)
		if err := db.Callback().Create().Remove(callback); err != nil {
			t.Fatal(err)
		}
		if !hit || !errors.Is(recoveryErr, injected) {
			t.Fatalf("completion-insert boundary not reached: hit=%t err=%v", hit, recoveryErr)
		}
		assertUnchanged()
		t.Log("completion insert failure rolls back password/session/challenge/authorization/outbox atomically")

		receipt, err := f.Store.RecoverPasswordWithCode(ctx, f.Protection, request, 5*time.Minute, newPassword, newPassword)
		if err != nil || receipt.ChallengeID != challenge.ChallengeID {
			t.Fatalf("password recovery retry failed: %v", err)
		}
		if err := f.Store.VerifyUserPassword(ctx, user, newPassword); err != nil {
			t.Fatal("new password was not committed")
		}
		if err := f.Store.VerifyUserPassword(ctx, user, oldPassword); !errors.Is(err, accesspersistence.ErrCurrentPasswordInvalid) {
			t.Fatal("old password still accepted after recovery")
		}
		for _, raw := range []string{sessionA, sessionB} {
			if _, err := f.Store.AuthenticateWebSession(ctx, raw); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
				t.Fatal("old session still accepted after recovery")
			}
		}
		var reset struct{ EventID, Kind, State, DestinationCiphertext, SecretCiphertext string }
		if err := db.Table("biz_security_notification_outbox").Select("event_id,kind,state,destination_ciphertext,secret_ciphertext").
			Where("business_event_id=?", "password-reset-complete/"+challenge.ChallengeID).Take(&reset).Error; err != nil {
			t.Fatal(err)
		}
		if reset.Kind != string(accessdomain.SecurityNotificationPasswordResetCompleted) || reset.State != accessdomain.NotificationStatePending ||
			reset.DestinationCiphertext == "" || reset.SecretCiphertext != "" || f.Sender.Count() != 1 {
			t.Fatal("password completion must be queued without credential material or synchronous delivery")
		}
		result, err = f.worker.RunOnce(ctx)
		if err != nil || result.EventID != reset.EventID || result.State != accessdomain.NotificationStateDelivered || f.Sender.Count() != 2 {
			t.Fatalf("completion worker failed: state=%s calls=%d err=%v", result.State, f.Sender.Count(), err)
		}
		completed, ok := f.Sender.Message(reset.EventID)
		if !ok || completed.Kind != accessdomain.SecurityNotificationPasswordResetCompleted || completed.Secret != "" || completed.Destination != email {
			t.Fatal("completion provider request contained credential material or incorrect metadata")
		}
		if _, err := f.Store.RecoverPasswordWithCode(ctx, f.Protection, request, 5*time.Minute, newPassword, newPassword); !errors.Is(err, accessdomain.ErrVerificationConsumed) {
			t.Fatalf("consumed recovery challenge replay accepted: %v", err)
		}
		var count int64
		if err := db.Table("biz_security_notification_outbox").Where("business_event_id=?", "password-reset-complete/"+challenge.ChallengeID).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		idle, err := f.worker.RunOnce(ctx)
		if err != nil || idle.EventID != "" || count != 1 || f.Sender.Count() != 2 {
			t.Fatal("replay created or sent a second completion notification")
		}
	})

	t.Run("member-lifecycle-stage-rolls-back-with-business-transaction", func(t *testing.T) {
		f := newIdentityDeliveryFixture(t)
		const tenant, user = "n185-rollback-tenant", "n185-rollback-user"
		f.bootstrapAccount(t, user, tenant, "rollback@example.invalid", "CoffeePass9A")
		rollback := errors.New("rollback lifecycle fixture")
		err := f.DB.WithContext(f.ctx).Transaction(func(tx *gorm.DB) error {
			notifier, err := accesspersistence.NewTenantMemberLifecycleNotificationRepository(tx, nil, f.Protection)
			if err != nil {
				return err
			}
			if _, err = notifier.Notify(f.ctx, accessports.TenantMemberLifecycleNotificationInput{
				TenantID: tenant, UserID: user, Status: accessdomain.TenantMemberStatusSuspended,
				Reason: "rollback must leave no executable message", Version: 41,
			}); err != nil {
				return err
			}
			return rollback
		})
		if !errors.Is(err, rollback) {
			t.Fatalf("rollback result=%v", err)
		}
		var count int64
		if err := f.DB.Table("biz_security_notification_outbox").Where("tenant_id=? AND user_id=?", tenant, user).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		idle, err := f.worker.RunOnce(f.ctx)
		if count != 0 || err != nil || idle.EventID != "" || f.Sender.Count() != 0 {
			t.Fatal("rolled back lifecycle produced an executable notification")
		}
	})

	t.Run("member-lifecycle-commits-before-reliable-worker-delivery", func(t *testing.T) {
		f := newIdentityDeliveryFixture(t)
		const tenant, user = "n185-commit-tenant", "n185-commit-user"
		f.bootstrapAccount(t, user, tenant, "committed@example.invalid", "CoffeePass9A")
		var delivery accessdomain.NotificationDeliveryReceipt
		if err := f.DB.WithContext(f.ctx).Transaction(func(tx *gorm.DB) error {
			notifier, err := accesspersistence.NewTenantMemberLifecycleNotificationRepository(tx, nil, f.Protection)
			if err != nil {
				return err
			}
			delivery, err = notifier.Notify(f.ctx, accessports.TenantMemberLifecycleNotificationInput{
				TenantID: tenant, UserID: user, Status: accessdomain.TenantMemberStatusSuspended,
				Reason: "qualification lifecycle", Version: 42,
			})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if delivery.State != accessdomain.NotificationStatePending || f.Sender.Count() != 0 {
			t.Fatal("lifecycle staging did provider I/O")
		}
		result, err := f.worker.RunOnce(f.ctx)
		if err != nil || result.EventID != delivery.EventID || result.State != accessdomain.NotificationStateDelivered || f.Sender.Count() != 1 {
			t.Fatalf("lifecycle worker failed: state=%s calls=%d err=%v", result.State, f.Sender.Count(), err)
		}
		message, ok := f.Sender.Message(delivery.EventID)
		if !ok || !strings.Contains(message.Secret, "qualification lifecycle") {
			t.Fatal("lifecycle provider payload is missing the server-owned reason")
		}
	})

	t.Run("credential-kinds-stay-required-completion-must-be-secretless", func(t *testing.T) {
		f := newIdentityDeliveryFixture(t)
		base := accessdomain.SecurityNotificationRequest{
			BusinessEventID: "n185-material-validation", Kind: accessdomain.SecurityNotificationPasswordResetCompleted,
			Purpose: accessdomain.VerificationPurposePasswordRecovery, UserID: "user",
			Channel: accessdomain.SecurityNotificationEmail, Destination: "user@example.invalid", ExpiresAt: time.Now().Add(time.Hour),
		}
		for _, kind := range []accessdomain.SecurityNotificationKind{accessdomain.SecurityNotificationVerificationCode, accessdomain.SecurityNotificationInitialCredential, accessdomain.SecurityNotificationPasswordReset} {
			request := base
			request.Kind = kind
			if _, err := f.repository.EnqueueSecurityNotification(f.ctx, request); !errors.Is(err, accessdomain.ErrVerificationInvalid) {
				t.Fatal("legacy missing-material guard weakened")
			}
		}
		for _, material := range []string{"NeverTransmit9A", "123456", " "} {
			request := base
			request.Secret = material
			if _, err := f.repository.EnqueueSecurityNotification(f.ctx, request); !errors.Is(err, accessdomain.ErrVerificationInvalid) {
				t.Fatal("completion accepted credential material")
			}
		}
		var count int64
		if err := f.DB.Table("biz_security_notification_outbox").Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 || f.Sender.Count() != 0 {
			t.Fatal("invalid material left notification side effects")
		}
	})
}
