//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"

	accessapp "github.com/hvritual/biz/internal/access/application"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accessnotification "github.com/hvritual/biz/internal/access/infrastructure/notification"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/bizruntime"
	notificationapp "github.com/hvritual/biz/internal/notification/application"
	"gorm.io/gorm"
)

// These tests use real HTTP handlers, the canonical Executor and real MySQL.
// Only the final provider is an explicitly in-memory test double. Never log
// request bodies, complete provider claims, OTPs, links or plaintext contacts.
type enterprise185HTTPDelivery struct {
	db         *gorm.DB
	repository *accesspersistence.VerificationRepository
	sender     *accessnotification.MemorySender
	worker     notificationapp.SecurityDeliveryWorker
}

func newEnterprise185HTTPDelivery(t *testing.T, db *gorm.DB, protection *accesspersistence.VerificationProtection) enterprise185HTTPDelivery {
	t.Helper()
	repository, err := accesspersistence.NewVerificationRepository(db, protection)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := repository.EnsureReliableSecurityNotificationSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	sender := accessnotification.NewMemorySender()
	return enterprise185HTTPDelivery{db: db, repository: repository, sender: sender,
		worker: notificationapp.SecurityDeliveryWorker{Repository: repository, Sender: sender,
			Policy: accessdomain.EnterpriseNotificationRetryPolicy(), WorkerID: "http-lifecycle-185"}}
}

func (f enterprise185HTTPDelivery) count(t *testing.T) int64 {
	t.Helper()
	var count int64
	if err := f.db.Table("biz_security_notification_outbox").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func (f enterprise185HTTPDelivery) event(t *testing.T, businessID, tenant, user string, kind accessdomain.SecurityNotificationKind) string {
	t.Helper()
	var row struct{ EventID, TenantID, UserID, Kind, State, DestinationCiphertext string }
	if err := f.db.Table("biz_security_notification_outbox").Where("business_event_id = ?", businessID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.TenantID != tenant || row.UserID != user || row.Kind != string(kind) || row.State != accessdomain.NotificationStatePending || row.DestinationCiphertext == "" {
		t.Fatal("committed notification identity, state or protected destination mismatch")
	}
	if _, sent := f.sender.Message(row.EventID); sent {
		t.Fatal("request path performed synchronous provider I/O")
	}
	return row.EventID
}

func (f enterprise185HTTPDelivery) deliver(t *testing.T, eventID string) accessdomain.SecurityNotificationClaim {
	t.Helper()
	before := f.sender.Count()
	result, err := f.worker.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.EventID != eventID || result.State != accessdomain.NotificationStateDelivered || f.sender.Count() != before+1 {
		t.Fatal("reliable worker did not deliver exactly the expected committed event")
	}
	message, ok := f.sender.Message(eventID)
	if !ok {
		t.Fatal("test provider did not observe expected event")
	}
	var row struct {
		DestinationCiphertext, SecretCiphertext, State string
		LeaseUntil                                     *time.Time
	}
	if err := f.db.Table("biz_security_notification_outbox").Where("event_id = ?", eventID).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != accessdomain.NotificationStateDelivered || row.DestinationCiphertext != "" || row.SecretCiphertext != "" || row.LeaseUntil != nil {
		t.Fatal("completed security task retained protected material or a lease")
	}
	return message
}

func (f enterprise185HTTPDelivery) idle(t *testing.T) {
	t.Helper()
	before := f.sender.Count()
	result, err := f.worker.RunOnce(context.Background())
	if err != nil || result.EventID != "" || f.sender.Count() != before {
		t.Fatal("idle/replayed worker unexpectedly produced another notification")
	}
}

func enterprise185IDPServer(t *testing.T, store *accesspersistence.Store, delivery enterprise185HTTPDelivery, protection *accesspersistence.VerificationProtection) (*httptest.Server, *http.Client) {
	t.Helper()
	ctx := context.Background()
	for _, ensure := range []func(context.Context) error{store.EnsureFirstPartyIDPSchema, store.EnsureFirstPartyIDPSecuritySchema, store.EnsureWebSessionSchema} {
		if err := ensure(ctx); err != nil {
			t.Fatal(err)
		}
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(nil)
	publicURL := "http://" + server.Listener.Addr().String()
	service, err := accessapp.NewVerificationService(delivery.repository, delivery.sender, enterprise170Policy())
	if err != nil {
		t.Fatal(err)
	}
	handler, err := bizruntime.NewFirstPartyIdPHandlerWithSecurity(bizruntime.FirstPartyIdPConfig{
		PublicURL: publicURL, ClientID: "notification-http-185", RedirectURL: "http://127.0.0.1/auth/callback",
		SigningKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})),
		LoginTTL:      5 * time.Minute, CodeTTL: time.Minute, TokenTTL: 5 * time.Minute,
		RecoveryAuthorizationTTL: 5 * time.Minute, OTPCodeDigits: 6,
		PrivacyConsent: bizruntime.FirstPartyPrivacyConsentConfig{AgreementVersion: "qualification-v1",
			PrivacyPolicyURL: "https://example.invalid/privacy", TermsURL: "https://example.invalid/terms",
			ReconsentPolicy: bizruntime.PrivacyReconsentCurrentVersion},
	}, store, service, protection)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	server.Config.Handler = handler
	server.Start()
	t.Cleanup(server.Close)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return server, client
}

func enterprise185Form(t *testing.T, client *http.Client, method, endpoint string, values url.Values) (int, string) {
	t.Helper()
	var response *http.Response
	var err error
	if method == http.MethodGet {
		response, err = client.Get(endpoint)
	} else {
		response, err = client.PostForm(endpoint, values)
	}
	if err != nil {
		t.Fatal("identity HTTP transport failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		t.Fatal("identity HTTP response could not be read")
	}
	return response.StatusCode, string(body)
}

func enterprise185Hidden(t *testing.T, body, name string) string {
	t.Helper()
	match := regexp.MustCompile(`name="` + regexp.QuoteMeta(name) + `" value="([^"]*)"`).FindStringSubmatch(body)
	if len(match) != 2 || match[1] == "" {
		t.Fatalf("identity form field %s missing", name)
	}
	return html.UnescapeString(match[1])
}
