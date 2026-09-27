//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

func TestEnterprise173SelfPasswordChangeAtomicAndRevokesSessions(t *testing.T) {
	fixture := newEnterprise173Fixture(t)
	ctx := context.Background()
	const (
		userID      = "enterprise-173-change-user"
		email       = "enterprise173.change@example.invalid"
		oldPassword = "LegacyPass9A"
		newPassword = "Coffee88A"
	)
	const tenantID = "tenant-173-change"
	sessionA, sessionB := fixture.bootstrapAccount(t, userID, tenantID, email, oldPassword)

	if err := fixture.Store.ChangeOwnPasswordAudited(ctx, tenantID, userID, "enterprise173-change-wrong", "wrong-password", newPassword, newPassword); !errors.Is(err, accesspersistence.ErrCurrentPasswordInvalid) {
		t.Fatalf("wrong current password accepted: %v", err)
	}
	if _, err := fixture.Store.AuthenticateUserPassword(ctx, email, oldPassword); err != nil {
		t.Fatalf("wrong-current attempt changed credential: %v", err)
	}
	if err := fixture.Store.ChangeOwnPassword(ctx, userID, oldPassword, "weakpass", "weakpass"); !errors.Is(err, accesspersistence.ErrWeakUserPassword) {
		t.Fatalf("weak password accepted: %v", err)
	}
	if err := fixture.Store.ChangeOwnPassword(ctx, userID, oldPassword, newPassword, "Different9A"); !errors.Is(err, accesspersistence.ErrPasswordMismatch) {
		t.Fatalf("mismatched password accepted: %v", err)
	}
	if err := fixture.Store.ChangeOwnPasswordAudited(ctx, tenantID, userID, "enterprise173-change-success", oldPassword, newPassword, newPassword); err != nil {
		t.Fatal(err)
	}
	enterprise188RequireAuditOutcome(t, fixture.DB, tenantID, "identity.password.change", "failure", 1)
	enterprise188RequireAuditOutcome(t, fixture.DB, tenantID, "identity.password.change", "success", 1)
	if _, err := fixture.Store.AuthenticateUserPassword(ctx, email, oldPassword); !errors.Is(err, accesspersistence.ErrInvalidUserCredentials) {
		t.Fatalf("old password still works: %v", err)
	}
	if _, err := fixture.Store.AuthenticateUserPassword(ctx, email, newPassword); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
	for _, raw := range []string{sessionA, sessionB} {
		if _, err := fixture.Store.AuthenticateWebSession(ctx, raw); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
			t.Fatalf("old session survived password change: %v", err)
		}
	}
}
