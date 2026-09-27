//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"gorm.io/gorm"
)

func TestEnterprise185AdminPasswordRecoveryRequestQueuesOnlySelfServiceNotification(t *testing.T) {
	fixture := newEnterprise173Fixture(t)
	ctx := context.Background()
	stamp := fmt.Sprint(time.Now().UnixNano())
	target, email, password := "e185-admin-recovery-target-"+stamp, "admin-recovery-"+stamp+"@example.invalid", "AdminRecover9A"
	tenantA, tenantB := "e185-admin-recovery-a-"+stamp, "e185-admin-recovery-b-"+stamp
	admin := "e185-admin-recovery-actor-" + stamp

	if err := fixture.Store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenantA, TenantName: tenantA, UserID: target, Email: email, Token: "admin-recovery-a-token-" + stamp,
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenantB, TenantName: tenantB, UserID: target, Email: email, Token: "admin-recovery-b-token-" + stamp,
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Store.SetUserPassword(ctx, target, password); err != nil {
		t.Fatal(err)
	}
	webIdentity, err := fixture.Store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.example.invalid", "sub-"+target, email, true)
	if err != nil {
		t.Fatal(err)
	}
	sessionA, _, err := fixture.Store.CreateWebSession(ctx, webIdentity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	sessionB, _, err := fixture.Store.CreateWebSession(ctx, webIdentity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	foreign := "e185-admin-recovery-foreign-" + stamp
	if err := fixture.Store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenantB, TenantName: tenantB, UserID: foreign, Email: foreign + "@example.invalid", Token: "admin-recovery-foreign-token-" + stamp,
	}, nil); err != nil {
		t.Fatal(err)
	}

	service, err := accesspersistence.NewTenantMemberPasswordRecoveryService(fixture.DB, nil, fixture.Protection)
	if err != nil {
		t.Fatal(err)
	}
	delivery := newEnterprise185HTTPDelivery(t, fixture.DB, fixture.Protection)

	t.Run("outbox-failure-does-not-create-recovery-request", func(t *testing.T) {
		const callback = "enterprise185:reject-admin-recovery-outbox"
		if err := fixture.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
			if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "biz_security_notification_outbox" {
				tx.AddError(errors.New("injected admin recovery outbox failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		_, requestErr := service.Request(ctx, tenantA, target, admin)
		if removeErr := fixture.DB.Callback().Create().Remove(callback); removeErr != nil {
			t.Fatal(removeErr)
		}
		if requestErr == nil || delivery.count(t) != 0 {
			t.Fatalf("faulted admin recovery err=%v outbox=%d", requestErr, delivery.count(t))
		}
	})

	receipt, err := service.Request(ctx, tenantA, target, admin)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.NotificationEventID == "" || receipt.NotificationState != accessdomain.NotificationStatePending || delivery.count(t) != 1 {
		t.Fatalf("admin recovery receipt=%+v outbox=%d", receipt, delivery.count(t))
	}
	var row struct {
		EventID, BusinessEventID, Kind, Purpose, UserID, TenantID, State, SecretCiphertext string
	}
	if err := fixture.DB.Table("biz_security_notification_outbox").
		Where("event_id=?", receipt.NotificationEventID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.UserID != target || row.TenantID != tenantA ||
		row.Kind != string(accessdomain.SecurityNotificationRecoveryRequest) ||
		row.Purpose != string(accessdomain.VerificationPurposePasswordRecovery) ||
		row.State != accessdomain.NotificationStatePending || row.SecretCiphertext != "" {
		t.Fatalf("admin recovery outbox=%+v", row)
	}
	message := delivery.deliver(t, receipt.NotificationEventID)
	if message.UserID != target || message.TenantID != tenantA || message.Destination != email ||
		message.Kind != accessdomain.SecurityNotificationRecoveryRequest || message.Secret != "" {
		t.Fatalf("admin recovery delivery=%+v", message)
	}

	if _, err := service.Request(ctx, tenantA, target, admin); err == nil {
		t.Fatal("immediate duplicate admin recovery was not rate limited")
	} else {
		var rateLimited accesspersistence.TenantMemberPasswordRecoveryRateLimitError
		if !errors.As(err, &rateLimited) || rateLimited.RetryAfter <= 0 || delivery.count(t) != 1 {
			t.Fatalf("admin recovery rate limit err=%v outbox=%d", err, delivery.count(t))
		}
	}
	if _, err := service.Request(ctx, tenantA, foreign, admin); !errors.Is(err, accesspersistence.ErrTenantMemberPasswordRecoveryNotFound) {
		t.Fatalf("foreign tenant recovery err=%v", err)
	}
	if delivery.count(t) != 1 {
		t.Fatal("foreign tenant request created a recovery event")
	}

	if _, err := fixture.Store.AuthenticateUserPassword(ctx, email, password); err != nil {
		t.Fatal("admin recovery request changed global account password")
	}
	for _, raw := range []string{sessionA, sessionB} {
		if _, err := fixture.Store.AuthenticateWebSession(ctx, raw); err != nil {
			t.Fatalf("admin recovery request revoked existing session: %v", err)
		}
	}
	delivery.idle(t)
}
