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
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accessnotification "github.com/hvritual/biz/internal/access/infrastructure/notification"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"github.com/hvritual/biz/internal/bizruntime"
	"github.com/hvritual/biz/modules/deviceops"
	"gorm.io/gorm"
	"yunka.io/framework/platform"
	"yunka.io/pkg/logExt"
)

type enterprise182Runtime struct {
	started                *bizruntime.Started
	store                  *accesspersistence.Store
	contactProtection      *accesspersistence.ContactProtection
	verificationProtection *accesspersistence.VerificationProtection
	verificationRepository *accesspersistence.VerificationRepository
	sender                 *accessnotification.MemorySender
	issuer                 *httptest.Server
}

func startEnterprise182Runtime(t *testing.T, db *gorm.DB) enterprise182Runtime {
	t.Helper()
	var issuerURL string
	issuer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(writer, request)
			return
		}
		write := map[string]any{
			"issuer":                                issuerURL,
			"authorization_endpoint":                issuerURL + "/authorize",
			"token_endpoint":                        issuerURL + "/token",
			"jwks_uri":                              issuerURL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(write)
	}))
	issuerURL = issuer.URL
	t.Cleanup(issuer.Close)

	contactProtection, err := accesspersistence.NewContactProtection(accesspersistence.ContactProtectionConfig{
		ActiveVersion: "v1",
		Keys:          map[string][]byte{"v1": []byte(strings.Repeat("C", 32))},
		LookupKey:     []byte(strings.Repeat("L", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	verificationProtection, err := accesspersistence.NewVerificationProtection(accesspersistence.VerificationProtectionConfig{
		ActiveVersion: "v1",
		Keys:          map[string][]byte{"v1": []byte(strings.Repeat("V", 32))},
		HMACKey:       []byte(strings.Repeat("H", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}

	config := deviceops.DefaultConfig()
	config.HTTPListenAddress = "127.0.0.1:0"
	config.GRPCListenAddress = "127.0.0.1:0"
	config.AutoMigrate = true
	provider, err := platform.New(platform.Options{
		Config: bizruntime.ConfigProvider{DeviceOps: config},
		Logger: logExt.NewBaseLogger(),
		Databases: map[string]platform.DatabaseFactory{
			"primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
				return platform.BorrowedDatabase(db), nil
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	options := bizruntime.Options{
		DeviceOps: config,
		WebAuth: bizruntime.WebAuthConfig{
			IssuerURL:    issuerURL,
			ClientID:     "enterprise182-web",
			RedirectURL:  "http://127.0.0.1/auth/callback",
			Scopes:       []string{"openid", "profile", "email"},
			SessionTTL:   time.Hour,
			FlowTTL:      5 * time.Minute,
			CookieSecure: false,
		},
		VerificationSecurity: bizruntime.VerificationSecurityConfig{
			CodeTTL:              5 * time.Minute,
			AuthorizationTTL:     5 * time.Minute,
			ResendInterval:       time.Minute,
			SendLimitWindow:      24 * time.Hour,
			MaxSendsPerWindow:    5,
			MaxVerificationTries: 3,
			CodeDigits:           6,
			Notification:         bizruntime.SecurityNotificationProviderConfig{Provider: "disabled"},
		},
	}
	started, err := bizruntime.BootstrapWithOptionsAndSecurity(
		ctx,
		provider,
		options,
		contactProtection,
		verificationProtection,
	)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = started.App.Shutdown(shutdown)
	})

	store, err := accesspersistence.NewWithContactProtection(db, contactProtection)
	if err != nil {
		t.Fatal(err)
	}
	verificationRepository, err := accesspersistence.NewVerificationRepository(db, verificationProtection)
	if err != nil {
		t.Fatal(err)
	}
	if err := verificationRepository.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	sender := accessnotification.NewMemorySender()
	return enterprise182Runtime{
		started:                started,
		store:                  store,
		contactProtection:      contactProtection,
		verificationProtection: verificationProtection,
		verificationRepository: verificationRepository,
		sender:                 sender,
		issuer:                 issuer,
	}
}

func enterprise182SessionHeaders(authentication accesspersistence.WebSessionAuthentication, key string) map[string]string {
	contextJSON, _ := json.Marshal(map[string]any{
		"actor_kind":       authentication.Session.ActorKind,
		"platform_subject": authentication.Session.PlatformSubject,
		"user_id":          authentication.Session.UserID,
		"active_tenant_id": authentication.Session.ActiveTenantID,
		"context_version":  authentication.Session.ContextVersion,
	})
	return map[string]string{
		"Content-Type":          "application/json",
		"X-CSRF-Token":          authentication.Session.CSRFToken,
		"X-Biz-Session-Context": string(contextJSON),
		"Idempotency-Key":       key,
	}
}

func enterprise182Post(t *testing.T, base, rawSession, path string, headers map[string]string, body any) (int, []byte) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, base+path, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(&http.Cookie{Name: "biz_session", Value: rawSession})
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	return response.StatusCode, responseBody
}

func enterprise182DecodeMap(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode response %s: %v", body, err)
	}
	return result
}

func enterprise182DeliverOTP(t *testing.T, repository *accesspersistence.VerificationRepository, eventID string) string {
	t.Helper()
	claim, receipt, err := repository.ClaimSecurityNotification(context.Background(), eventID)
	if err != nil {
		t.Fatalf("claim OTP %s: %+v %v", eventID, receipt, err)
	}
	if claim.Secret == "" {
		t.Fatal("OTP secret missing from protected claim")
	}
	if _, err := repository.CompleteSecurityNotification(context.Background(), claim, "qualification-delivered", "", true); err != nil {
		t.Fatal(err)
	}
	// Delivery intentionally erases the destination. Completion must not decrypt it.
	return claim.Secret
}

func enterprise182ChallengeEventID(t *testing.T, db *gorm.DB, challengeID string) string {
	t.Helper()
	var eventID string
	if err := db.Table("biz_security_notification_outbox").
		Select("event_id").Where("challenge_id = ?", challengeID).Scan(&eventID).Error; err != nil {
		t.Fatal(err)
	}
	if eventID == "" {
		t.Fatalf("challenge %s has no notification outbox", challengeID)
	}
	return eventID
}

func enterprise182SetTenantContacts(
	t *testing.T,
	db *gorm.DB,
	protection *accesspersistence.ContactProtection,
	tenantID, userID, email, phone string,
) uint64 {
	t.Helper()
	repository, err := accesspersistence.NewTenantMemberRepositoryWithContactProtection(db, protection)
	if err != nil {
		t.Fatal(err)
	}
	member, err := repository.Get(context.Background(), tenantID, userID)
	if err != nil {
		t.Fatal(err)
	}
	member.Email = email
	member.Phone = phone
	member.Name = "Qualification " + tenantID
	if err := repository.Update(context.Background(), &member, member.Version); err != nil {
		t.Fatal(err)
	}
	confirmed, err := repository.Get(context.Background(), tenantID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Version != member.Version {
		t.Fatalf("fixture version disagrees with persisted membership: memory=%d persisted=%d", member.Version, confirmed.Version)
	}
	return confirmed.Version
}

func enterprise182SeedSecondOwner(t *testing.T, db *gorm.DB, tenantID, userID string) {
	t.Helper()
	email := userID + "@example.invalid"
	if err := db.Exec("INSERT INTO biz_users(id,email,status,created_at) VALUES(?,?,?,NOW(6))", userID, email, "active").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_memberships(tenant_id,user_id,status,version,created_at,updated_at) VALUES(?,?,?,1,NOW(6),NOW(6))",
		tenantID, userID, accessdomain.TenantMemberStatusActive).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_member_roles(tenant_id,user_id,role_id) VALUES(?,?,?)", tenantID, userID, tenantID+":owner").Error; err != nil {
		t.Fatal(err)
	}
}

func TestEnterprise182ContactChangeAndTenantDeletionAreSelfOnlyAtomicAndTenantScoped(t *testing.T) {
	db := openDB(t)
	runtime := startEnterprise182Runtime(t, db)
	ctx := context.Background()
	stamp := fmt.Sprint(time.Now().UnixNano())

	userID := "e182-shared-" + stamp
	tenantA := "e182-a-" + stamp
	tenantB := "e182-b-" + stamp
	globalEmail := "global-" + stamp + "@example.invalid"
	password := "CoffeePass9A"
	tokenA := "e182-token-a-" + stamp
	tokenB := "e182-token-b-" + stamp

	for _, tenant := range []struct{ id, token string }{{tenantA, tokenA}, {tenantB, tokenB}} {
		if err := runtime.store.Bootstrap(ctx, accesspersistence.Bootstrap{
			TenantID:   tenant.id,
			TenantName: "Tenant " + tenant.id,
			UserID:     userID,
			Email:      globalEmail,
			Token:      tenant.token,
		}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := runtime.store.SetUserPassword(ctx, userID, password); err != nil {
		t.Fatal(err)
	}
	initialAVersion := enterprise182SetTenantContacts(t, db, runtime.contactProtection, tenantA, userID,
		"a-"+stamp+"@example.invalid", "+491701234567")
	initialBVersion := enterprise182SetTenantContacts(t, db, runtime.contactProtection, tenantB, userID,
		"b-"+stamp+"@example.invalid", "+8613800138000")
	if initialAVersion != 2 || initialBVersion != 2 {
		t.Fatalf("initial member versions A=%d B=%d want=2", initialAVersion, initialBVersion)
	}

	identity, err := runtime.store.ResolveOrBindOIDCIdentity(ctx, runtime.issuer.URL, "subject-"+stamp, globalEmail, true)
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
	rawB, _, err := runtime.store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	sessionB, err := runtime.store.SwitchWebSessionTenant(ctx, rawB, tenantB)
	if err != nil {
		t.Fatal(err)
	}

	base := "http://" + runtime.started.HTTPAddress()

	// Client identity injection is rejected before it can become an alternate identity source.
	headersA := enterprise182SessionHeaders(sessionA, "e182-inject-"+stamp)
	status, _ := enterprise182Post(t, base, rawA, "/auth/personal/contact-change/request", headersA, map[string]any{
		"channel":          "email",
		"destination":      "injected-" + stamp + "@example.invalid",
		"current_password": password,
		"user_id":          "other-user",
		"tenant_id":        tenantB,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("identity injection status=%d want=%d", status, http.StatusBadRequest)
	}

	// Wrong password creates no challenge and changes nothing.
	var challengeCountBefore int64
	if err := db.Table("biz_verification_challenges").Count(&challengeCountBefore).Error; err != nil {
		t.Fatal(err)
	}
	headersA = enterprise182SessionHeaders(sessionA, "e182-wrong-password-"+stamp)
	status, _ = enterprise182Post(t, base, rawA, "/auth/personal/contact-change/request", headersA, map[string]any{
		"channel":          "email",
		"destination":      "new-a-" + stamp + "@example.invalid",
		"current_password": "WrongPassword9A",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("wrong password status=%d want=%d", status, http.StatusBadRequest)
	}
	var challengeCountAfter int64
	if err := db.Table("biz_verification_challenges").Count(&challengeCountAfter).Error; err != nil {
		t.Fatal(err)
	}
	if challengeCountAfter != challengeCountBefore {
		t.Fatalf("wrong password created verification challenge: before=%d after=%d", challengeCountBefore, challengeCountAfter)
	}

	// Email happy path, with wrong OTP and cross-tenant binding negatives first.
	newEmailA := "new-a-" + stamp + "@example.invalid"
	headersA = enterprise182SessionHeaders(sessionA, "e182-email-request-"+stamp)
	status, body := enterprise182Post(t, base, rawA, "/auth/personal/contact-change/request", headersA, map[string]any{
		"channel":          "email",
		"destination":      newEmailA,
		"current_password": password,
	})
	if status != http.StatusOK {
		t.Fatalf("email request status=%d body=%s", status, body)
	}
	emailChallenge := enterprise182DecodeMap(t, body)
	if int(emailChallenge["resend_after_seconds"].(float64)) != 60 {
		t.Fatalf("resend window=%v want=60", emailChallenge["resend_after_seconds"])
	}
	emailChallengeID := emailChallenge["challenge_id"].(string)
	emailFlowID := emailChallenge["flow_id"].(string)
	emailOTP := enterprise182DeliverOTP(t, runtime.verificationRepository, enterprise182ChallengeEventID(t, db, emailChallengeID))

	headersB := enterprise182SessionHeaders(sessionB, "e182-cross-tenant-"+stamp)
	status, _ = enterprise182Post(t, base, rawB, "/auth/personal/contact-change/complete", headersB, map[string]any{
		"channel": "email", "challenge_id": emailChallengeID, "flow_id": emailFlowID,
		"otp_code": emailOTP, "version": initialBVersion,
		"destination": newEmailA,
	})
	if status == http.StatusOK {
		t.Fatal("tenant B consumed tenant A contact challenge")
	}

	headersA = enterprise182SessionHeaders(sessionA, "e182-email-complete-"+stamp)
	status, _ = enterprise182Post(t, base, rawA, "/auth/personal/contact-change/complete", headersA, map[string]any{
		"channel": "email", "challenge_id": emailChallengeID, "flow_id": emailFlowID,
		"otp_code": "not-a-code", "version": initialAVersion,
		"destination": newEmailA,
	})
	if status == http.StatusOK {
		t.Fatal("wrong contact-change OTP changed member")
	}
	memberRepository, err := accesspersistence.NewTenantMemberRepositoryWithContactProtection(db, runtime.contactProtection)
	if err != nil {
		t.Fatal(err)
	}
	memberA, err := memberRepository.Get(ctx, tenantA, userID)
	if err != nil {
		t.Fatal(err)
	}
	if memberA.Version != initialAVersion || memberA.Email == accesspersistence.MaskEmail(newEmailA) {
		t.Fatalf("wrong OTP mutated tenant A member: %+v", memberA)
	}

	status, body = enterprise182Post(t, base, rawA, "/auth/personal/contact-change/complete", headersA, map[string]any{
		"channel": "email", "challenge_id": emailChallengeID, "flow_id": emailFlowID,
		"otp_code": emailOTP, "version": initialAVersion,
		"destination": newEmailA,
	})
	if status != http.StatusOK {
		t.Fatalf("email complete status=%d body=%s", status, body)
	}
	emailReceipt := enterprise182DecodeMap(t, body)
	if emailReceipt["email"] != accesspersistence.MaskEmail(newEmailA) {
		t.Fatalf("email receipt leaked/wrong masked value: %+v", emailReceipt)
	}
	emailVersion := uint64(emailReceipt["version"].(float64))

	// Replay must fail.
	headersReplay := enterprise182SessionHeaders(sessionA, "e182-email-replay-"+stamp)
	status, _ = enterprise182Post(t, base, rawA, "/auth/personal/contact-change/complete", headersReplay, map[string]any{
		"channel": "email", "challenge_id": emailChallengeID, "flow_id": emailFlowID,
		"otp_code": emailOTP, "version": emailVersion,
		"destination": newEmailA,
	})
	if status != http.StatusConflict {
		t.Fatalf("replayed OTP status=%d want=%d", status, http.StatusConflict)
	}

	// Phone happy path.
	newPhoneA := "+491709876543"
	headersPhoneRequest := enterprise182SessionHeaders(sessionA, "e182-phone-request-"+stamp)
	status, body = enterprise182Post(t, base, rawA, "/auth/personal/contact-change/request", headersPhoneRequest, map[string]any{
		"channel": "sms", "destination": newPhoneA, "current_password": password,
	})
	if status != http.StatusOK {
		t.Fatalf("phone request status=%d body=%s", status, body)
	}
	phoneChallenge := enterprise182DecodeMap(t, body)
	phoneChallengeID := phoneChallenge["challenge_id"].(string)
	phoneFlowID := phoneChallenge["flow_id"].(string)
	phoneOTP := enterprise182DeliverOTP(t, runtime.verificationRepository, enterprise182ChallengeEventID(t, db, phoneChallengeID))
	headersPhoneComplete := enterprise182SessionHeaders(sessionA, "e182-phone-complete-"+stamp)
	status, body = enterprise182Post(t, base, rawA, "/auth/personal/contact-change/complete", headersPhoneComplete, map[string]any{
		"channel": "sms", "challenge_id": phoneChallengeID, "flow_id": phoneFlowID,
		"otp_code": phoneOTP, "version": emailVersion,
		"destination": newPhoneA,
	})
	if status != http.StatusOK {
		t.Fatalf("phone complete status=%d body=%s", status, body)
	}
	phoneReceipt := enterprise182DecodeMap(t, body)
	if phoneReceipt["phone"] != accesspersistence.MaskPhone(newPhoneA) {
		t.Fatalf("phone receipt leaked/wrong masked value: %+v", phoneReceipt)
	}
	currentAVersion := uint64(phoneReceipt["version"].(float64))

	// Tenant B and global Account facts remain unchanged by tenant A rebind.
	memberB, err := memberRepository.Get(ctx, tenantB, userID)
	if err != nil {
		t.Fatal(err)
	}
	if memberB.Version != initialBVersion || memberB.Email != accesspersistence.MaskEmail("b-"+stamp+"@example.invalid") ||
		memberB.Phone != accesspersistence.MaskPhone("+8613800138000") {
		t.Fatalf("tenant A contact changes crossed into tenant B: %+v", memberB)
	}
	if _, err := runtime.store.AuthenticateUserPassword(ctx, globalEmail, password); err != nil {
		t.Fatalf("tenant contact change altered global credential: %v", err)
	}

	// Last owner cannot even start self deletion.
	headersDeleteLastOwner := enterprise182SessionHeaders(sessionA, "e182-delete-last-owner-"+stamp)
	status, _ = enterprise182Post(t, base, rawA, "/auth/personal/tenant-deletion/request", headersDeleteLastOwner, map[string]any{"channel": "email"})
	if status != http.StatusConflict {
		t.Fatalf("last owner deletion request status=%d want=%d", status, http.StatusConflict)
	}
	enterprise182SeedSecondOwner(t, db, tenantA, "e182-second-owner-"+stamp)

	// Deletion verification uses the bound contact and rejects cross-purpose challenge.
	headersDeleteRequest := enterprise182SessionHeaders(sessionA, "e182-delete-request-"+stamp)
	status, body = enterprise182Post(t, base, rawA, "/auth/personal/tenant-deletion/request", headersDeleteRequest, map[string]any{"channel": "email"})
	if status != http.StatusOK {
		t.Fatalf("deletion request status=%d body=%s", status, body)
	}
	deleteChallenge := enterprise182DecodeMap(t, body)
	deleteChallengeID := deleteChallenge["challenge_id"].(string)
	deleteFlowID := deleteChallenge["flow_id"].(string)
	deleteOTP := enterprise182DeliverOTP(t, runtime.verificationRepository, enterprise182ChallengeEventID(t, db, deleteChallengeID))

	headersDeleteComplete := enterprise182SessionHeaders(sessionA, "e182-delete-complete-"+stamp)
	status, _ = enterprise182Post(t, base, rawA, "/auth/personal/tenant-deletion/complete", headersDeleteComplete, map[string]any{
		"channel": "email", "challenge_id": emailChallengeID, "flow_id": emailFlowID,
		"otp_code": emailOTP, "version": currentAVersion,
		"confirm_tenant_id": tenantA, "confirm_irreversible": true,
	})
	if status == http.StatusOK {
		t.Fatal("contact-change challenge was accepted for tenant deletion")
	}
	status, _ = enterprise182Post(t, base, rawA, "/auth/personal/tenant-deletion/complete", headersDeleteComplete, map[string]any{
		"channel": "email", "challenge_id": deleteChallengeID, "flow_id": deleteFlowID,
		"otp_code": "not-a-code", "version": currentAVersion,
		"confirm_tenant_id": tenantA, "confirm_irreversible": true,
	})
	if status == http.StatusOK {
		t.Fatal("wrong deletion OTP removed membership")
	}

	// Force final-notification staging conflict: entire deletion must roll back, including OTP consumption.
	conflictBusinessEvent := "tenant-self/deletion-complete/" + deleteChallengeID
	if err := db.Exec(`
INSERT INTO biz_security_notification_outbox
(event_id,business_event_id,challenge_id,binding_hash,kind,purpose,user_id,tenant_id,flow_id,channel,destination_hash,masked_destination,destination_ciphertext,secret_ciphertext,key_version,state,attempts,provider_receipt,failure_code,expires_at,created_at,updated_at)
VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,0,'','',DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 5 MINUTE),UTC_TIMESTAMP(6),UTC_TIMESTAMP(6))`,
		"e182-conflict-"+stamp, conflictBusinessEvent, "", "wrong-binding",
		string(accessdomain.SecurityNotificationTenantDeletion), string(accessdomain.VerificationPurposeAccountDeletion),
		userID, tenantA, deleteFlowID, string(accessdomain.SecurityNotificationEmail), "wrong-hash", "***", "cipher", "", "v1",
		accessdomain.NotificationStatePending,
	).Error; err != nil {
		t.Fatal(err)
	}
	status, _ = enterprise182Post(t, base, rawA, "/auth/personal/tenant-deletion/complete", headersDeleteComplete, map[string]any{
		"channel": "email", "challenge_id": deleteChallengeID, "flow_id": deleteFlowID,
		"otp_code": deleteOTP, "version": currentAVersion,
		"confirm_tenant_id": tenantA, "confirm_irreversible": true,
	})
	if status != http.StatusConflict {
		t.Fatalf("notification conflict deletion status=%d want=%d", status, http.StatusConflict)
	}
	memberA, err = memberRepository.Get(ctx, tenantA, userID)
	if err != nil {
		t.Fatal(err)
	}
	if memberA.Status != accessdomain.TenantMemberStatusActive || memberA.Version != currentAVersion {
		t.Fatalf("failed deletion left partial membership: %+v", memberA)
	}
	var consumedAt *time.Time
	if err := db.Table("biz_verification_challenges").Select("consumed_at").Where("challenge_id = ?", deleteChallengeID).Scan(&consumedAt).Error; err != nil {
		t.Fatal(err)
	}
	if consumedAt != nil {
		t.Fatal("failed deletion consumed OTP challenge")
	}
	if err := db.Table("biz_security_notification_outbox").Where("business_event_id = ?", conflictBusinessEvent).Delete(map[string]any{}).Error; err != nil {
		t.Fatal(err)
	}

	status, body = enterprise182Post(t, base, rawA, "/auth/personal/tenant-deletion/complete", headersDeleteComplete, map[string]any{
		"channel": "email", "challenge_id": deleteChallengeID, "flow_id": deleteFlowID,
		"otp_code": deleteOTP, "version": currentAVersion,
		"confirm_tenant_id": tenantA, "confirm_irreversible": true,
	})
	if status != http.StatusOK {
		t.Fatalf("tenant deletion complete status=%d body=%s", status, body)
	}
	deleteReceipt := enterprise182DecodeMap(t, body)
	deleteEventID := deleteReceipt["notification_event_id"].(string)

	// Current-tenant sessions and API token are revoked, but tenant B and global Account survive.
	if _, err := runtime.store.AuthenticateWebSession(ctx, rawA); !errors.Is(err, accesspersistence.ErrWebSessionInvalid) {
		t.Fatalf("tenant A old session survived self deletion: %v", err)
	}
	if _, err := runtime.store.AuthenticateWebSession(ctx, rawB); err != nil {
		t.Fatalf("tenant B session was revoked by tenant A deletion: %v", err)
	}
	if _, err := runtime.store.AuthenticateUserPassword(ctx, globalEmail, password); err != nil {
		t.Fatalf("global Account credential was removed: %v", err)
	}
	memberB, err = memberRepository.Get(ctx, tenantB, userID)
	if err != nil || memberB.Status != accessdomain.TenantMemberStatusActive {
		t.Fatalf("tenant B membership changed by tenant A deletion: %+v %v", memberB, err)
	}
	var tokenADisabled, tokenBDisabled bool
	if err := db.Table("biz_api_tokens").Select("disabled").Where("token_hash = ?", accesspersistence.TokenHash(tokenA)).Scan(&tokenADisabled).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_api_tokens").Select("disabled").Where("token_hash = ?", accesspersistence.TokenHash(tokenB)).Scan(&tokenBDisabled).Error; err != nil {
		t.Fatal(err)
	}
	if !tokenADisabled || tokenBDisabled {
		t.Fatalf("tenant API token revocation crossed boundary: A=%v B=%v", tokenADisabled, tokenBDisabled)
	}

	// Self-deleted current tenant is tombstoned, stripped, and cannot use admin restore.
	var deletedRow struct {
		Status                                                                                   string
		Version                                                                                  uint64
		Name, Email, EmailCiphertext, Phone, PhoneCiphertext, EmployeeID, Position, DepartmentID string
		SelfDeletedAt                                                                            *time.Time
	}
	if err := db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", tenantA, userID).Scan(&deletedRow).Error; err != nil {
		t.Fatal(err)
	}
	if deletedRow.Status != accessdomain.TenantMemberStatusRemoved || deletedRow.SelfDeletedAt == nil ||
		deletedRow.Name != "" || deletedRow.Email != "" || deletedRow.EmailCiphertext != "" ||
		deletedRow.Phone != "" || deletedRow.PhoneCiphertext != "" || deletedRow.EmployeeID != "" ||
		deletedRow.Position != "" || deletedRow.DepartmentID != "" {
		t.Fatalf("tenant self deletion did not clean tenant profile: %+v", deletedRow)
	}
	memberA, err = memberRepository.Get(ctx, tenantA, userID)
	if err != nil {
		t.Fatal(err)
	}
	if err := memberA.Restore(time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := memberRepository.Restore(ctx, &memberA, deletedRow.Version); !errors.Is(err, accesspersistence.ErrTenantSelfDeletionIrreversible) {
		t.Fatalf("self-deleted membership was restorable: %v", err)
	}

	// Notification may fail after the irreversible tenant transaction without rolling it back.
	claim, receipt, err := runtime.verificationRepository.ClaimSecurityNotification(ctx, deleteEventID)
	if err != nil {
		t.Fatalf("claim deletion confirmation: %+v %v", receipt, err)
	}
	if claim.Destination != newEmailA {
		t.Fatalf("deletion notification lost protected pre-delete destination: %q", claim.Destination)
	}
	failedDelivery, err := runtime.verificationRepository.CompleteSecurityNotification(ctx, claim, "", "PROVIDER_DOWN", false)
	if err != nil || failedDelivery.State != accessdomain.NotificationStateFailed {
		t.Fatalf("deletion notification failure not observable: %+v %v", failedDelivery, err)
	}
	if err := db.Table("biz_memberships").Select("status").Where("tenant_id = ? AND user_id = ?", tenantA, userID).Scan(&deletedRow.Status).Error; err != nil {
		t.Fatal(err)
	}
	if deletedRow.Status != accessdomain.TenantMemberStatusRemoved {
		t.Fatalf("notification failure rolled back legitimate deletion: %s", deletedRow.Status)
	}
	claim, _, err = runtime.verificationRepository.ClaimSecurityNotification(ctx, deleteEventID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.verificationRepository.CompleteSecurityNotification(ctx, claim, "qualification-retry", "", true); err != nil {
		t.Fatal(err)
	}
	var encryptedAfterDelivery string
	if err := db.Table("biz_security_notification_outbox").Select("destination_ciphertext").Where("event_id = ?", deleteEventID).Scan(&encryptedAfterDelivery).Error; err != nil {
		t.Fatal(err)
	}
	if encryptedAfterDelivery != "" {
		t.Fatal("successful confirmation delivery retained temporary destination ciphertext")
	}

	// Audit and persisted public notification fields contain no raw contact or credential.
	var evidence []struct {
		Target        string
		Reason        string
		RequestDigest string
		ReceiptRef    string
	}
	if err := db.Table("biz_audit_events").Select("target,reason,request_digest,receipt_ref").
		Where("tenant_id = ? AND actor_user_id = ? AND operation_id IN ?", tenantA, userID, []string{
			"tenant.personal.contact_change", "tenant.membership.self_delete",
		}).Find(&evidence).Error; err != nil {
		t.Fatal(err)
	}
	if len(evidence) < 4 {
		t.Fatalf("self-security audit evidence rows=%d want>=4", len(evidence))
	}
	serialized, _ := json.Marshal(evidence)
	for _, secret := range []string{newEmailA, newPhoneA, password, emailOTP, phoneOTP, deleteOTP} {
		if bytes.Contains(serialized, []byte(secret)) {
			t.Fatalf("audit leaked secret/contact %q", secret)
		}
	}
	var publicOutbox []struct {
		BusinessEventID, MaskedDestination, ProviderReceipt, FailureCode string
	}
	if err := db.Table("biz_security_notification_outbox").
		Select("business_event_id,masked_destination,provider_receipt,failure_code").
		Where("tenant_id = ? AND user_id = ?", tenantA, userID).Find(&publicOutbox).Error; err != nil {
		t.Fatal(err)
	}
	outboxJSON, _ := json.Marshal(publicOutbox)
	for _, secret := range []string{newEmailA, newPhoneA, password, emailOTP, phoneOTP, deleteOTP} {
		if bytes.Contains(outboxJSON, []byte(secret)) {
			t.Fatalf("public outbox leaked secret/contact %q", secret)
		}
	}
}

func TestEnterprise182ConcurrentContactOTPConsumptionAllowsOneMutation(t *testing.T) {
	db := openDB(t)
	runtime := startEnterprise182Runtime(t, db)
	ctx := context.Background()
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenantID := "e182-race-" + stamp
	userID := "e182-race-user-" + stamp
	email := "race-" + stamp + "@example.invalid"
	password := "CoffeePass9A"

	if err := runtime.store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenantID, TenantName: tenantID, UserID: userID, Email: email, Token: "race-token-" + stamp,
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runtime.store.SetUserPassword(ctx, userID, password); err != nil {
		t.Fatal(err)
	}
	version := enterprise182SetTenantContacts(t, db, runtime.contactProtection, tenantID, userID, email, "+491701111111")

	service, err := accesspersistence.NewTenantSelfSecurityService(db, runtime.contactProtection, runtime.verificationProtection, accessdomain.VerificationPolicy{
		CodeTTL: 5 * time.Minute, AuthorizationTTL: 5 * time.Minute,
		ResendInterval: time.Minute, SendLimitWindow: 24 * time.Hour,
		MaxSendsPerWindow: 5, MaxVerificationTries: 3, CodeDigits: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	newEmail := "race-new-" + stamp + "@example.invalid"
	challenge, err := service.RequestContactChange(ctx, userID, tenantID, password, accessdomain.SecurityNotificationEmail,
		newEmail, "race-flow-"+stamp, "race-event-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	otp := enterprise182DeliverOTP(t, runtime.verificationRepository, enterprise182ChallengeEventID(t, db, challenge.ChallengeID))

	type outcome struct {
		receipt accesspersistence.TenantSelfContactChangeReceipt
		err     error
	}
	start := make(chan struct{})
	results := make(chan outcome, 2)
	for index := 0; index < 2; index++ {
		go func(index int) {
			<-start
			receipt, err := service.CompleteContactChange(
				context.Background(), userID, tenantID, accessdomain.SecurityNotificationEmail,
				challenge.ChallengeID, challenge.FlowID, otp, newEmail, version, fmt.Sprintf("race-%d-%s", index, stamp),
			)
			results <- outcome{receipt: receipt, err: err}
		}(index)
	}
	close(start)

	successes := 0
	failures := 0
	for range 2 {
		result := <-results
		if result.err == nil {
			successes++
			if result.receipt.Version != version+1 {
				t.Fatalf("winning concurrent receipt version=%d want=%d", result.receipt.Version, version+1)
			}
		} else {
			failures++
			if !errors.Is(result.err, accessdomain.ErrVerificationConsumed) &&
				!errors.Is(result.err, accessports.ErrTenantMemberConflict) {
				t.Fatalf("unexpected losing concurrent error: %v", result.err)
			}
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("concurrent OTP outcomes success=%d failure=%d want=1/1", successes, failures)
	}
	repository, _ := accesspersistence.NewTenantMemberRepositoryWithContactProtection(db, runtime.contactProtection)
	member, err := repository.Get(ctx, tenantID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if member.Version != version+1 || member.Email != accesspersistence.MaskEmail(newEmail) {
		t.Fatalf("concurrent OTP left invalid member state: %+v", member)
	}
}
