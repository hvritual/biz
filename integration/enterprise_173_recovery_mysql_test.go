//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

func TestEnterprise173PasswordRecoveryReplayExpiryAndConcurrentConsumption(t *testing.T) {
	fixture := newEnterprise173Fixture(t)
	ctx := context.Background()

	t.Run("success and replay", func(t *testing.T) {
		const (
			userID      = "enterprise-173-recovery-user"
			email       = "enterprise173.recovery@example.invalid"
			oldPassword = "LegacyRecover9A"
		)
		sessionA, sessionB := fixture.bootstrapAccount(t, userID, "tenant-173-recovery", email, oldPassword)
		challenge, code := fixture.sendRecoveryOTP(t, "enterprise173/recovery/normal", "recovery-flow-normal", userID, email)

		if _, err := fixture.Store.RecoverPasswordWithCode(ctx, fixture.Protection, domain.VerifyChallengeRequest{
			ChallengeID: challenge.ChallengeID, FlowID: "recovery-flow-normal", Purpose: domain.VerificationPurposePasswordRecovery,
			UserID: userID, Channel: domain.SecurityNotificationEmail, Destination: email, Code: "000000",
		}, 5*time.Minute, "ResetPass9A", "ResetPass9A"); !errors.Is(err, domain.ErrVerificationInvalid) {
			t.Fatalf("wrong OTP accepted: %v", err)
		}
		if _, err := fixture.Store.AuthenticateUserPassword(ctx, email, oldPassword); err != nil {
			t.Fatalf("wrong OTP changed password: %v", err)
		}
		if _, err := fixture.Store.RecoverPasswordWithCode(ctx, fixture.Protection, domain.VerifyChallengeRequest{
			ChallengeID: challenge.ChallengeID, FlowID: "recovery-flow-normal", Purpose: domain.VerificationPurposePasswordRecovery,
			UserID: userID, Channel: domain.SecurityNotificationEmail, Destination: email, Code: code,
		}, 5*time.Minute, "weakpass", "weakpass"); !errors.Is(err, accesspersistence.ErrWeakUserPassword) {
			t.Fatalf("weak recovery password accepted: %v", err)
		}
		receipt, err := fixture.Store.RecoverPasswordWithCode(ctx, fixture.Protection, domain.VerifyChallengeRequest{
			ChallengeID: challenge.ChallengeID, FlowID: "recovery-flow-normal", Purpose: domain.VerificationPurposePasswordRecovery,
			UserID: userID, Channel: domain.SecurityNotificationEmail, Destination: email, Code: code,
		}, 5*time.Minute, "ResetPass9A", "ResetPass9A")
		if err != nil || receipt.ChallengeID != challenge.ChallengeID {
			t.Fatalf("recovery failed: %+v %v", receipt, err)
		}
		if _, err := fixture.Store.AuthenticateUserPassword(ctx, email, oldPassword); !errors.Is(err, accesspersistence.ErrInvalidUserCredentials) {
			t.Fatalf("old password works after recovery: %v", err)
		}
		if _, err := fixture.Store.AuthenticateUserPassword(ctx, email, "ResetPass9A"); err != nil {
			t.Fatalf("recovery password rejected: %v", err)
		}
		for _, raw := range []string{sessionA, sessionB} {
			if _, err := fixture.Store.AuthenticateWebSession(ctx, raw); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
				t.Fatalf("old session survived recovery: %v", err)
			}
		}
		if _, err := fixture.Store.RecoverPasswordWithCode(ctx, fixture.Protection, domain.VerifyChallengeRequest{
			ChallengeID: challenge.ChallengeID, FlowID: "recovery-flow-normal", Purpose: domain.VerificationPurposePasswordRecovery,
			UserID: userID, Channel: domain.SecurityNotificationEmail, Destination: email, Code: code,
		}, 5*time.Minute, "AgainPass9A", "AgainPass9A"); !errors.Is(err, domain.ErrVerificationConsumed) {
			t.Fatalf("recovery replay accepted: %v", err)
		}
		enterprise188RequireAuditOutcome(t, fixture.DB, "tenant-173-recovery", "identity.password.recover", "success", 1)
		enterprise188RequireAuditOutcome(t, fixture.DB, "tenant-173-recovery", "identity.password.recover", "failure", 2)
	})

	t.Run("expired", func(t *testing.T) {
		const (
			userID = "enterprise-173-expired-user"
			email  = "enterprise173.expired@example.invalid"
		)
		fixture.bootstrapAccount(t, userID, "tenant-173-expired", email, "ExpiredOld9A")
		challenge, code := fixture.sendRecoveryOTP(t, "enterprise173/recovery/expired", "recovery-flow-expired", userID, email)
		if err := fixture.DB.Exec("UPDATE biz_verification_challenges SET expires_at=DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 SECOND) WHERE challenge_id=?", challenge.ChallengeID).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.Store.RecoverPasswordWithCode(ctx, fixture.Protection, domain.VerifyChallengeRequest{
			ChallengeID: challenge.ChallengeID, FlowID: "recovery-flow-expired", Purpose: domain.VerificationPurposePasswordRecovery,
			UserID: userID, Channel: domain.SecurityNotificationEmail, Destination: email, Code: code,
		}, 5*time.Minute, "ExpiredNew9A", "ExpiredNew9A"); !errors.Is(err, domain.ErrVerificationExpired) {
			t.Fatalf("expired recovery accepted: %v", err)
		}
		enterprise188RequireAuditOutcome(t, fixture.DB, "tenant-173-expired", "identity.password.recover", "failure", 1)
	})

	t.Run("concurrent single consumption", func(t *testing.T) {
		const (
			userID = "enterprise-173-concurrent-user"
			email  = "enterprise173.concurrent@example.invalid"
		)
		fixture.bootstrapAccount(t, userID, "tenant-173-concurrent", email, "ConcurrentOld9A")
		challenge, code := fixture.sendRecoveryOTP(t, "enterprise173/recovery/concurrent", "recovery-flow-concurrent", userID, email)
		var success atomic.Int32
		var consumed atomic.Int32
		var other atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := fixture.Store.RecoverPasswordWithCode(context.Background(), fixture.Protection, domain.VerifyChallengeRequest{
					ChallengeID: challenge.ChallengeID, FlowID: "recovery-flow-concurrent",
					Purpose: domain.VerificationPurposePasswordRecovery, UserID: userID,
					Channel: domain.SecurityNotificationEmail, Destination: email, Code: code,
				}, 5*time.Minute, "ConcurrentNew9A", "ConcurrentNew9A")
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
			t.Fatalf("concurrent recovery: success=%d consumed=%d other=%d", success.Load(), consumed.Load(), other.Load())
		}
		enterprise188RequireAuditOutcome(t, fixture.DB, "tenant-173-concurrent", "identity.password.recover", "success", 1)
		enterprise188RequireAuditOutcome(t, fixture.DB, "tenant-173-concurrent", "identity.password.recover", "failure", 1)
	})
}

