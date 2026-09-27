package bizruntime

import (
	"bytes"
	"errors"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestServiceAPIFixedCanonicalVector(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	body := []byte(`{"plan":"pro"}`)
	bodyHash := sha256.Sum256(body)
	canonical := serviceAPICanonicalString(
		http.MethodPost,
		"/v1/platform/tenants/tenant-a/subscription/change-previews",
		"a=1&a=2&z=9",
		"1760000000",
		"nonce-0123456789abcdef",
		hex.EncodeToString(bodyHash[:]),
	)
	const wantCanonical = "BIZ-HMAC-SHA256-V1\nPOST\n/v1/platform/tenants/tenant-a/subscription/change-previews\na=1&a=2&z=9\n1760000000\nnonce-0123456789abcdef\n029c40d2e5ce24535086fada67eb3ec85a1f8167d5a99eab7917ec0576126e99"
	if canonical != wantCanonical {
		t.Fatalf("canonical mismatch\n got: %q\nwant: %q", canonical, wantCanonical)
	}
	const wantSignature = "2702210d0a4cd19475e0861acd10d796e21b1c7f8dc319347174f833fa51ede9"
	if got := serviceAPISignature(secret, canonical); got != wantSignature {
		t.Fatalf("signature=%s want=%s", got, wantSignature)
	}
}

func TestServiceAPICanonicalQuerySortsKeysAndDuplicateValues(t *testing.T) {
	got, err := canonicalServiceAPIQuery("z=9&a=2&a=1&space=hello+world&encoded=%7E")
	if err != nil {
		t.Fatal(err)
	}
	if got != "a=1&a=2&encoded=~&space=hello+world&z=9" {
		t.Fatalf("canonical query=%q", got)
	}
}

func TestServiceAPICanonicalPathRejectsAmbiguity(t *testing.T) {
	for _, raw := range []string{
		"https://example.invalid/v1//platform/plans",
		"https://example.invalid/v1/%2Fplatform/plans",
		"https://example.invalid/v1/%5Cplatform/plans",
		"https://example.invalid/v1/../platform/plans",
	} {
		t.Run(raw, func(t *testing.T) {
			value, err := url.Parse(raw)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := canonicalServiceAPIPath(value); err == nil {
				t.Fatalf("accepted ambiguous path %q", raw)
			}
		})
	}
	value, err := url.Parse("https://example.invalid/v1/platform/tenants/tenant-a/subscription/change-previews")
	if err != nil {
		t.Fatal(err)
	}
	got, err := canonicalServiceAPIPath(value)
	if err != nil || got != "/v1/platform/tenants/tenant-a/subscription/change-previews" {
		t.Fatalf("canonical path=%q err=%v", got, err)
	}
}

func TestServiceAPIBodyDigestRestoresBody(t *testing.T) {
	request, err := http.NewRequest(http.MethodPost, "https://example.invalid/v1/platform/plans", strings.NewReader(`{"name":"x"}`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := digestAndRestoreServiceAPIBody(request)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(`{"name":"x"}`))
	if got != hex.EncodeToString(sum[:]) {
		t.Fatalf("digest=%s", got)
	}
	restored, err := io.ReadAll(request.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, []byte(`{"name":"x"}`)) {
		t.Fatalf("body not restored: %q", restored)
	}
}

func TestServiceAPIConfigRejectsWeakOrWronglyScopedCredentials(t *testing.T) {
	base := ServiceAPIAuthConfig{
		ClockSkew: time.Minute,
		Credentials: []ServiceAPICredentialConfig{{
			KeyID: "service-a", Subject: "platform-service:a",
			Secret: []byte(strings.Repeat("s", 32)),
			Operations: []string{"commercial.plan.discover"},
		}},
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	weak := base
	weak.Credentials = append([]ServiceAPICredentialConfig(nil), base.Credentials...)
	weak.Credentials[0].Secret = []byte("short")
	if err := weak.Validate(); err == nil {
		t.Fatal("weak service secret accepted")
	}

	queryTokenStyle := base
	queryTokenStyle.Credentials = append([]ServiceAPICredentialConfig(nil), base.Credentials...)
	queryTokenStyle.Credentials[0].TenantID = "tenant-a"
	if err := queryTokenStyle.Validate(); err == nil {
		t.Fatal("tenant binding accepted for operation without trusted tenant path parameter")
	}

	unknown := base
	unknown.Credentials = append([]ServiceAPICredentialConfig(nil), base.Credentials...)
	unknown.Credentials[0].Operations = []string{"unknown.operation"}
	if err := unknown.Validate(); err == nil {
		t.Fatal("unknown operation accepted")
	}
}

func TestServiceAPIRouteResolutionUsesCatalogOperation(t *testing.T) {
	auth, err := newServiceAPIAuthenticator(ServiceAPIAuthConfig{
		ClockSkew: time.Minute,
		Credentials: []ServiceAPICredentialConfig{{
			KeyID: "service-a", Subject: "platform-service:a",
			Secret: []byte(strings.Repeat("s", 32)),
			Operations: []string{"commercial.plan.discover"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := auth.resolveRoute(http.MethodGet, "/v1/platform/plans")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Operation != "commercial.plan.discover" || resolved.TenantID != "" {
		t.Fatalf("route=%+v", resolved)
	}
	if _, err := auth.resolveRoute(http.MethodPost, "/v1/platform/plans"); err == nil {
		t.Fatal("method tamper resolved to signed operation")
	}
}


func TestServiceAPIRejectsDuplicateSignedHeadersAndAuthorizationMixing(t *testing.T) {
	auth, err := newServiceAPIAuthenticator(ServiceAPIAuthConfig{
		ClockSkew: time.Minute,
		Credentials: []ServiceAPICredentialConfig{{
			KeyID: "service-a", Subject: "platform-service:a",
			Secret: []byte(strings.Repeat("s", 32)),
			Operations: []string{"commercial.plan.discover"},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	baseRequest := func() *http.Request {
		request, err := http.NewRequest(http.MethodGet, "https://example.invalid/v1/platform/plans", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set(serviceAPIKeyIDHeader, "service-a")
		request.Header.Set(serviceAPITimestampHeader, strconv.FormatInt(time.Now().Unix(), 10))
		request.Header.Set(serviceAPINonceHeader, "nonce-duplicate-000001")
		request.Header.Set(serviceAPISignatureHeader, strings.Repeat("a", 64))
		return request
	}

	t.Run("duplicate-header", func(t *testing.T) {
		request := baseRequest()
		request.Header.Add(serviceAPIKeyIDHeader, "service-b")
		if _, err := auth.authenticate(request); !errors.Is(err, errServiceAPIInvalidRequest) {
			t.Fatalf("duplicate service header error=%v", err)
		}
	})

	t.Run("authorization-mixed-with-signed-request", func(t *testing.T) {
		request := baseRequest()
		request.Header.Set("Authorization", "Basic opaque")
		if _, err := auth.authenticate(request); !errors.Is(err, errServiceAPIInvalidRequest) {
			t.Fatalf("mixed Authorization error=%v", err)
		}
	})
}
