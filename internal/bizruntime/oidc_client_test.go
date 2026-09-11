package bizruntime

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCE12PKCEUsesRFC7636S256(t *testing.T) {
	const verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	const want = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := pkceChallenge(verifier); got != want {
		t.Fatalf("pkce challenge = %q, want %q", got, want)
	}
}

func TestCE12WebAuthConfigRejectsInsecureRemoteRedirect(t *testing.T) {
	config := WebAuthConfig{
		IssuerURL:    "https://id.example.test",
		ClientID:     "biz-web",
		RedirectURL:  "http://app.example.test/auth/callback",
		Scopes:       []string{"openid", "profile", "email"},
		SessionTTL:   time.Hour,
		FlowTTL:      5 * time.Minute,
		CookieSecure: true,
	}
	if err := config.Validate(); err == nil {
		t.Fatal("expected insecure non-loopback redirect to be rejected")
	}
}

func TestCE12OIDCClaimsBindIssuerAudienceNonceAndExpiry(t *testing.T) {
	client := &oidcClient{config: WebAuthConfig{
		IssuerURL: "https://id.example.test",
		ClientID:  "biz-web",
	}}
	audience, err := json.Marshal([]string{"biz-web", "other"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2_000_000_000, 0).UTC()
	claims := oidcClaims{
		Issuer:          "https://id.example.test",
		Subject:         "subject-1",
		Audience:        audience,
		AuthorizedParty: "biz-web",
		ExpiresAt:       now.Add(time.Hour).Unix(),
		IssuedAt:        now.Add(-time.Minute).Unix(),
		Nonce:           "nonce-1",
	}
	if err := client.validateClaims(claims, TokenHashForWeb("nonce-1"), now); err != nil {
		t.Fatalf("valid claims rejected: %v", err)
	}
	claims.Audience, _ = json.Marshal("different-client")
	if err := client.validateClaims(claims, TokenHashForWeb("nonce-1"), now); err == nil {
		t.Fatal("wrong audience must be rejected")
	}
}

func TestCE12ReturnToCannotEscapeOrigin(t *testing.T) {
	cases := map[string]string{
		"":                                  "/",
		"/customers?tab=active":             "/customers?tab=active",
		"https://evil.example/steal":        "/",
		"//evil.example/steal":              "/",
		"javascript:alert(document.domain)": "/",
	}
	for input, want := range cases {
		if got := safeReturnTo(input); got != want {
			t.Errorf("safeReturnTo(%q) = %q, want %q", input, got, want)
		}
	}
}