func TestEnterprise191PasswordRecoverySuccessRate(t *testing.T) {
	fixture := newEnterprise173Fixture(t)
	ctx := context.Background()
	const (
		sampleCount = 100
		workerCount = 8
	)
	type recoveryResult struct {
		sample   int
		phase    string
		identity string
		err      error
	}
	runSample := func(sample int) recoveryResult {
		userID := fmt.Sprintf("enterprise191-reset-%03d", sample)
		email := fmt.Sprintf("enterprise191-reset-%03d@example.invalid", sample)
		oldPassword := "OldPass9A"
		newPassword := "NewPass9A"
		if err := fixture.Store.BootstrapGlobalUser(ctx, accesspersistence.GlobalUserBootstrap{ID: userID, Email: email}); err != nil {
			return recoveryResult{sample: sample, phase: "bootstrap", err: err}
		}
		if err := fixture.Store.SetUserPassword(ctx, userID, oldPassword); err != nil {
			return recoveryResult{sample: sample, phase: "set-password", err: err}
		}
		flowID := fmt.Sprintf("enterprise191-reset-flow-%03d", sample)
		challenge, delivery, err := fixture.Service.SendVerificationCode(ctx, domain.VerificationChallengeRequest{
			BusinessEventID: fmt.Sprintf("enterprise191/reset/%03d", sample),
			FlowID:          flowID,
			Purpose:         domain.VerificationPurposePasswordRecovery,
			UserID:          userID,
			Channel:         domain.SecurityNotificationEmail,
			Destination:     email,
		})
		if err != nil || delivery.State != domain.NotificationStatePending {
			return recoveryResult{sample: sample, phase: "queue-otp", err: fmt.Errorf("delivery=%+v err=%w", delivery, err)}
		}
		delivery, err = fixture.Service.DeliverSecurityNotification(ctx, challenge.NotificationEventID)
		if err != nil || delivery.State != domain.NotificationStateDelivered {
			return recoveryResult{sample: sample, phase: "deliver-otp", err: fmt.Errorf("delivery=%+v err=%w", delivery, err)}
		}
		message, ok := fixture.Sender.Message(challenge.NotificationEventID)
		if !ok || message.Secret == "" {
			return recoveryResult{sample: sample, phase: "otp-evidence", err: errors.New("recovery OTP evidence missing")}
		}
		_, err = fixture.Store.RecoverPasswordWithCode(ctx, fixture.Protection, domain.VerifyChallengeRequest{
			ChallengeID: challenge.ChallengeID,
			FlowID:      flowID,
			Purpose:     domain.VerificationPurposePasswordRecovery,
			UserID:      userID,
			Channel:     domain.SecurityNotificationEmail,
			Destination: email,
			Code:        message.Secret,
		}, 5*time.Minute, newPassword, newPassword)
		if err != nil {
			return recoveryResult{sample: sample, phase: "recover", err: err}
		}
		identity, err := fixture.Store.AuthenticateUserPassword(ctx, email, newPassword)
		if err != nil || identity.UserID != userID {
			return recoveryResult{sample: sample, phase: "readback", identity: identity.UserID, err: err}
		}
		return recoveryResult{sample: sample, identity: identity.UserID}
	}

	jobs := make(chan int)
	results := make(chan recoveryResult, sampleCount)
	var wg sync.WaitGroup
	wg.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer wg.Done()
			for sample := range jobs {
				results <- runSample(sample)
			}
		}()
	}
	for sample := 0; sample < sampleCount; sample++ {
		jobs <- sample
	}
	close(jobs)
	wg.Wait()
	close(results)

	successes := 0
	for result := range results {
		if result.err == nil && result.identity == fmt.Sprintf("enterprise191-reset-%03d", result.sample) {
			successes++
			continue
		}
		t.Logf("ENTERPRISE191_RESET_FAILURE sample=%d phase=%s identity=%q err=%v", result.sample+1, result.phase, result.identity, result.err)
	}
	rate := float64(successes) / float64(sampleCount)
	t.Logf(
		"ENTERPRISE191_PASSWORD_RESET_METRIC samples=%d successes=%d success_rate=%.5f environment=ci_mysql",
		sampleCount, successes, rate,
	)
	if rate < 0.99 {
		t.Fatalf("password recovery success rate %.5f below 0.99", rate)
	}
}
