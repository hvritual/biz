//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

func TestEnterprise173RecoveryRollbackKeepsCredentialAndChallengeReusable(t *testing.T) {
	fixture := newEnterprise173Fixture(t)
	ctx := context.Background()
	const (
		userID      = "enterprise-173-rollback-user"
		email       = "enterprise173.rollback@example.invalid"
		oldPassword = "RollbackOld9A"
		newPassword = "RollbackNew9A"
	)
	fixture.bootstrapAccount(t, userID, "tenant-173-rollback", email, oldPassword)
	challenge, code := fixture.sendRecoveryOTP(t, "enterprise173/recovery/rollback", "recovery-flow-rollback", userID, email)

	const triggerName = "enterprise173_fail_challenge_update"
	_ = fixture.DB.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error
	if err := fixture.DB.Exec("CREATE TRIGGER " + triggerName + " BEFORE UPDATE ON biz_verification_challenges FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='forced rollback'").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fixture.DB.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error })

	_, err := fixture.Store.RecoverPasswordWithCode(ctx, fixture.Protection, domain.VerifyChallengeRequest{
		ChallengeID: challenge.ChallengeID, FlowID: "recovery-flow-rollback",
		Purpose: domain.VerificationPurposePasswordRecovery, UserID: userID,
		Channel: domain.SecurityNotificationEmail, Destination: email, Code: code,
	}, 5*time.Minute, newPassword, newPassword)
	if err == nil {
		t.Fatal("forced transaction failure unexpectedly succeeded")
	}
	if _, err := fixture.Store.AuthenticateUserPassword(ctx, email, oldPassword); err != nil {
		t.Fatalf("rollback lost original credential: %v", err)
	}
	if _, err := fixture.Store.AuthenticateUserPassword(ctx, email, newPassword); !errors.Is(err, accesspersistence.ErrInvalidUserCredentials) {
		t.Fatalf("rollback leaked new credential: %v", err)
	}
	var consumedAt sql.NullTime
	if err := fixture.DB.Table("biz_verification_challenges").Select("consumed_at").Where("challenge_id = ?", challenge.ChallengeID).Scan(&consumedAt).Error; err != nil {
		t.Fatal(err)
	}
	if consumedAt.Valid {
		t.Fatalf("rollback consumed challenge: %v", consumedAt.Time)
	}
	var authorizationCount int64
	if err := fixture.DB.Table("biz_one_time_authorizations").Where("challenge_id = ?", challenge.ChallengeID).Count(&authorizationCount).Error; err != nil {
		t.Fatal(err)
	}
	if authorizationCount != 0 {
		t.Fatalf("rollback retained authorization rows: %d", authorizationCount)
	}

	if err := fixture.DB.Exec("DROP TRIGGER IF EXISTS " + triggerName).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.Store.RecoverPasswordWithCode(ctx, fixture.Protection, domain.VerifyChallengeRequest{
		ChallengeID: challenge.ChallengeID, FlowID: "recovery-flow-rollback",
		Purpose: domain.VerificationPurposePasswordRecovery, UserID: userID,
		Channel: domain.SecurityNotificationEmail, Destination: email, Code: code,
	}, 5*time.Minute, newPassword, newPassword); err != nil {
		t.Fatalf("same OTP could not retry after rollback: %v", err)
	}
}
