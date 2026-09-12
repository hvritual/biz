package bizruntime

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"yunka.io/framework/core/identity"
)

const (
	secureSessionCookie = "__Host-biz-session"
	secureLoginCookie   = "__Host-biz-login"
	devSessionCookie    = "biz_session"
	devLoginCookie      = "biz_login"
)

type runtimeWebAuth struct {
	config WebAuthConfig
	oidc   *oidcClient
	mu     sync.RWMutex
	store  *accesspersistence.Store
}

func newRuntimeWebAuth(ctx context.Context, config WebAuthConfig) (*runtimeWebAuth, error) {
	result := &runtimeWebAuth{config: config}
	if !config.Enabled() {
		return result, nil
	}
	client, err := newOIDCClient(ctx, config)
	if err != nil {
		return nil, err
	}
	result.oidc = client
	return result, nil
}

func (auth *runtimeWebAuth) enabled() bool {
	return auth != nil && auth.config.Enabled() && auth.oidc != nil
}

func (auth *runtimeWebAuth) setStore(store *accesspersistence.Store) {
	if auth == nil {
		return
	}
	auth.mu.Lock()
	auth.store = store
	auth.mu.Unlock()
}

func (auth *runtimeWebAuth) currentStore() *accesspersistence.Store {
	if auth == nil {
		return nil
	}
	auth.mu.RLock()
	defer auth.mu.RUnlock()
	return auth.store
}

func (auth *runtimeWebAuth) register(mux *http.ServeMux) {
	if !auth.enabled() || mux == nil {
		return
	}
	mux.HandleFunc("GET /auth/login", auth.handleLogin)
	mux.HandleFunc("GET /auth/callback", auth.handleCallback)
	mux.HandleFunc("GET /auth/session", auth.handleSession)
	mux.HandleFunc("GET /auth/session/tenants", auth.handleTenants)
	mux.HandleFunc("POST /auth/session/tenant", auth.handleSwitchTenant)
	mux.HandleFunc("POST /auth/logout", auth.handleLogout)
}

func (auth *runtimeWebAuth) handleLogin(writer http.ResponseWriter, request *http.Request) {
	store := auth.currentStore()
	if store == nil {
		http.Error(writer, "authentication unavailable", http.StatusServiceUnavailable)
		return
	}
	state, err := randomURLSecret(32)
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusServiceUnavailable)
		return
	}
	browser, err := randomURLSecret(32)
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusServiceUnavailable)
		return
	}
	verifier, err := randomURLSecret(32)
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusServiceUnavailable)
		return
	}
	nonce, err := randomURLSecret(32)
	if err != nil {
		http.Error(writer, "authentication unavailable", http.StatusServiceUnavailable)
		return
	}
	returnTo := safeReturnTo(request.URL.Query().Get("return_to"))
	if err := store.CreateWebLoginFlow(request.Context(), state, browser, verifier, nonce, returnTo, auth.config.FlowTTL); err != nil {
		http.Error(writer, "authentication unavailable", http.StatusServiceUnavailable)
		return
	}
	auth.setCookie(writer, auth.loginCookieName(), browser, auth.config.FlowTTL)
	http.Redirect(writer, request, auth.oidc.authorizationURL(state, nonce, pkceChallenge(verifier)), http.StatusFound)
}

