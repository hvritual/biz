//go:build integration

package integration

import (
	"context"
	"errors"
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
	})
}
