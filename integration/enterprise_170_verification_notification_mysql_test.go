//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/access/domain"
	accessnotification "github.com/hvritual/biz/internal/access/infrastructure/notification"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"gorm.io/gorm"
)

func enterprise170Policy() domain.VerificationPolicy {
	return domain.VerificationPolicy{
		CodeTTL:              5 * time.Minute,
		AuthorizationTTL:     5 * time.Minute,
		ResendInterval:       time.Minute,
		SendLimitWindow:      24 * time.Hour,
		MaxSendsPerWindow:    5,
		MaxVerificationTries: 3,
		CodeDigits:           6,
	}
}

func enterprise170Repository(t *testing.T, db *gorm.DB) *accesspersistence.VerificationRepository {
	t.Helper()
	repository := enterprise170RepositoryWithoutSchema(t, db)
	if err := repository.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	return repository
}

func enterprise170RepositoryWithoutSchema(t *testing.T, db *gorm.DB) *accesspersistence.VerificationRepository {
	t.Helper()
	protection, err := accesspersistence.NewVerificationProtection(accesspersistence.VerificationProtectionConfig{
		ActiveVersion: "v1",
		Keys:          map[string][]byte{"v1": []byte(strings.Repeat("K", 32))},
		HMACKey:       []byte(strings.Repeat("H", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	repository, err := accesspersistence.NewVerificationRepository(db, protection)
	if err != nil {
		t.Fatal(err)
	}
	return repository
}

func TestEnterprise170VerificationHappyPathReplayTamperAndAtomicConsumption(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	repository := enterprise170Repository(t, db)
	sender := accessnotification.NewMemorySender()
	service, err := accessapp.NewVerificationService(repository, sender, enterprise170Policy())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	request := domain.VerificationChallengeRequest{
		BusinessEventID: "enterprise170-login-event",
		FlowID:          "login-flow-170",
		Purpose:         domain.VerificationPurposeLogin,
		UserID:          "user-170",
		TenantID:        "tenant-170",
		Channel:         domain.SecurityNotificationEmail,
		Destination:     "User170@Example.Invalid",
	}

	challenge, err := repository.CreateVerificationChallenge(ctx, request, enterprise170Policy())
	if err != nil {
		t.Fatal(err)
	}
	if challenge.ChallengeID == "" || challenge.NotificationEventID == "" || challenge.DeliveryState != domain.NotificationStatePending {
		t.Fatalf("invalid challenge receipt: %+v", challenge)
	}
	var pending struct {
		DestinationCiphertext string
		SecretCiphertext      string
		State                 string
	}
	if err := db.Table("biz_security_notification_outbox").
		Select("destination_ciphertext, secret_ciphertext, state").
		Where("event_id = ?", challenge.NotificationEventID).Scan(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending.State != domain.NotificationStatePending || pending.DestinationCiphertext == "" || pending.SecretCiphertext == "" {
		t.Fatalf("pending notification did not retain protected delivery material: %+v", pending)
	}
	if strings.Contains(pending.DestinationCiphertext, "user170@example.invalid") {
		t.Fatal("outbox destination is plaintext")
	}

	delivered, err := service.DeliverSecurityNotification(ctx, challenge.NotificationEventID)
	if err != nil {
		t.Fatal(err)
	}
	if delivered.State != domain.NotificationStateDelivered || delivered.ProviderReceipt == "" {
		t.Fatalf("delivery receipt is not delivered: %+v", delivered)
	}
	message, ok := sender.Message(challenge.NotificationEventID)
	if !ok || message.Secret == "" || message.Destination != "user170@example.invalid" {
		t.Fatalf("test adapter did not receive normalized protected payload: %+v %v", message, ok)
	}
	if len(message.Secret) != enterprise170Policy().CodeDigits {
		t.Fatalf("unexpected OTP length: %q", message.Secret)
	}
	if pending.SecretCiphertext == message.Secret || strings.Contains(pending.SecretCiphertext, message.Secret) {
		t.Fatal("OTP appeared in outbox plaintext")
	}
	var storedCodeHash string
	if err := db.Table("biz_verification_challenges").Select("code_hash").Where("challenge_id = ?", challenge.ChallengeID).Scan(&storedCodeHash).Error; err != nil {
		t.Fatal(err)
	}
	if storedCodeHash == "" || storedCodeHash == message.Secret || strings.Contains(storedCodeHash, message.Secret) {
		t.Fatal("OTP appeared in challenge storage")
	}
	var cleared struct{ DestinationCiphertext, SecretCiphertext string }
	if err := db.Table("biz_security_notification_outbox").
		Select("destination_ciphertext, secret_ciphertext").Where("event_id = ?", challenge.NotificationEventID).Scan(&cleared).Error; err != nil {
		t.Fatal(err)
	}
	if cleared.DestinationCiphertext != "" || cleared.SecretCiphertext != "" {
		t.Fatalf("successful delivery retained reversible secrets: %+v", cleared)
	}

	duplicate, err := repository.CreateVerificationChallenge(ctx, request, enterprise170Policy())
	if err != nil {
		t.Fatal(err)
	}
	if duplicate.ChallengeID != challenge.ChallengeID || duplicate.NotificationEventID != challenge.NotificationEventID {
		t.Fatalf("duplicate business event created new logical delivery: old=%+v new=%+v", challenge, duplicate)
	}
	if _, err := service.DeliverSecurityNotification(ctx, duplicate.NotificationEventID); err != nil {
		t.Fatal(err)
	}
	if sender.Count() != 1 {
		t.Fatalf("duplicate delivery reached sender %d times", sender.Count())
	}

	tampered := domain.VerifyChallengeRequest{
		ChallengeID: challenge.ChallengeID, FlowID: request.FlowID,
		Purpose: domain.VerificationPurposePasswordRecovery,
		UserID:  request.UserID, TenantID: request.TenantID, Channel: request.Channel,
		Destination: request.Destination, Code: message.Secret,
	}
	if _, err := service.VerifyCode(ctx, tampered); !errors.Is(err, domain.ErrVerificationInvalid) {
		t.Fatalf("purpose substitution accepted: %v", err)
	}
	verify := tampered
	verify.Purpose = request.Purpose
	authorization, err := service.VerifyCode(ctx, verify)
	if err != nil || authorization.Code == "" {
		t.Fatalf("correct OTP did not mint one-time authorization: %+v %v", authorization, err)
	}
	if _, err := service.VerifyCode(ctx, verify); !errors.Is(err, domain.ErrVerificationConsumed) {
		t.Fatalf("OTP replay accepted: %v", err)
	}

	consume := domain.ConsumeAuthorizationRequest{
		Code: authorization.Code, FlowID: request.FlowID, Purpose: request.Purpose,
		UserID: request.UserID, TenantID: request.TenantID, Channel: request.Channel, Destination: request.Destination,
	}
	tamperedConsume := consume
	tamperedConsume.TenantID = "tenant-other"
	if _, err := service.ConsumeAuthorization(ctx, tamperedConsume); !errors.Is(err, domain.ErrVerificationInvalid) {
		t.Fatalf("tenant substitution consumed authorization: %v", err)
	}

	var success atomic.Int32
	var consumed atomic.Int32
	var other atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.ConsumeAuthorization(context.Background(), consume)
			switch {
			case err == nil:
				success.Add(1)
			case errors.Is(err, domain.ErrVerificationConsumed):
				consumed.Add(1)
			default:
				other.Add(1)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 || consumed.Load() != 1 || other.Load() != 0 {
		t.Fatalf("authorization concurrent consumption: success=%d consumed=%d other=%d", success.Load(), consumed.Load(), other.Load())
	}
	if _, err := service.ConsumeAuthorization(ctx, consume); !errors.Is(err, domain.ErrVerificationConsumed) {
		t.Fatalf("authorization replay accepted: %v", err)
	}

	expiryRequest := domain.VerificationChallengeRequest{
		BusinessEventID: "enterprise170-auth-expiry", FlowID: "auth-expiry-flow",
		Purpose: domain.VerificationPurposeContactChange, UserID: "auth-expiry-user",
		Channel: domain.SecurityNotificationEmail, Destination: "auth.expiry@example.invalid",
	}
	expiryChallenge, expiryDelivery, err := service.SendVerificationCode(ctx, expiryRequest)
	if err != nil || expiryDelivery.State != domain.NotificationStatePending {
		t.Fatalf("authorization expiry fixture was not queued: %+v %v", expiryDelivery, err)
	}
	expiryDelivery, err = service.DeliverSecurityNotification(ctx, expiryChallenge.NotificationEventID)
	if err != nil || expiryDelivery.State != domain.NotificationStateDelivered {
		t.Fatalf("authorization expiry fixture delivery failed: %+v %v", expiryDelivery, err)
	}
	expiryMessage, _ := sender.Message(expiryChallenge.NotificationEventID)
	expiringAuthorization, err := service.VerifyCode(ctx, domain.VerifyChallengeRequest{
		ChallengeID: expiryChallenge.ChallengeID, FlowID: expiryRequest.FlowID, Purpose: expiryRequest.Purpose,
		UserID: expiryRequest.UserID, Channel: expiryRequest.Channel, Destination: expiryRequest.Destination, Code: expiryMessage.Secret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE biz_one_time_authorizations SET expires_at=DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 SECOND) WHERE challenge_id=?", expiryChallenge.ChallengeID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConsumeAuthorization(ctx, domain.ConsumeAuthorizationRequest{
		Code: expiringAuthorization.Code, FlowID: expiryRequest.FlowID, Purpose: expiryRequest.Purpose,
		UserID: expiryRequest.UserID, Channel: expiryRequest.Channel, Destination: expiryRequest.Destination,
	}); !errors.Is(err, domain.ErrVerificationExpired) {
		t.Fatalf("expired one-time authorization accepted: %v", err)
	}
}

func TestEnterprise170LimitsFailureIdempotencyRollbackAndExpiry(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	repository := enterprise170Repository(t, db)
	sender := accessnotification.NewMemorySender()
	policy := enterprise170Policy()
	service, err := accessapp.NewVerificationService(repository, sender, policy)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	base := domain.VerificationChallengeRequest{
		BusinessEventID: "enterprise170-limit-1", FlowID: "limit-flow",
		Purpose: domain.VerificationPurposePasswordRecovery, UserID: "limit-user",
		Channel: domain.SecurityNotificationEmail, Destination: "limit@example.invalid",
	}
	first, err := repository.CreateVerificationChallenge(ctx, base, policy)
	if err != nil {
		t.Fatal(err)
	}
	second := base
	second.BusinessEventID = "enterprise170-limit-2"
	if _, err := repository.CreateVerificationChallenge(ctx, second, policy); !errors.Is(err, domain.ErrVerificationRateLimited) {
		t.Fatalf("60-second resend limit not enforced: %v", err)
	} else {
		var rate domain.RateLimitError
		if !errors.As(err, &rate) || rate.RetryAfter <= 0 {
			t.Fatalf("rate limit lacks retry receipt: %v", err)
		}
	}

	if _, err := service.DeliverSecurityNotification(ctx, first.NotificationEventID); err != nil {
		t.Fatal(err)
	}
	firstMessage, _ := sender.Message(first.NotificationEventID)
	wrongCode := "000000"
	if firstMessage.Secret == wrongCode {
		wrongCode = "111111"
	}
	for attempt := 1; attempt <= policy.MaxVerificationTries; attempt++ {
		_, err := service.VerifyCode(ctx, domain.VerifyChallengeRequest{
			ChallengeID: first.ChallengeID, FlowID: base.FlowID, Purpose: base.Purpose,
			UserID: base.UserID, Channel: base.Channel, Destination: base.Destination, Code: wrongCode,
		})
		if attempt < policy.MaxVerificationTries && !errors.Is(err, domain.ErrVerificationInvalid) {
			t.Fatalf("wrong attempt %d returned %v", attempt, err)
		}
		if attempt == policy.MaxVerificationTries && !errors.Is(err, domain.ErrVerificationRateLimited) {
			t.Fatalf("attempt limit did not rate-limit: %v", err)
		}
	}
	if _, err := service.VerifyCode(ctx, domain.VerifyChallengeRequest{
		ChallengeID: first.ChallengeID, FlowID: base.FlowID, Purpose: base.Purpose,
		UserID: base.UserID, Channel: base.Channel, Destination: base.Destination, Code: firstMessage.Secret,
	}); !errors.Is(err, domain.ErrVerificationRateLimited) {
		t.Fatalf("correct OTP bypassed exhausted attempt budget: %v", err)
	}

	windowPolicy := policy
	windowPolicy.MaxSendsPerWindow = 2
	for index := 0; index < 2; index++ {
		request := domain.VerificationChallengeRequest{
			BusinessEventID: "enterprise170-window-" + string(rune('a'+index)),
			FlowID:          "window-flow", Purpose: domain.VerificationPurposeContactChange,
			UserID: "window-user", Channel: domain.SecurityNotificationSMS, Destination: "+491701234567",
		}
		receipt, err := repository.CreateVerificationChallenge(ctx, request, windowPolicy)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("UPDATE biz_verification_challenges SET created_at=DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 61 SECOND) WHERE challenge_id=?", receipt.ChallengeID).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repository.CreateVerificationChallenge(ctx, domain.VerificationChallengeRequest{
		BusinessEventID: "enterprise170-window-c", FlowID: "window-flow",
		Purpose: domain.VerificationPurposeContactChange, UserID: "window-user",
		Channel: domain.SecurityNotificationSMS, Destination: "+491701234567",
	}, windowPolicy); !errors.Is(err, domain.ErrVerificationRateLimited) {
		t.Fatalf("send-window limit not enforced: %v", err)
	}

	expiryPolicy := policy
	expiryPolicy.CodeTTL = time.Minute
	expiryRequest := domain.VerificationChallengeRequest{
		BusinessEventID: "enterprise170-expired", FlowID: "expiry-flow",
		Purpose: domain.VerificationPurposeAccountDeletion, UserID: "expiry-user",
		Channel: domain.SecurityNotificationEmail, Destination: "expiry@example.invalid",
	}
	expiryChallenge, expiryDelivery, err := service.SendVerificationCode(ctx, expiryRequest)
	if err != nil || expiryDelivery.State != domain.NotificationStatePending {
		t.Fatalf("expiry fixture was not queued: %+v %v", expiryDelivery, err)
	}
	expiryDelivery, err = service.DeliverSecurityNotification(ctx, expiryChallenge.NotificationEventID)
	if err != nil || expiryDelivery.State != domain.NotificationStateDelivered {
		t.Fatalf("expiry fixture delivery failed: %+v %v", expiryDelivery, err)
	}
	expiryMessage, _ := sender.Message(expiryChallenge.NotificationEventID)
	if err := db.Exec("UPDATE biz_verification_challenges SET expires_at=DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 SECOND) WHERE challenge_id=?", expiryChallenge.ChallengeID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.VerifyCode(ctx, domain.VerifyChallengeRequest{
		ChallengeID: expiryChallenge.ChallengeID, FlowID: expiryRequest.FlowID, Purpose: expiryRequest.Purpose,
		UserID: expiryRequest.UserID, Channel: expiryRequest.Channel, Destination: expiryRequest.Destination, Code: expiryMessage.Secret,
	}); !errors.Is(err, domain.ErrVerificationExpired) {
		t.Fatalf("expired OTP accepted: %v", err)
	}

	failingSender := accessnotification.NewMemorySender()
	failingSender.SetFailure(errors.New("test channel unavailable"))
	failingService, err := accessapp.NewVerificationService(repository, failingSender, policy)
	if err != nil {
		t.Fatal(err)
	}
	failureChallenge, failureDelivery, sendErr := failingService.SendVerificationCode(ctx, domain.VerificationChallengeRequest{
		BusinessEventID: "enterprise170-delivery-failure", FlowID: "failure-flow",
		Purpose: domain.VerificationPurposeLogin, UserID: "failure-user",
		Channel: domain.SecurityNotificationEmail, Destination: "failure@example.invalid",
	})
	if sendErr != nil || failureDelivery.State != domain.NotificationStatePending {
		t.Fatalf("failed-channel fixture was not queued: challenge=%+v delivery=%+v err=%v", failureChallenge, failureDelivery, sendErr)
	}
	failureDelivery, sendErr = failingService.DeliverSecurityNotification(ctx, failureChallenge.NotificationEventID)
	if sendErr == nil || failureDelivery.State != domain.NotificationStateFailed || failureDelivery.FailureCode != "DELIVERY_FAILED" {
		t.Fatalf("failed channel reported sent: challenge=%+v delivery=%+v err=%v", failureChallenge, failureDelivery, sendErr)
	}
	var protectedFailure struct{ DestinationCiphertext, SecretCiphertext string }
	if err := db.Table("biz_security_notification_outbox").Select("destination_ciphertext, secret_ciphertext").
		Where("event_id = ?", failureChallenge.NotificationEventID).Scan(&protectedFailure).Error; err != nil {
		t.Fatal(err)
	}
	if protectedFailure.DestinationCiphertext == "" || protectedFailure.SecretCiphertext == "" || strings.Contains(protectedFailure.DestinationCiphertext, "failure@example.invalid") {
		t.Fatalf("failed delivery did not retain only protected retry material: %+v", protectedFailure)
	}

	countBefore := sender.Count()
	rollbackErr := db.Transaction(func(tx *gorm.DB) error {
		txRepository := enterprise170RepositoryWithoutSchema(t, tx)
		_, err := txRepository.EnqueueSecurityNotification(ctx, domain.SecurityNotificationRequest{
			BusinessEventID: "enterprise170-rollback-event", Kind: domain.SecurityNotificationInitialCredential,
			Purpose: domain.VerificationPurposeLogin, UserID: "rollback-user", FlowID: "rollback-flow",
			Channel: domain.SecurityNotificationEmail, Destination: "rollback@example.invalid",
			Secret: "Temporary-Secret-170!", ExpiresAt: time.Now().UTC().Add(time.Hour),
		})
		if err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if rollbackErr == nil {
		t.Fatal("rollback fixture did not roll back")
	}
	var rollbackCount int64
	if err := db.Table("biz_security_notification_outbox").Where("business_event_id = ?", "enterprise170-rollback-event").Count(&rollbackCount).Error; err != nil {
		t.Fatal(err)
	}
	if rollbackCount != 0 || sender.Count() != countBefore {
		t.Fatalf("rollback produced executable side effect: rows=%d sender=%d/%d", rollbackCount, sender.Count(), countBefore)
	}

	tamperNotification := domain.SecurityNotificationRequest{
		BusinessEventID: "enterprise170-tampered-outbox", Kind: domain.SecurityNotificationLoginLock,
		Purpose: domain.VerificationPurposeLogin, UserID: "tamper-user", FlowID: "tamper-flow",
		Channel: domain.SecurityNotificationEmail, Destination: "tamper@example.invalid",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	tamperReceipt, err := service.QueueSecurityNotification(ctx, tamperNotification)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE biz_security_notification_outbox SET purpose=? WHERE event_id=?", string(domain.VerificationPurposeAccountDeletion), tamperReceipt.EventID).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeliverSecurityNotification(ctx, tamperReceipt.EventID); !errors.Is(err, accesspersistence.ErrVerificationCipherCorrupt) {
		t.Fatalf("tampered outbox facts were delivered: %v", err)
	}

	generic := domain.SecurityNotificationRequest{
		BusinessEventID: "enterprise170-initial-credential", Kind: domain.SecurityNotificationInitialCredential,
		Purpose: domain.VerificationPurposeLogin, UserID: "new-user", FlowID: "invite-flow",
		Channel: domain.SecurityNotificationEmail, Destination: "new.user@example.invalid",
		Secret: "OneTime-Initial-Credential!", ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	queued, err := service.QueueSecurityNotification(ctx, generic)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := service.QueueSecurityNotification(ctx, generic)
	if err != nil || duplicate.EventID != queued.EventID {
		t.Fatalf("duplicate business event was not idempotent: %+v %+v %v", queued, duplicate, err)
	}
	beforeGenericDelivery := sender.Count()
	if _, err := service.DeliverSecurityNotification(ctx, queued.EventID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeliverSecurityNotification(ctx, queued.EventID); err != nil {
		t.Fatal(err)
	}
	if sender.Count() != beforeGenericDelivery+1 {
		t.Fatalf("duplicate logical notification delivered more than once: before=%d after=%d", beforeGenericDelivery, sender.Count())
	}
	changed := generic
	changed.Secret = "Different-Secret!"
	if _, err := service.QueueSecurityNotification(ctx, changed); !errors.Is(err, domain.ErrVerificationConflict) {
		t.Fatalf("business event id reuse with changed facts accepted: %v", err)
	}
}

func TestEnterprise170VerificationNotificationMigration(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	path := filepath.Join("..", "internal", "access", "infrastructure", "persistence", "migrations", "0009_enterprise_verification_notification.sql")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(payload)).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"biz_verification_challenges", "biz_one_time_authorizations", "biz_security_notification_outbox"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("migration missing table %s", table)
		}
	}
	for _, index := range []string{"uniq_verification_business_event", "uniq_one_time_authorization_challenge", "uniq_security_notification_business_event"} {
		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND INDEX_NAME=?", index).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Fatalf("migration missing index %s", index)
		}
	}
}
