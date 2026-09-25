//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	accessnotification "github.com/hvritual/biz/internal/access/infrastructure/notification"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
)

// This suite reuses the existing primary MySQL fixture and runtime. Run only
// through the canonical serial qualification workflow; it does not reset data,
// create databases or provision an extra container. An API fixture is NOT a
// supplier-delivery or browser proof.
func TestEnterprise183NotificationPreferencesMySQLAndHTTP(t *testing.T) {
	db := openDB(t)
	runtime := startEnterprise182Runtime(t, db)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	stamp := fmt.Sprint(time.Now().UnixNano())
	userID, tenantA, tenantB := "e183-u-"+stamp, "e183-a-"+stamp, "e183-b-"+stamp
	email := "e183-" + stamp + "@example.invalid"
	for _, tenant := range []string{tenantA, tenantB} {
		if err := runtime.store.Bootstrap(ctx, accesspersistence.Bootstrap{
			TenantID: tenant, TenantName: tenant, UserID: userID, Email: email, Token: "e183-token-" + tenant,
		}, nil); err != nil {
			t.Fatal(err)
		}
	}
	ownerA := domain.NotificationPreferenceOwner{TenantID: tenantA, UserID: userID}
	ownerB := domain.NotificationPreferenceOwner{TenantID: tenantB, UserID: userID}
	identity, err := runtime.store.ResolveOrBindOIDCIdentity(ctx, runtime.issuer.URL, "e183-sub-"+stamp, email, true)
	if err != nil {
		t.Fatal(err)
	}
	rawA, _, err := runtime.store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	sessionA, err := runtime.store.SwitchWebSessionTenant(ctx, rawA, tenantA)
	if err != nil {
		t.Fatal(err)
	}
	base := "http://" + runtime.started.HTTPAddress()
	endpoint := "/auth/personal/notification-preferences"
	denySMS := domain.NotificationPreferenceChange{Channel: domain.NotificationPreferenceSMS, Allowed: false, ExpectedVersion: 0, IdempotencyKey: "sms-first"}
	var firstReceipt domain.NotificationPreferenceReceipt

	t.Run("missing-explicit-and-independent-channels", func(t *testing.T) {
		initial, err := runtime.store.ReadNotificationPreferences(ctx, ownerA)
		if err != nil {
			t.Fatal(err)
		}
		for _, preference := range []domain.NotificationPreference{initial.SMS, initial.Email} {
			allowed, err := preference.OptionalAllowed()
			if err != nil || !allowed || preference.State != domain.NotificationPreferenceDefault || preference.Version != 0 {
				t.Fatalf("missing preference: %+v / %v", preference, err)
			}
		}
		if initial.Policy != domain.NotificationPreferencePolicy {
			t.Fatalf("policy: %s", initial.Policy)
		}
		firstReceipt, err = runtime.store.ChangeNotificationPreference(ctx, ownerA, denySMS)
		if err != nil {
			t.Fatal(err)
		}
		if firstReceipt.Preference.State != domain.NotificationPreferenceDeny || firstReceipt.Preference.Version != 1 {
			t.Fatalf("deny receipt: %+v", firstReceipt)
		}
		if _, err := runtime.store.ChangeNotificationPreference(ctx, ownerA, domain.NotificationPreferenceChange{
			Channel: domain.NotificationPreferenceEmail, Allowed: true, ExpectedVersion: 0, IdempotencyKey: "email-first",
		}); err != nil {
			t.Fatal(err)
		}
		got, err := runtime.store.ReadNotificationPreferences(ctx, ownerA)
		if err != nil || got.SMS.State != domain.NotificationPreferenceDeny || got.Email.State != domain.NotificationPreferenceAllow || got.Email.Version != 1 {
			t.Fatalf("channel separation: %+v / %v", got, err)
		}
	})

	t.Run("tenant-and-user-isolation", func(t *testing.T) {
		otherUser := "e183-other-" + stamp
		enterprise182SeedSecondOwner(t, db, tenantA, otherUser)
		for _, owner := range []domain.NotificationPreferenceOwner{ownerB, {TenantID: tenantA, UserID: otherUser}} {
			got, err := runtime.store.ReadNotificationPreferences(ctx, owner)
			if err != nil || got.SMS.State != domain.NotificationPreferenceDefault || got.Email.State != domain.NotificationPreferenceDefault {
				t.Fatalf("owner isolation: %+v / %v", got, err)
			}
		}
		// Existing authority tables may compare identifiers case-insensitively;
		// preference access still requires the exact canonical owner identity.
		alias := ownerA
		alias.UserID = strings.ToUpper(alias.UserID)
		if _, err := runtime.store.ReadNotificationPreferences(ctx, alias); !errors.Is(err, domain.ErrNotificationPreferenceForbidden) {
			t.Fatalf("canonical owner alias accepted: %v", err)
		}
	})

	t.Run("stable-replay-and-no-historical-overwrite", func(t *testing.T) {
		replay, err := runtime.store.ChangeNotificationPreference(ctx, ownerA, denySMS)
		if err != nil || !reflect.DeepEqual(replay, firstReceipt) {
			t.Fatalf("unstable replay: %+v / %v", replay, err)
		}
		changedPayload := denySMS
		changedPayload.Allowed = true
		if _, err := runtime.store.ChangeNotificationPreference(ctx, ownerA, changedPayload); !errors.Is(err, domain.ErrNotificationPreferenceIdempotencyConflict) {
			t.Fatalf("key reuse: %v", err)
		}
		stale := denySMS
		stale.IdempotencyKey = "stale-cas"
		if _, err := runtime.store.ChangeNotificationPreference(ctx, ownerA, stale); !errors.Is(err, domain.ErrNotificationPreferenceConflict) {
			t.Fatalf("stale CAS: %v", err)
		}
		next := denySMS
		next.Allowed = true
		next.ExpectedVersion = 1
		next.IdempotencyKey = "sms-second"
		if _, err := runtime.store.ChangeNotificationPreference(ctx, ownerA, next); err != nil {
			t.Fatal(err)
		}
		replay, err = runtime.store.ChangeNotificationPreference(ctx, ownerA, denySMS)
		if err != nil || !reflect.DeepEqual(replay, firstReceipt) {
			t.Fatalf("old receipt changed: %+v / %v", replay, err)
		}
		current, err := runtime.store.ReadNotificationPreference(ctx, ownerA, domain.NotificationPreferenceSMS)
		if err != nil || current.State != domain.NotificationPreferenceAllow || current.Version != 2 {
			t.Fatalf("replay overwrote current state: %+v / %v", current, err)
		}
	})

	t.Run("concurrent-first-write-has-one-CAS-winner", func(t *testing.T) {
		start := make(chan struct{})
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			command := domain.NotificationPreferenceChange{Channel: domain.NotificationPreferenceSMS, Allowed: i == 0, ExpectedVersion: 0, IdempotencyKey: fmt.Sprintf("race-%d", i)}
			go func(command domain.NotificationPreferenceChange) {
				<-start
				_, err := runtime.store.ChangeNotificationPreference(ctx, ownerB, command)
				results <- err
			}(command)
		}
		close(start)
		successes, conflicts := 0, 0
		for i := 0; i < 2; i++ {
			err := <-results
			if err == nil {
				successes++
			} else if errors.Is(err, domain.ErrNotificationPreferenceConflict) {
				conflicts++
			} else {
				t.Errorf("unexpected concurrent result: %v", err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
		}
		current, err := runtime.store.ReadNotificationPreference(ctx, ownerB, domain.NotificationPreferenceSMS)
		if err != nil || current.Version != 1 {
			t.Fatalf("current: %+v / %v", current, err)
		}
	})

	t.Run("concurrent-identical-key-produces-one-receipt", func(t *testing.T) {
		command := domain.NotificationPreferenceChange{Channel: domain.NotificationPreferenceEmail, Allowed: false, ExpectedVersion: 0, IdempotencyKey: "same-key"}
		type outcome struct {
			receipt domain.NotificationPreferenceReceipt
			err     error
		}
		start := make(chan struct{})
		results := make(chan outcome, 2)
		for i := 0; i < 2; i++ {
			go func() {
				<-start
				receipt, err := runtime.store.ChangeNotificationPreference(ctx, ownerB, command)
				results <- outcome{receipt, err}
			}()
		}
		close(start)
		one, two := <-results, <-results
		if one.err != nil || two.err != nil || !reflect.DeepEqual(one.receipt, two.receipt) {
			t.Fatalf("replay race: %+v / %+v", one, two)
		}
		var count int64
		if err := db.Table("biz_notification_preference_receipts").Where("tenant_id = ? AND user_id = ? AND key_hash = ?", tenantB, userID, one.receipt.ReceiptID).Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("receipt count=%d err=%v", count, err)
		}
		if err := db.Table("biz_audit_events").Where("audit_id = ?", one.receipt.ReceiptID).Count(&count).Error; err != nil || count != 2 {
			t.Fatalf("audit count=%d err=%v", count, err)
		}
	})

	t.Run("receipt-and-audit-failure-roll-back-the-choice", func(t *testing.T) {
		for _, table := range []string{"biz_notification_preference_receipts", "biz_audit_events"} {
			t.Run(table, func(t *testing.T) {
				before, err := runtime.store.ReadNotificationPreferences(ctx, ownerA)
				if err != nil {
					t.Fatal(err)
				}
				injected := errors.New("injected preference transaction failure")
				callback := "enterprise183:fail-" + table
				if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
					if tx.Statement.Schema != nil && tx.Statement.Schema.Table == table {
						tx.AddError(injected)
					}
				}); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := db.Callback().Create().Remove(callback); err != nil {
						t.Error(err)
					}
				}()
				command := domain.NotificationPreferenceChange{Channel: domain.NotificationPreferenceSMS, Allowed: false, ExpectedVersion: before.SMS.Version, IdempotencyKey: "rollback-" + table}
				if _, err := runtime.store.ChangeNotificationPreference(ctx, ownerA, command); !errors.Is(err, injected) {
					t.Fatalf("expected rollback failure, got %v", err)
				}
				after, err := runtime.store.ReadNotificationPreferences(ctx, ownerA)
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("partial preference write: %+v / %v", after, err)
				}
				keyHash, _, err := command.Fingerprints(ownerA)
				if err != nil {
					t.Fatal(err)
				}
				var count int64
				if err := db.Table("biz_notification_preference_receipts").Where("tenant_id = ? AND user_id = ? AND key_hash = ?", tenantA, userID, keyHash).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("partial receipt: %d / %v", count, err)
				}
				if err := db.Table("biz_audit_events").Where("audit_id = ?", keyHash).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("partial audit: %d / %v", count, err)
				}
			})
		}
	})

	t.Run("controlled-reader-denies-and-new-store-reads-persistence", func(t *testing.T) {
		fresh, err := accesspersistence.New(db)
		if err != nil {
			t.Fatal(err)
		}
		var reader ports.OptionalNotificationPreferenceReader = fresh
		preference, err := reader.ReadNotificationPreference(ctx, ownerB, domain.NotificationPreferenceEmail)
		if err != nil {
			t.Fatal(err)
		}
		allowed, err := preference.OptionalAllowed()
		if err != nil || allowed || preference.Version != 1 {
			t.Fatalf("denied channel allowed: %+v / %v", preference, err)
		}
		created := 0
		enqueue := func(context.Context) error { created++; return nil }
		admitted, err := accessnotification.AdmitOptionalNotification(ctx, reader, ownerB, domain.NotificationPreferenceEmail, enqueue)
		if err != nil || admitted || created != 0 {
			t.Fatalf("explicit refusal created an optional task: admitted=%v tasks=%d error=%v", admitted, created, err)
		}
		// A's independent, explicitly allowed email remains admissible.
		admitted, err = accessnotification.AdmitOptionalNotification(ctx, reader, ownerA, domain.NotificationPreferenceEmail, enqueue)
		if err != nil || !admitted || created != 1 {
			t.Fatalf("independent allowed channel: admitted=%v tasks=%d error=%v", admitted, created, err)
		}
		// #185 binds this seam to its durable outbox and pre-delivery recheck.
		// This test proves admission, not provider delivery or its TOCTOU boundary.
	})

	t.Run("HTTP-session-CSRF-body-and-context-negatives", func(t *testing.T) {
		validBody := `{"channel":"sms","allowed":false,"expected_version":2}`
		type caseData struct {
			name, session, path, body string
			headers                   map[string]string
			status                    int
		}
		headers := func() map[string]string { return enterprise182SessionHeaders(sessionA, "http-negative") }
		noCSRF := headers()
		delete(noCSRF, "X-CSRF-Token")
		noContext := headers()
		delete(noContext, "X-Biz-Session-Context")
		forgedContext := headers()
		forgedContext["X-Biz-Session-Context"] = `{}`
		cases := []caseData{
			{"no-session", "", endpoint, validBody, headers(), 401},
			{"no-CSRF", rawA, endpoint, validBody, noCSRF, 403},
			{"no-context", rawA, endpoint, validBody, noContext, 409},
			{"forged-context", rawA, endpoint, validBody, forgedContext, 409},
			{"query-tenant", rawA, endpoint + "?tenant_id=" + tenantB, validBody, headers(), 400},
			{"body-identity", rawA, endpoint, `{"channel":"sms","allowed":false,"expected_version":2,"user_id":"other"}`, headers(), 400},
			{"null-allowed", rawA, endpoint, `{"channel":"sms","allowed":null,"expected_version":2}`, headers(), 400},
			{"duplicate-allowed", rawA, endpoint, `{"channel":"sms","allowed":true,"allowed":false,"expected_version":2}`, headers(), 400},
			{"missing-version", rawA, endpoint, `{"channel":"sms","allowed":false}`, headers(), 400},
			{"unknown-channel", rawA, endpoint, `{"channel":"push","allowed":false,"expected_version":2}`, headers(), 400},
			{"stale-CAS", rawA, endpoint, `{"channel":"sms","allowed":false,"expected_version":0}`, headers(), 409},
		}
		before, err := runtime.store.ReadNotificationPreferences(ctx, ownerA)
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				status, _ := notificationPreferenceHTTP(t, base, http.MethodPost, test.path, test.session, test.headers, test.body)
				if status != test.status {
					t.Fatalf("status=%d want=%d", status, test.status)
				}
			})
		}
		after, err := runtime.store.ReadNotificationPreferences(ctx, ownerA)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("negative request mutated state: %+v / %v", after, err)
		}
	})

	t.Run("HTTP-idempotency-refresh-and-new-login-readback", func(t *testing.T) {
		headers := enterprise182SessionHeaders(sessionA, "http-deny")
		body := `{"channel":"sms","allowed":false,"expected_version":2}`
		status, response := notificationPreferenceHTTP(t, base, http.MethodPost, endpoint, rawA, headers, body)
		if status != 200 {
			t.Fatalf("save=%d %s", status, response)
		}
		status, replay := notificationPreferenceHTTP(t, base, http.MethodPost, endpoint, rawA, headers, body)
		if status != 200 || !bytes.Equal(response, replay) {
			t.Fatalf("HTTP receipt mismatch: %d %s / %s", status, response, replay)
		}
		status, response = notificationPreferenceHTTP(t, base, http.MethodGet, endpoint, rawA, headers, "")
		if status != 200 {
			t.Fatalf("GET=%d %s", status, response)
		}
		var current domain.NotificationPreferenceSet
		if err := json.Unmarshal(response, &current); err != nil {
			t.Fatal(err)
		}
		if current.TenantID != tenantA || current.UserID != userID || current.SMS.State != domain.NotificationPreferenceDeny || current.SMS.Version != 3 {
			t.Fatalf("GET current: %+v", current)
		}
		freshRaw, _, err := runtime.store.CreateWebSession(ctx, identity, time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		freshSession, err := runtime.store.SwitchWebSessionTenant(ctx, freshRaw, tenantA)
		if err != nil {
			t.Fatal(err)
		}
		status, freshResponse := notificationPreferenceHTTP(t, base, http.MethodGet, endpoint, freshRaw, enterprise182SessionHeaders(freshSession, "unused-read-key"), "")
		if status != 200 || !bytes.Equal(response, freshResponse) {
			t.Fatalf("new-session mismatch: %d %s / %s", status, response, freshResponse)
		}
	})

	t.Run("transaction-revalidates-the-bound-session-before-read-write-and-replay", func(t *testing.T) {
		before, err := runtime.store.ReadNotificationPreferences(ctx, ownerA)
		if err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"switch", "revoke", "expire", "csrf"} {
			t.Run(action, func(t *testing.T) {
				raw, _, err := runtime.store.CreateWebSession(ctx, identity, time.Hour)
				if err != nil {
					t.Fatal(err)
				}
				snapshot, err := runtime.store.SwitchWebSessionTenant(ctx, raw, tenantA)
				if err != nil {
					t.Fatal(err)
				}
				bound := runtime.store.NotificationPreferencesForWebSession(raw, snapshot.Session)
				want := accesspersistence.ErrWebSessionChanged
				switch action {
				case "switch":
					_, err = runtime.store.SwitchWebSessionTenant(ctx, raw, tenantB)
				case "revoke":
					err = runtime.store.RevokeWebSession(ctx, raw)
					want = accesspersistence.ErrWebSessionInvalid
				case "expire":
					err = db.Table("biz_web_sessions").Where("token_hash = ?", accesspersistence.TokenHash(raw)).Update("expires_at", time.Now().Add(-time.Minute)).Error
					want = accesspersistence.ErrWebSessionInvalid
				case "csrf":
					err = db.Table("biz_web_sessions").Where("token_hash = ?", accesspersistence.TokenHash(raw)).Update("csrf_token", "replacement-csrf").Error
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, err := bound.ReadNotificationPreferences(ctx, ownerA); !errors.Is(err, want) {
					t.Fatalf("stale snapshot read: %v want %v", err, want)
				}
				command := domain.NotificationPreferenceChange{Channel: domain.NotificationPreferenceSMS, Allowed: true, ExpectedVersion: before.SMS.Version, IdempotencyKey: "stale-session-" + action}
				if _, err := bound.ChangeNotificationPreference(ctx, ownerA, command); !errors.Is(err, want) {
					t.Fatalf("stale snapshot write: %v want %v", err, want)
				}
				if _, err := bound.ChangeNotificationPreference(ctx, ownerA, denySMS); !errors.Is(err, want) {
					t.Fatalf("replay bypassed session validation: %v want %v", err, want)
				}
			})
		}
		// The self-service port must reject a different valid owner, not just a
		// missing owner. This also guards misuse by a future BFF adapter.
		bound := runtime.store.NotificationPreferencesForWebSession(rawA, sessionA.Session)
		if _, err := bound.ReadNotificationPreferences(ctx, ownerB); !errors.Is(err, accesspersistence.ErrWebSessionChanged) {
			t.Fatalf("bound owner substitution accepted: %v", err)
		}
		after, err := runtime.store.ReadNotificationPreferences(ctx, ownerA)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatalf("stale session changed state: %+v / %v", after, err)
		}
	})

	t.Run("tenant-switch-rejects-old-context", func(t *testing.T) {
		oldHeaders := enterprise182SessionHeaders(sessionA, "old-tab-write")
		newSession, err := runtime.store.SwitchWebSessionTenant(ctx, rawA, tenantB)
		if err != nil {
			t.Fatal(err)
		}
		status, _ := notificationPreferenceHTTP(t, base, http.MethodPost, endpoint, rawA, oldHeaders, `{"channel":"email","allowed":true,"expected_version":1}`)
		if status != 409 {
			t.Fatalf("old context accepted: %d", status)
		}
		status, response := notificationPreferenceHTTP(t, base, http.MethodGet, endpoint, rawA, enterprise182SessionHeaders(newSession, "unused"), "")
		var current domain.NotificationPreferenceSet
		if status != 200 || json.Unmarshal(response, &current) != nil || current.TenantID != tenantB || current.Email.State != domain.NotificationPreferenceDeny {
			t.Fatalf("new tenant readback: %d %s", status, response)
		}
	})

	t.Run("inactive-or-missing-owner-fails-closed", func(t *testing.T) {
		if _, err := runtime.store.ReadNotificationPreferences(ctx, domain.NotificationPreferenceOwner{TenantID: tenantA, UserID: "not-present"}); !errors.Is(err, domain.ErrNotificationPreferenceForbidden) {
			t.Fatalf("missing owner: %v", err)
		}
		if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", tenantB, userID).Update("status", "suspended").Error; err != nil {
			t.Fatal(err)
		}
		if _, err := runtime.store.ReadNotificationPreference(ctx, ownerB, domain.NotificationPreferenceEmail); !errors.Is(err, domain.ErrNotificationPreferenceForbidden) {
			t.Fatalf("inactive reader: %v", err)
		}
		if _, err := runtime.store.ChangeNotificationPreference(ctx, ownerB, domain.NotificationPreferenceChange{Channel: domain.NotificationPreferenceEmail, Allowed: true, ExpectedVersion: 1, IdempotencyKey: "inactive"}); !errors.Is(err, domain.ErrNotificationPreferenceForbidden) {
			t.Fatalf("inactive writer: %v", err)
		}
		status, _ := notificationPreferenceHTTP(t, base, http.MethodGet, endpoint, rawA, enterprise182SessionHeaders(sessionA, "unused"), "")
		if status != 401 {
			t.Fatalf("suspended session=%d want=401", status)
		}
		// Persisted deny is retained; deactivation is not a preference reset.
		var state string
		if err := db.Table("biz_notification_preferences").Select("state").Where("tenant_id = ? AND user_id = ? AND channel = ?", tenantB, userID, "email").Scan(&state).Error; err != nil || state != "deny" {
			t.Fatalf("deny lost: %q / %v", state, err)
		}
	})
}

func notificationPreferenceHTTP(t *testing.T, base, method, path, rawSession string, headers map[string]string, body string) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if rawSession != "" {
		request.AddCookie(&http.Cookie{Name: "biz_session", Value: rawSession})
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	result, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("cacheable personal response: %q", response.Header.Get("Cache-Control"))
	}
	return response.StatusCode, result
}