func (auth *runtimeWebAuth) handleCallback(writer http.ResponseWriter, request *http.Request) {
	store := auth.currentStore()
	if store == nil {
		http.Error(writer, "authentication unavailable", http.StatusServiceUnavailable)
		return
	}
	if providerError := strings.TrimSpace(request.URL.Query().Get("error")); providerError != "" {
		http.Error(writer, "identity provider rejected login", http.StatusUnauthorized)
		return
	}
	code := strings.TrimSpace(request.URL.Query().Get("code"))
	state := strings.TrimSpace(request.URL.Query().Get("state"))
	loginCookie, err := request.Cookie(auth.loginCookieName())
	if err != nil || code == "" || state == "" || strings.TrimSpace(loginCookie.Value) == "" {
		http.Error(writer, "invalid login callback", http.StatusUnauthorized)
		return
	}
	flow, err := store.ConsumeWebLoginFlow(request.Context(), state, loginCookie.Value)
	if err != nil {
		http.Error(writer, "invalid login callback", http.StatusUnauthorized)
		return
	}
	token, err := auth.oidc.exchange(request.Context(), code, flow.CodeVerifier)
	if err != nil {
		http.Error(writer, "identity provider token exchange failed", http.StatusUnauthorized)
		return
	}
	verified, err := auth.oidc.verifyIDToken(request.Context(), token.IDToken, flow.NonceHash)
	if err != nil {
		http.Error(writer, "identity verification failed", http.StatusUnauthorized)
		return
	}
	webIdentity, err := store.ResolveOrBindOIDCIdentity(request.Context(), verified.Issuer, verified.Subject, verified.Email, verified.EmailVerified)
	if err != nil {
		http.Error(writer, "identity is not provisioned", http.StatusForbidden)
		return
	}
	rawSession, _, err := store.CreateWebSession(request.Context(), webIdentity, auth.config.SessionTTL)
	if err != nil {
		http.Error(writer, "session creation failed", http.StatusServiceUnavailable)
		return
	}
	auth.clearCookie(writer, auth.loginCookieName())
	auth.setCookie(writer, auth.sessionCookieName(), rawSession, auth.config.SessionTTL)
	http.Redirect(writer, request, flow.ReturnTo, http.StatusFound)
}

