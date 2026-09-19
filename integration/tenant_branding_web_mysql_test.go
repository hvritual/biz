//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/bizruntime"
	"github.com/hvritual/biz/modules/deviceops"
	"yunka.io/framework/platform"
	"yunka.io/pkg/logExt"
)

// The cookie is issued by the real session store. OIDC login protocol itself is
// certified separately by CE-12; this fixture only serves discovery metadata.
func TestTenantBrandingCookieCSRFAndLiveMembership(t *testing.T) {
	db := openDB(t)
	var issuer *httptest.Server
	issuer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize", "token_endpoint": issuer.URL + "/token", "jwks_uri": issuer.URL + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
	}))
	defer issuer.Close()
	config := deviceops.DefaultConfig()
	config.HTTPListenAddress = "127.0.0.1:0"
	config.GRPCListenAddress = "127.0.0.1:0"
	config.AutoMigrate = true
	provider, err := platform.New(platform.Options{Config: bizruntime.ConfigProvider{DeviceOps: config}, Logger: logExt.NewBaseLogger(), Databases: map[string]platform.DatabaseFactory{"primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
		return platform.BorrowedDatabase(db), nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, err := bizruntime.BootstrapWithOptions(ctx, provider, bizruntime.Options{DeviceOps: config, WebAuth: bizruntime.WebAuthConfig{IssuerURL: issuer.URL, ClientID: "branding-test", RedirectURL: "http://127.0.0.1/auth/callback", Scopes: []string{"openid"}, SessionTTL: time.Hour, FlowTTL: time.Minute}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = started.App.Shutdown(shutdown)
	})
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenant, user, token := "brand-cookie-"+stamp, "brand-cookie-user-"+stamp, "brand-cookie-token-"+stamp
	seedB123TenantAdmin(t, db, tenant, user, user+"@example.invalid", token)
	seedECIR05TenantProfilePermissions(t, db, tenant)
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := store.ResolveOrBindOIDCIdentity(ctx, issuer.URL, user, user+"@example.invalid", true)
	if err != nil {
		t.Fatal(err)
	}
	raw, auth, err := store.CreateWebSession(ctx, linked, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Session.ActiveTenantID != tenant {
		auth, err = store.SwitchWebSessionTenant(ctx, raw, tenant)
		if err != nil {
			t.Fatal(err)
		}
	}
	endpoint := "http://" + started.HTTPAddress() + "/v1/tenant/branding"
	send := func(method, csrf, key, expected string) int {
		t.Helper()
		var payload []byte
		if method == "PATCH" {
			payload = []byte(`{"preset":"violet","primary":"","version":"1"}`)
		}
		req, err := http.NewRequest(method, endpoint, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.AddCookie(&http.Cookie{Name: "biz_session", Value: raw})
		req.Header.Set("Content-Type", "application/json")
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		if expected != "" {
			req.Header.Set("X-Biz-Session-Context", expected)
		}
		res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, _ := io.ReadAll(res.Body)
		t.Logf("branding cookie %s status=%d body=%s", method, res.StatusCode, body)
		return res.StatusCode
	}
	expected, _ := json.Marshal(map[string]any{
		"actor_kind": string(auth.Session.ActorKind), "platform_subject": "", "user_id": user,
		"active_tenant_id": tenant, "context_version": auth.Session.ContextVersion,
	})
	if code := send("GET", "", "", string(expected)); code != 200 {
		t.Fatalf("cookie GET %d", code)
	}
	if code := send("PATCH", "wrong", "bad-csrf-"+stamp, string(expected)); code != 401 && code != 403 {
		t.Fatalf("bad csrf %d", code)
	}
	if code := send("PATCH", auth.Session.CSRFToken, "wrong-context-"+stamp, `{"actor_kind":"user","user_id":"other","active_tenant_id":"other"}`); code != 409 {
		t.Fatalf("wrong context %d", code)
	}
	if code := send("PATCH", auth.Session.CSRFToken, "cookie-save-"+stamp, string(expected)); code != 200 {
		t.Fatalf("cookie save %d", code)
	}
	if err := db.Exec("UPDATE biz_memberships SET status='suspended' WHERE tenant_id=? AND user_id=?", tenant, user).Error; err != nil {
		t.Fatal(err)
	}
	if code := send("GET", "", "", string(expected)); code == 200 {
		t.Fatal("revoked membership retained branding access")
	}
}
