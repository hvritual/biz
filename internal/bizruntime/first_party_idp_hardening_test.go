package bizruntime

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestCE12FirstPartyIdPKeyRolloverPublishesActiveAndPreviousKeys(t *testing.T) {
	active := testIDPPrivateKeyPEM(t)
	previous := testIDPPrivateKeyPEM(t)
	idp, err := newRuntimeFirstPartyIdP(testFirstPartyIdPConfig(active, []FirstPartyIdPVerificationKey{{PEM: previous}}))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	idp.handleJWKS(recorder, httptest.NewRequest(http.MethodGet, "/idp/jwks", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("jwks status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Keys) != 2 {
		t.Fatalf("jwks keys=%d want 2", len(body.Keys))
	}
	if body.Keys[0]["kid"] == body.Keys[1]["kid"] {
		t.Fatal("active and previous signing keys must have different kid values")
	}
}

func TestCE12FirstPartyIdPLogoutRequiresExactRegisteredRedirect(t *testing.T) {
	config := testFirstPartyIdPConfig(testIDPPrivateKeyPEM(t), nil)
	config.PostLogoutRedirectURL = "http://127.0.0.1:18080/signed-out"
	idp, err := newRuntimeFirstPartyIdP(config)
	if err != nil {
		t.Fatal(err)
	}
	invalid := httptest.NewRecorder()
	idp.handleProviderLogout(invalid, httptest.NewRequest(http.MethodGet, "/idp/logout?post_logout_redirect_uri="+url.QueryEscape("http://127.0.0.1:18081/anything"), nil))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid logout redirect status=%d want 400", invalid.Code)
	}
	valid := httptest.NewRecorder()
	idp.handleProviderLogout(valid, httptest.NewRequest(http.MethodGet, "/idp/logout?post_logout_redirect_uri="+url.QueryEscape(config.PostLogoutRedirectURL), nil))
	if valid.Code != http.StatusFound {
		t.Fatalf("valid logout redirect status=%d want 302", valid.Code)
	}
	if got := valid.Header().Get("Location"); got != config.PostLogoutRedirectURL {
		t.Fatalf("logout location=%q want %q", got, config.PostLogoutRedirectURL)
	}
}

func TestCE12FirstPartyIdPLoginReferrerPolicyPreservesFormOrigin(t *testing.T) {
	idp, err := newRuntimeFirstPartyIdP(testFirstPartyIdPConfig(testIDPPrivateKeyPEM(t), nil))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	idp.renderLogin(recorder, http.StatusOK, "request", "csrf", "", "")
	if got := recorder.Header().Get("Referrer-Policy"); got != "origin" {
		t.Fatalf("login Referrer-Policy=%q want origin", got)
	}
	if got := recorder.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatal("login page must retain Content-Security-Policy")
	}
}

func testFirstPartyIdPConfig(active string, previous []FirstPartyIdPVerificationKey) FirstPartyIdPConfig {
	return FirstPartyIdPConfig{
		PublicURL:           "http://127.0.0.1:18081",
		ClientID:            "biz-web",
		RedirectURL:         "http://127.0.0.1:18080/auth/callback",
		SigningKeyPEM:       active,
		PreviousSigningKeys: previous,
		LoginTTL:            5 * time.Minute,
		CodeTTL:             time.Minute,
		TokenTTL:            5 * time.Minute,
	}
}

func testIDPPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
}