func (auth *runtimeWebAuth) handleSession(writer http.ResponseWriter, request *http.Request) {
	authentication, _, err := auth.authenticateSession(request)
	if err != nil {
		writeJSON(writer, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	writeJSON(writer, http.StatusOK, sessionResponse(authentication))
}

func (auth *runtimeWebAuth) handleTenants(writer http.ResponseWriter, request *http.Request) {
	authentication, _, err := auth.authenticateSession(request)
	if err != nil {
		http.Error(writer, "Unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"tenants":          authentication.Session.Tenants,
		"active_tenant_id": authentication.Session.ActiveTenantID,
	})
}

func (auth *runtimeWebAuth) handleSwitchTenant(writer http.ResponseWriter, request *http.Request) {
	authentication, rawSession, err := auth.authenticateSession(request)
	if err != nil {
		http.Error(writer, "Unauthorized", http.StatusUnauthorized)
		return
	}
	store := auth.currentStore()
	if store == nil || !store.ValidateWebSessionCSRF(authentication, request.Header.Get("X-CSRF-Token")) {
		http.Error(writer, "Forbidden", http.StatusForbidden)
		return
	}
	var input struct {
		TenantID string `json:"tenant_id"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.TenantID) == "" {
		http.Error(writer, "invalid tenant selection", http.StatusBadRequest)
		return
	}
	updated, err := store.SwitchWebSessionTenant(request.Context(), rawSession, input.TenantID)
	if err != nil {
		http.Error(writer, "tenant selection denied", http.StatusForbidden)
		return
	}
	writeJSON(writer, http.StatusOK, sessionResponse(updated))
}

func (auth *runtimeWebAuth) handleLogout(writer http.ResponseWriter, request *http.Request) {
	authentication, rawSession, err := auth.authenticateSession(request)
	if err == nil {
		store := auth.currentStore()
		if store == nil || !store.ValidateWebSessionCSRF(authentication, request.Header.Get("X-CSRF-Token")) {
			http.Error(writer, "Forbidden", http.StatusForbidden)
			return
		}
		if err := store.RevokeWebSession(request.Context(), rawSession); err != nil {
			http.Error(writer, "logout failed", http.StatusServiceUnavailable)
			return
		}
	}
	auth.clearCookie(writer, auth.sessionCookieName())
	writer.WriteHeader(http.StatusNoContent)
}

func (auth *runtimeWebAuth) authenticateSession(request *http.Request) (accesspersistence.WebSessionAuthentication, string, error) {
	if !auth.enabled() || request == nil {
		return accesspersistence.WebSessionAuthentication{}, "", accesspersistence.ErrWebSessionInvalid
	}
	cookie, err := request.Cookie(auth.sessionCookieName())
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return accesspersistence.WebSessionAuthentication{}, "", accesspersistence.ErrWebSessionInvalid
	}
	store := auth.currentStore()
	if store == nil {
		return accesspersistence.WebSessionAuthentication{}, "", accesspersistence.ErrWebSessionInvalid
	}
	authentication, err := store.AuthenticateWebSession(request.Context(), cookie.Value)
	return authentication, cookie.Value, err
}

func (auth *runtimeWebAuth) authenticateAPI(request *http.Request) (identity.Principal, error) {
	authentication, _, err := auth.authenticateSession(request)
	if err != nil {
		return identity.Principal{}, err
	}
	if err := validateExpectedWebSession(request.Header.Get("X-Biz-Session-Context"), authentication.Session); err != nil {
		return identity.Principal{}, err
	}
	if authentication.Session.ActorKind == accesspersistence.WebActorUser && authentication.Session.ActiveTenantID == "" {
		return identity.Principal{}, accesspersistence.ErrWebTenantDenied
	}
	if isUnsafeMethod(request.Method) {
		store := auth.currentStore()
		if store == nil || !store.ValidateWebSessionCSRF(authentication, request.Header.Get("X-CSRF-Token")) {
			return identity.Principal{}, accesspersistence.ErrWebSessionInvalid
		}
	}
	if !authentication.Principal.Authenticated {
		return identity.Principal{}, accesspersistence.ErrWebSessionInvalid
	}
	return authentication.Principal, nil
}

func sessionResponse(authentication accesspersistence.WebSessionAuthentication) map[string]any {
	response := map[string]any{
		"authenticated":    true,
		"actor_kind":       authentication.Session.ActorKind,
		"active_tenant_id": authentication.Session.ActiveTenantID,
		"tenants":          authentication.Session.Tenants,
		"expires_at":       authentication.Session.ExpiresAt.UTC().Format(time.RFC3339),
		"csrf_token":       authentication.Session.CSRFToken,
	}
	if authentication.Session.UserID != "" {
		response["user_id"] = authentication.Session.UserID
	}
	if authentication.Session.PlatformSubject != "" {
		response["platform_subject"] = authentication.Session.PlatformSubject
	}
	if len(authentication.Principal.Roles) > 0 {
		response["roles"] = authentication.Principal.Roles
	}
	return response
}

func (auth *runtimeWebAuth) sessionCookieName() string {
	if auth.config.CookieSecure {
		return secureSessionCookie
	}
	return devSessionCookie
}

func (auth *runtimeWebAuth) loginCookieName() string {
	if auth.config.CookieSecure {
		return secureLoginCookie
	}
	return devLoginCookie
}

func (auth *runtimeWebAuth) setCookie(writer http.ResponseWriter, name, value string, ttl time.Duration) {
	http.SetCookie(writer, &http.Cookie{
		Name: name, Value: value, Path: "/", HttpOnly: true, Secure: auth.config.CookieSecure,
		SameSite: http.SameSiteLaxMode, MaxAge: int(ttl.Seconds()), Expires: time.Now().UTC().Add(ttl),
	})
}

func (auth *runtimeWebAuth) clearCookie(writer http.ResponseWriter, name string) {
	http.SetCookie(writer, &http.Cookie{
		Name: name, Value: "", Path: "/", HttpOnly: true, Secure: auth.config.CookieSecure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0).UTC(),
	})
}

func safeReturnTo(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "/"
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(raw, "//") {
		return "/"
	}
	return parsed.RequestURI()
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func randomURLSecret(size int) (string, error) {
	if size < 16 {
		return "", errors.New("biz runtime: random secret size too small")
	}
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func constantTimeEqual(left, right string) bool {
	if len(left) == 0 || len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func (auth *runtimeWebAuth) bootstrapPlatformIdentity(ctx context.Context) error {
	if auth == nil || !auth.enabled() || strings.TrimSpace(auth.config.PlatformExternalSubject) == "" {
		return nil
	}
	store := auth.currentStore()
	if store == nil {
		return errors.New("biz runtime: OIDC platform identity store unavailable")
	}
	return store.BindOIDCPlatformIdentity(ctx, strings.TrimRight(auth.config.IssuerURL, "/"), auth.config.PlatformExternalSubject, auth.config.PlatformSubject, auth.config.PlatformEmail)
}

func (auth *runtimeWebAuth) describeProvider() string {
	if auth == nil || auth.oidc == nil {
		return ""
	}
	return fmt.Sprintf("%s (%s)", auth.oidc.provider.Issuer, auth.config.ClientID)
}
