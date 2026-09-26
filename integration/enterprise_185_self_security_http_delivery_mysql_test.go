//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

func TestEnterprise185ContactAndDeletionBFFEventsReachReliableWorker(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	runtime := startEnterprise182Runtime(t, db)
	f := newEnterprise185HTTPDelivery(t, db, runtime.verificationProtection)
	ctx := context.Background()
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenant, user := "e185-self-"+stamp, "e185-user-"+stamp
	email := "self-" + stamp + "@example.invalid"
	password := "CoffeePass9A"
	if err := runtime.store.Bootstrap(ctx, accesspersistence.Bootstrap{TenantID: tenant, TenantName: tenant, UserID: user, Email: email, Token: "self-token-" + stamp}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runtime.store.SetUserPassword(ctx, user, password); err != nil {
		t.Fatal(err)
	}
	version := enterprise182SetTenantContacts(t, db, runtime.contactProtection, tenant, user, email, "+491701234567")
	enterprise182SeedSecondOwner(t, db, tenant, "e185-second-"+stamp)
	identity, err := runtime.store.ResolveOrBindOIDCIdentity(ctx, runtime.issuer.URL, "subject-"+stamp, email, true)
	if err != nil {
		t.Fatal(err)
	}
	raw, session, err := runtime.store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if session.Session.ActiveTenantID != tenant {
		t.Fatal("self fixture did not get current tenant")
	}
	base := "http://" + runtime.started.HTTPAddress()
	newEmail := "new-" + stamp + "@example.invalid"

	if !t.Run("contact-change-cookie-csrf-otp-outbox-worker", func(t *testing.T) {
		headers := enterprise182SessionHeaders(session, "contact-request-"+stamp)
		status, body := enterprise182Post(t, base, raw, "/auth/personal/contact-change/request", headers, map[string]any{"channel": "email", "destination": newEmail, "current_password": password})
		if status != http.StatusOK || f.sender.Count() != 0 {
			t.Fatalf("contact request HTTP status=%d or synchronous send", status)
		}
		challenge := enterprise182DecodeMap(t, body)
		challengeID := challenge["challenge_id"].(string)
		message := f.deliver(t, enterprise182ChallengeEventID(t, db, challengeID))
		if message.TenantID != tenant || message.UserID != user || message.Destination != newEmail {
			t.Fatal("contact verification targeted wrong identity")
		}
		input := map[string]any{"channel": "email", "destination": newEmail, "challenge_id": challengeID, "flow_id": challenge["flow_id"], "otp_code": message.Secret, "version": version}
		headers = enterprise182SessionHeaders(session, "contact-complete-"+stamp)
		status, body = enterprise182Post(t, base, raw, "/auth/personal/contact-change/complete", headers, input)
		if status != http.StatusOK || f.sender.Count() != 1 {
			t.Fatalf("contact complete HTTP status=%d or synchronous send", status)
		}
		receipt := enterprise182DecodeMap(t, body)
		version = uint64(receipt["version"].(float64))
		eventID := f.event(t, "tenant-self/contact-change-complete/"+challengeID, tenant, user, accessdomain.SecurityNotificationContactChanged)
		completion := f.deliver(t, eventID)
		if completion.TenantID != tenant || completion.UserID != user {
			t.Fatal("contact completion identity mismatch")
		}
		f.idle(t)
	}) {
		return
	}

	t.Run("self-deletion-confirmation-delivers-after-profile-erasure", func(t *testing.T) {
		status, body := enterprise182Post(t, base, raw, "/auth/personal/tenant-deletion/request", enterprise182SessionHeaders(session, "delete-request-"+stamp), map[string]any{"channel": "email"})
		if status != http.StatusOK {
			t.Fatalf("deletion request HTTP status=%d", status)
		}
		challenge := enterprise182DecodeMap(t, body)
		challengeID := challenge["challenge_id"].(string)
		message := f.deliver(t, enterprise182ChallengeEventID(t, db, challengeID))
		if message.Destination != newEmail {
			t.Fatal("deletion challenge did not use latest tenant contact")
		}
		status, body = enterprise182Post(t, base, raw, "/auth/personal/tenant-deletion/complete", enterprise182SessionHeaders(session, "delete-complete-"+stamp), map[string]any{"channel": "email", "challenge_id": challengeID, "flow_id": challenge["flow_id"], "otp_code": message.Secret, "version": version, "confirm_tenant_id": tenant, "confirm_irreversible": true})
		if status != http.StatusOK {
			t.Fatalf("deletion complete HTTP status=%d", status)
		}
		receipt := enterprise182DecodeMap(t, body)
		var member struct {
			Status, Email, EmailCiphertext, Phone, PhoneCiphertext string
			SelfDeletedAt                                          *time.Time
		}
		if err := db.Table("biz_memberships").Where("tenant_id=? AND user_id=?", tenant, user).Take(&member).Error; err != nil {
			t.Fatal(err)
		}
		if member.SelfDeletedAt == nil || member.Status != accessdomain.TenantMemberStatusRemoved || member.Email != "" || member.EmailCiphertext != "" || member.Phone != "" || member.PhoneCiphertext != "" {
			t.Fatal("completed deletion retained tenant profile")
		}
		if _, err := runtime.store.AuthenticateWebSession(ctx, raw); err == nil {
			t.Fatal("deleted tenant session still active")
		}
		eventID := f.event(t, "tenant-self/deletion-complete/"+challengeID, tenant, user, accessdomain.SecurityNotificationTenantDeletion)
		if receipt["notification_event_id"] != eventID {
			t.Fatal("deletion receipt and outbox differ")
		}
		completion := f.deliver(t, eventID)
		if completion.Destination != newEmail {
			t.Fatal("final confirmation lost protected pre-delete contact")
		}
		f.idle(t)
	})
}

func TestEnterprise185AppealBFFTargetsOnlyCurrentTenantOwners(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	runtime := startEnterprise182Runtime(t, db)
	f := newEnterprise185HTTPDelivery(t, db, runtime.verificationProtection)
	ctx := context.Background()
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenantA, tenantB := "e185-appeal-a-"+stamp, "e185-appeal-b-"+stamp
	owner, user := "e185-owner-"+stamp, "e185-applicant-"+stamp
	email := "applicant-" + stamp + "@example.invalid"
	if err := runtime.store.Bootstrap(ctx, accesspersistence.Bootstrap{TenantID: tenantA, TenantName: tenantA, UserID: owner, Email: owner + "@example.invalid", Token: "owner-" + stamp}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runtime.store.Bootstrap(ctx, accesspersistence.Bootstrap{TenantID: tenantB, TenantName: tenantB, UserID: user, Email: email, Token: "applicant-" + stamp}, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_memberships(tenant_id,user_id,status,version,created_at,updated_at) VALUES(?,?,?,1,NOW(6),NOW(6))", tenantA, user, accessdomain.TenantMemberStatusSuspended).Error; err != nil {
		t.Fatal(err)
	}
	identity, err := runtime.store.ResolveOrBindOIDCIdentity(ctx, runtime.issuer.URL, "subject-"+stamp, email, true)
	if err != nil {
		t.Fatal(err)
	}
	raw, session, err := runtime.store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + runtime.started.HTTPAddress()
	headers := enterprise182SessionHeaders(session, "appeal-"+stamp)
	status, _ := enterprise182Post(t, base, raw, "/auth/member-appeals", headers, map[string]any{"tenant_id": tenantA, "user_id": owner, "reason": "injected identity"})
	if status != http.StatusBadRequest || f.count(t) != 0 {
		t.Fatal("appeal accepted browser-supplied alternate identity")
	}
	status, body := enterprise182Post(t, base, raw, "/auth/member-appeals", headers, map[string]any{"tenant_id": tenantA, "reason": "review suspended membership"})
	if status != http.StatusAccepted || f.sender.Count() != 0 {
		t.Fatalf("appeal HTTP status=%d or synchronous provider call", status)
	}
	receipt := enterprise182DecodeMap(t, body)
	events := receipt["notification_event_ids"].([]any)
	if len(events) != 1 {
		t.Fatal("appeal did not target exactly tenant A owner")
	}
	message := f.deliver(t, events[0].(string))
	if message.UserID != owner || message.TenantID != tenantA || message.Kind != accessdomain.SecurityNotificationMemberAppeal {
		t.Fatal("appeal leaked to another owner/tenant")
	}
	before := f.count(t)
	status, _ = enterprise182Post(t, base, raw, "/auth/member-appeals", headers, map[string]any{"tenant_id": tenantA, "reason": "duplicate appeal"})
	if status != http.StatusTooManyRequests || f.count(t) != before {
		t.Fatal("repeated appeal produced duplicate notification")
	}
	var memberStatus string
	if err := db.Table("biz_memberships").Select("status").Where("tenant_id=? AND user_id=?", tenantA, user).Scan(&memberStatus).Error; err != nil {
		t.Fatal(err)
	}
	if memberStatus != accessdomain.TenantMemberStatusSuspended {
		t.Fatal("notification delivery granted applicant access")
	}
	f.idle(t)
}
