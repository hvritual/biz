//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/access/domain"
	accessnotification "github.com/hvritual/biz/internal/access/infrastructure/notification"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"gorm.io/gorm"
	"yunka.io/gateway/authz"
)

type enterprise173Fixture struct {
	DB         *gorm.DB
	Store      *accesspersistence.Store
	Protection *accesspersistence.VerificationProtection
	Service    *accessapp.VerificationService
	Sender     *accessnotification.MemorySender
}

func newEnterprise173Fixture(t *testing.T) enterprise173Fixture {
	t.Helper()
	db := ce08FreshFixtureDB(t)
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureFirstPartyIDPSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureFirstPartyIDPSecuritySchema(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureWebSessionSchema(ctx); err != nil {
		t.Fatal(err)
	}
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
	if err := repository.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	sender := accessnotification.NewMemorySender()
	service, err := accessapp.NewVerificationService(repository, sender, domain.VerificationPolicy{
		CodeTTL: 5 * time.Minute, AuthorizationTTL: 5 * time.Minute,
		ResendInterval: time.Minute, SendLimitWindow: 24 * time.Hour,
		MaxSendsPerWindow: 5, MaxVerificationTries: 3, CodeDigits: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	return enterprise173Fixture{DB: db, Store: store, Protection: protection, Service: service, Sender: sender}
}

func (fixture enterprise173Fixture) bootstrapAccount(t *testing.T, userID, tenantID, email, password string) (string, string) {
	t.Helper()
	ctx := context.Background()
	if err := fixture.Store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenantID, TenantName: tenantID, UserID: userID, Email: email, Token: "token-" + userID + "-" + tenantID,
	}, []authz.PermissionKey{"tenant.member.read"}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Store.SetUserPassword(ctx, userID, password); err != nil {
		t.Fatal(err)
	}
	identity, err := fixture.Store.ResolveOrBindOIDCIdentity(ctx, "https://issuer.example.invalid", "sub-"+userID, email, true)
	if err != nil {
		t.Fatal(err)
	}
	sessionA, _, err := fixture.Store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	sessionB, _, err := fixture.Store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return sessionA, sessionB
}

func (fixture enterprise173Fixture) sendRecoveryOTP(t *testing.T, businessEventID, flowID, userID, email string) (domain.VerificationChallengeReceipt, string) {
	t.Helper()
	challenge, delivery, err := fixture.Service.SendVerificationCode(context.Background(), domain.VerificationChallengeRequest{
		BusinessEventID: businessEventID,
		FlowID:          flowID,
		Purpose:         domain.VerificationPurposePasswordRecovery,
		UserID:          userID,
		Channel:         domain.SecurityNotificationEmail,
		Destination:     email,
	})
	if err != nil || delivery.State != domain.NotificationStateDelivered {
		t.Fatalf("recovery OTP delivery failed: %+v %v", delivery, err)
	}
	message, ok := fixture.Sender.Message(challenge.NotificationEventID)
	if !ok || message.Secret == "" {
		t.Fatal("recovery OTP evidence missing")
	}
	return challenge, message.Secret
}
