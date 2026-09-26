//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	accessapp "github.com/hvritual/biz/internal/access/application"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accessnotification "github.com/hvritual/biz/internal/access/infrastructure/notification"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	notificationapp "github.com/hvritual/biz/internal/notification/application"
	"gorm.io/gorm"
	"yunka.io/gateway/authz"
)

func TestEnterprise185IdentityLifecycleOutboxCommitsBeforeReliableWorkerDelivery(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	store, err := accesspersistence.New(db)
	if err != nil { t.Fatal(err) }
	if err = store.AutoMigrate(ctx); err != nil { t.Fatal(err) }
	if err = store.EnsureFirstPartyIDPSchema(ctx); err != nil { t.Fatal(err) }
	if err = store.EnsureFirstPartyIDPSecuritySchema(ctx); err != nil { t.Fatal(err) }
	if err = store.EnsureWebSessionSchema(ctx); err != nil { t.Fatal(err) }

	protection, err := accesspersistence.NewVerificationProtection(accesspersistence.VerificationProtectionConfig{
		ActiveVersion: "v1",
		Keys: map[string][]byte{"v1": []byte(strings.Repeat("K", 32))},
		HMACKey: []byte(strings.Repeat("H", 32)),
	})
	if err != nil { t.Fatal(err) }
	repository, err := accesspersistence.NewVerificationRepository(db, protection)
	if err != nil { t.Fatal(err) }
	if err = repository.EnsureSchema(ctx); err != nil { t.Fatal(err) }
	if err = repository.EnsureReliableSecurityNotificationSchema(ctx); err != nil { t.Fatal(err) }

	sender := accessnotification.NewMemorySender()
	service, err := accessapp.NewVerificationService(repository, sender, enterprise170Policy())
	if err != nil { t.Fatal(err) }
	worker := notificationapp.SecurityDeliveryWorker{
		Repository: repository,
		Sender: sender,
		Policy: accessdomain.EnterpriseNotificationRetryPolicy(),
		WorkerID: "identity-worker-185",
	}
	if err = worker.Validate(); err != nil { t.Fatal(err) }

	tenant, user := "n185-life-tenant", "n185-life-user"
	email := "n185-life@example.invalid"
	if err = store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenant, TenantName: tenant, UserID: user, Email: email, Token: "n185-life-token",
	}, []authz.PermissionKey{"tenant.notification.read"}); err != nil { t.Fatal(err) }
	if err = store.SetUserPassword(ctx, user, "CoffeePass9A"); err != nil { t.Fatal(err) }

	t.Run("verification-request-commits-outbox-before-provider-io", func(t *testing.T) {
		challenge, delivery, err := service.SendVerificationCode(ctx, accessdomain.VerificationChallengeRequest{
			BusinessEventID: "n185-life-recovery-request",
			FlowID: "n185-life-recovery-flow",
			Purpose: accessdomain.VerificationPurposePasswordRecovery,
			UserID: user,
			Channel: accessdomain.SecurityNotificationEmail,
			Destination: email,
		})
		if err != nil || challenge.ChallengeID == "" || delivery.State != accessdomain.NotificationStatePending || sender.Count() != 0 {
			t.Fatalf("challenge=%+v delivery=%+v sender=%d err=%v", challenge, delivery, sender.Count(), err)
		}
		result, err := worker.RunOnce(ctx)
		if err != nil || result.EventID != challenge.NotificationEventID || result.State != accessdomain.NotificationStateDelivered || sender.Count() != 1 {
			t.Fatalf("worker result=%+v sender=%d err=%v", result, sender.Count(), err)
		}
		message, ok := sender.Message(challenge.NotificationEventID)
		if !ok || message.Secret == "" || message.Destination != email {
			t.Fatalf("delivered OTP evidence missing: %+v ok=%v", message, ok)
		}

		receipt, err := store.RecoverPasswordWithCode(ctx, protection, accessdomain.VerifyChallengeRequest{
			ChallengeID: challenge.ChallengeID,
			FlowID: "n185-life-recovery-flow",
			Purpose: accessdomain.VerificationPurposePasswordRecovery,
			UserID: user,
			Channel: accessdomain.SecurityNotificationEmail,
			Destination: email,
			Code: message.Secret,
		}, 5*time.Minute, "CoffeePass9B", "CoffeePass9B")
		if err != nil || receipt.ChallengeID != challenge.ChallengeID {
			t.Fatalf("password recovery receipt=%+v err=%v", receipt, err)
		}
		var reset struct {
			EventID string
			State string
			DestinationCiphertext string
		}
		if err := db.Table("biz_security_notification_outbox").
			Select("event_id,state,destination_ciphertext").
			Where("business_event_id=? AND kind=?", "password-reset-complete/"+challenge.ChallengeID, string(accessdomain.SecurityNotificationPasswordReset)).
			Take(&reset).Error; err != nil { t.Fatal(err) }
		if reset.EventID == "" || reset.State != accessdomain.NotificationStatePending || reset.DestinationCiphertext == "" || sender.Count() != 1 {
			t.Fatalf("password reset outbox=%+v sender=%d", reset, sender.Count())
		}
		result, err = worker.RunOnce(ctx)
		if err != nil || result.EventID != reset.EventID || result.State != accessdomain.NotificationStateDelivered || sender.Count() != 2 {
			t.Fatalf("password reset worker=%+v sender=%d err=%v", result, sender.Count(), err)
		}
	})

	t.Run("member-lifecycle-stage-rolls-back-with-business-transaction", func(t *testing.T) {
		rollback := errors.New("rollback lifecycle fixture")
		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			notifier, err := accesspersistence.NewTenantMemberLifecycleNotificationRepository(tx, nil, protection)
			if err != nil { return err }
			if _, err = notifier.Notify(ctx, accessports.TenantMemberLifecycleNotificationInput{
				TenantID: tenant, UserID: user, Status: accessdomain.TenantMemberStatusSuspended,
				Reason: "rollback must leave no executable message", Version: 41,
			}); err != nil { return err }
			return rollback
		})
		if !errors.Is(err, rollback) { t.Fatalf("rollback result=%v", err) }
		var count int64
		if err := db.Table("biz_security_notification_outbox").
			Where("business_event_id=?", "member-lifecycle/"+tenant+"/"+user+"/"+accessdomain.TenantMemberStatusSuspended+"/41").
			Count(&count).Error; err != nil { t.Fatal(err) }
		if count != 0 { t.Fatalf("rolled back lifecycle outbox count=%d", count) }
	})

	t.Run("member-lifecycle-commits-before-reliable-worker-delivery", func(t *testing.T) {
		notifier, err := accesspersistence.NewTenantMemberLifecycleNotificationRepository(db, nil, protection)
		if err != nil { t.Fatal(err) }
		delivery, err := notifier.Notify(ctx, accessports.TenantMemberLifecycleNotificationInput{
			TenantID: tenant, UserID: user, Status: accessdomain.TenantMemberStatusSuspended,
			Reason: "qualification lifecycle", Version: 42,
		})
		if err != nil || delivery.State != accessdomain.NotificationStatePending || sender.Count() != 2 {
			t.Fatalf("lifecycle delivery=%+v sender=%d err=%v", delivery, sender.Count(), err)
		}
		result, err := worker.RunOnce(ctx)
		if err != nil || result.EventID != delivery.EventID || result.State != accessdomain.NotificationStateDelivered || sender.Count() != 3 {
			t.Fatalf("lifecycle worker=%+v sender=%d err=%v", result, sender.Count(), err)
		}
		message, ok := sender.Message(delivery.EventID)
		if !ok || !strings.Contains(message.Secret, "qualification lifecycle") {
			t.Fatalf("lifecycle message=%+v ok=%v", message, ok)
		}
	})
}
