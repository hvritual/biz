package bizruntime

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"html/template"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

const (
	secureIDPAuthCookie = "__Host-biz-idp-auth"
	devIDPAuthCookie    = "biz_idp_auth"
)

type firstPartyVerificationKey struct {
	kid string
	key *rsa.PublicKey
}

type runtimeFirstPartyIdP struct {
	config           FirstPartyIdPConfig
	key              *rsa.PrivateKey
	kid              string
	verificationKeys []firstPartyVerificationKey
	mu               sync.RWMutex
	store            *accesspersistence.Store
}

func newRuntimeFirstPartyIdP(config FirstPartyIdPConfig) (*runtimeFirstPartyIdP, error) {
	result := &runtimeFirstPartyIdP{config: config}
	if !config.Enabled() {
		return result, nil
	}
	key, err := parseRSAPrivateKey([]byte(config.SigningKeyPEM))
	if err != nil {
		return nil, err
	}
	if key.N.BitLen() < 2048 {
		return nil, errors.New("biz runtime: first-party IdP RSA signing key must be at least 2048 bits")
	}
	kid, err := rsaKeyID(&key.PublicKey, config.SigningKeyID)
	if err != nil {
		return nil, err
	}
	verificationKeys := []firstPartyVerificationKey{{kid: kid, key: &key.PublicKey}}
	seen := map[string]struct{}{kid: {}}
	for _, previous := range config.PreviousSigningKeys {
		public, err := parseRSAPublicKey([]byte(previous.PEM))
		if err != nil {
			return nil, err
		}
		if public.N.BitLen() < 2048 {
			return nil, errors.New("biz runtime: previous first-party IdP RSA key must be at least 2048 bits")
		}
		previousKid, err := rsaKeyID(public, previous.KeyID)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[previousKid]; duplicate {
			return nil, errors.New("biz runtime: duplicate first-party IdP signing key id")
		}
		seen[previousKid] = struct{}{}
		verificationKeys = append(verificationKeys, firstPartyVerificationKey{kid: previousKid, key: public})
	}
	result.key = key
	result.kid = kid
	result.verificationKeys = verificationKeys
	return result, nil
}

func (idp *runtimeFirstPartyIdP) enabled() bool {
	return idp != nil && idp.config.Enabled() && idp.key != nil
}

func (idp *runtimeFirstPartyIdP) setStore(store *accesspersistence.Store) {
	if idp == nil {
		return
	}
	idp.mu.Lock()
	idp.store = store
	idp.mu.Unlock()
}

func (idp *runtimeFirstPartyIdP) currentStore() *accesspersistence.Store {
	if idp == nil {
		return nil
	}
	idp.mu.RLock()
	defer idp.mu.RUnlock()
	return idp.store
}

func (idp *runtimeFirstPartyIdP) providerMetadata() oidcProviderMetadata {
	issuer := idp.config.IssuerURL()
	return oidcProviderMetadata{
		Issuer:                      issuer,
		AuthorizationEndpoint:       issuer + "/authorize",
		TokenEndpoint:               issuer + "/token",
		JWKSURI:                     issuer + "/jwks",
		EndSessionEndpoint:          issuer + "/logout",
		IDTokenSigningAlgsSupported: []string{"RS256"},
	}
}

func (idp *runtimeFirstPartyIdP) register(mux *http.ServeMux) {
	if !idp.enabled() || mux == nil {
		return
	}
	mux.HandleFunc("GET /idp/.well-known/openid-configuration", idp.handleDiscovery)
	mux.HandleFunc("GET /idp/jwks", idp.handleJWKS)
	mux.HandleFunc("GET /idp/authorize", idp.handleAuthorize)
	mux.HandleFunc("POST /idp/login", idp.handleLogin)
	mux.HandleFunc("POST /idp/token", idp.handleToken)
	mux.HandleFunc("GET /idp/logout", idp.handleProviderLogout)
}

func (idp *runtimeFirstPartyIdP) handleDiscovery(writer http.ResponseWriter, _ *http.Request) {
	provider := idp.providerMetadata()
	writeJSON(writer, http.StatusOK, map[string]any{
		"issuer":                                provider.Issuer,
		"authorization_endpoint":                provider.AuthorizationEndpoint,
		"token_endpoint":                        provider.TokenEndpoint,
		"jwks_uri":                              provider.JWKSURI,
		"end_session_endpoint":                  provider.EndSessionEndpoint,
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported":                      []string{"openid", "profile", "email"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func (idp *runtimeFirstPartyIdP) handleJWKS(writer http.ResponseWriter, _ *http.Request) {
	keys := make([]map[string]any, 0, len(idp.verificationKeys))
	for _, verification := range idp.verificationKeys {
		keys = append(keys, map[string]any{
			"kty": "RSA",
			"use": "sig",
			"alg": "RS256",
			"kid": verification.kid,
			"n":   base64.RawURLEncoding.EncodeToString(verification.key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(rsaExponentBytes(verification.key.E)),
		})
	}
	writeJSON(writer, http.StatusOK, map[string]any{"keys": keys})
}

func (idp *runtimeFirstPartyIdP) handleAuthorize(writer http.ResponseWriter, request *http.Request) {
	store := idp.currentStore()
	if store == nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	query := request.URL.Query()
	if query.Get("response_type") != "code" || query.Get("client_id") != idp.config.ClientID || query.Get("redirect_uri") != idp.config.RedirectURL {
		http.Error(writer, "invalid authorization request", http.StatusBadRequest)
		return
	}
	state := strings.TrimSpace(query.Get("state"))
	nonce := strings.TrimSpace(query.Get("nonce"))
	challenge := strings.TrimSpace(query.Get("code_challenge"))
	scope := strings.TrimSpace(query.Get("scope"))
	if state == "" || nonce == "" || challenge == "" || query.Get("code_challenge_method") != "S256" || !scopeContains(scope, "openid") {
		http.Error(writer, "invalid authorization request", http.StatusBadRequest)
		return
	}
	requestID, browserSecret, csrf, err := store.CreateFirstPartyAuthorizationRequest(request.Context(), accesspersistence.FirstPartyAuthorizationRequestInput{
		ClientID:      idp.config.ClientID,
		RedirectURI:   idp.config.RedirectURL,
		State:         state,
		Nonce:         nonce,
		CodeChallenge: challenge,
		Scope:         scope,
	}, idp.config.LoginTTL)
	if err != nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	idp.setAuthCookie(writer, browserSecret, idp.config.LoginTTL)
	idp.renderLogin(writer, http.StatusOK, requestID, csrf, "", "")
}

func (idp *runtimeFirstPartyIdP) handleLogin(writer http.ResponseWriter, request *http.Request) {
	store := idp.currentStore()
	if store == nil {
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
	if err := request.ParseForm(); err != nil {
		http.Error(writer, "invalid login request", http.StatusBadRequest)
		return
	}
	requestID := strings.TrimSpace(request.Form.Get("request_id"))
	csrf := strings.TrimSpace(request.Form.Get("csrf_token"))
	email := strings.TrimSpace(request.Form.Get("email"))
	password := request.Form.Get("password")
	cookie, err := request.Cookie(idp.authCookieName())
	if err != nil || requestID == "" || csrf == "" || strings.TrimSpace(cookie.Value) == "" {
		http.Error(writer, "invalid login request", http.StatusUnauthorized)
		return
	}
	if _, err := store.LoadFirstPartyAuthorizationRequest(request.Context(), requestID, cookie.Value, csrf); err != nil {
		http.Error(writer, "invalid login request", http.StatusUnauthorized)
		return
	}
	identity, err := store.AuthenticateFirstPartyLogin(request.Context(), email, password, request.RemoteAddr, accesspersistence.DefaultFirstPartyLoginPolicy())
	if err != nil {
		if errors.Is(err, accesspersistence.ErrInvalidUserCredentials) {
			idp.renderLogin(writer, http.StatusUnauthorized, requestID, csrf, email, "邮箱或密码错误")
			return
		}
		http.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)
		return
	}
	code, authorization, err := store.IssueFirstPartyAuthorizationCode(request.Context(), requestID, cookie.Value, csrf, identity.UserID, idp.config.CodeTTL)
	if err != nil {
		http.Error(writer, "login transaction expired", http.StatusUnauthorized)
		return
	}
	idp.clearAuthCookie(writer)
	redirect, _ := url.Parse(authorization.RedirectURI)
	values := redirect.Query()
	values.Set("code", code)
	values.Set("state", authorization.State)
	redirect.RawQuery = values.Encode()
	http.Redirect(writer, request, redirect.String(), http.StatusFound)
}

func (idp *runtimeFirstPartyIdP) handleToken(writer http.ResponseWriter, request *http.Request) {
	store := idp.currentStore()
	if store == nil {
		writeOAuthError(writer, http.StatusServiceUnavailable, "temporarily_unavailable", "identity provider unavailable")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 16<<10)
	if err := request.ParseForm(); err != nil {
		writeOAuthError(writer, http.StatusBadRequest, "invalid_request", "invalid token request")
		return
	}
	if request.Form.Get("grant_type") != "authorization_code" || request.Form.Get("client_id") != idp.config.ClientID || request.Form.Get("redirect_uri") != idp.config.RedirectURL {
		writeOAuthError(writer, http.StatusBadRequest, "invalid_grant", "invalid authorization code grant")
		return
	}
	code := strings.TrimSpace(request.Form.Get("code"))
	verifier := strings.TrimSpace(request.Form.Get("code_verifier"))
	if code == "" || len(verifier) < 43 || len(verifier) > 128 {
		writeOAuthError(writer, http.StatusBadRequest, "invalid_grant", "invalid authorization code grant")
		return
	}
	grant, err := store.ConsumeFirstPartyAuthorizationCode(request.Context(), code, idp.config.ClientID, idp.config.RedirectURL, verifier)
	if err != nil {
		writeOAuthError(writer, http.StatusBadRequest, "invalid_grant", "invalid authorization code grant")
		return
	}
	idToken, err := idp.signIDToken(grant)
	if err != nil {
		writeOAuthError(writer, http.StatusServiceUnavailable, "temporarily_unavailable", "token signing failed")
		return
	}
	accessToken, err := randomURLSecret(32)
	if err != nil {
		writeOAuthError(writer, http.StatusServiceUnavailable, "temporarily_unavailable", "token creation failed")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int64(idp.config.TokenTTL.Seconds()),
		"id_token":     idToken,
		"scope":        grant.Scope,
	})
}

func (idp *runtimeFirstPartyIdP) handleProviderLogout(writer http.ResponseWriter, request *http.Request) {
	expected := strings.TrimSpace(idp.config.PostLogoutRedirectURL)
	if expected == "" {
		expected = strings.TrimRight(idp.config.PublicURL, "/") + "/"
	}
	target := strings.TrimSpace(request.URL.Query().Get("post_logout_redirect_uri"))
	if target == "" {
		target = expected
	}
	if target != expected {
		http.Error(writer, "invalid post logout redirect", http.StatusBadRequest)
		return
	}
	idp.clearAuthCookie(writer)
	http.Redirect(writer, request, target, http.StatusFound)
}

func (idp *runtimeFirstPartyIdP) signIDToken(grant accesspersistence.FirstPartyAuthorizationGrant) (string, error) {
	now := time.Now().UTC()
	header := map[string]any{"alg": "RS256", "kid": idp.kid, "typ": "JWT"}
	claims := map[string]any{
		"iss":            idp.config.IssuerURL(),
		"sub":            "biz-user:" + grant.UserID,
		"aud":            idp.config.ClientID,
		"exp":            now.Add(idp.config.TokenTTL).Unix(),
		"iat":            now.Unix(),
		"nonce":          grant.Nonce,
		"email":          grant.Email,
		"email_verified": true,
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(nil, idp.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (idp *runtimeFirstPartyIdP) renderLogin(writer http.ResponseWriter, status int, requestID, csrf, email, message string) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	formActions := []string{"'self'"}
	seenOrigins := map[string]struct{}{}
	for _, raw := range []string{idp.config.PublicURL, idp.config.RedirectURL} {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			continue
		}
		origin := parsed.Scheme + "://" + parsed.Host
		if _, duplicate := seenOrigins[origin]; duplicate {
			continue
		}
		seenOrigins[origin] = struct{}{}
		formActions = append(formActions, origin)
	}
	writer.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action "+strings.Join(formActions, " ")+"; base-uri 'none'; frame-ancestors 'none'")
	// A navigate-mode HTML form POST needs a non-null Origin so the IdP
	// transaction cookie remains same-site. `origin` still strips the OIDC
	// authorization path/query from Referer, avoiding state/nonce leakage.
	writer.Header().Set("Referrer-Policy", "origin")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("X-Frame-Options", "DENY")
	writer.WriteHeader(status)
	_ = firstPartyLoginTemplate.Execute(writer, map[string]string{
		"RequestID": requestID,
		"CSRF":      csrf,
		"Email":     email,
		"Message":   message,
	})
}

func (idp *runtimeFirstPartyIdP) authCookieName() string {
	if idp.config.CookieSecure {
		return secureIDPAuthCookie
	}
	return devIDPAuthCookie
}

func (idp *runtimeFirstPartyIdP) setAuthCookie(writer http.ResponseWriter, value string, ttl time.Duration) {
	http.SetCookie(writer, &http.Cookie{
		Name: idp.authCookieName(), Value: value, Path: "/", HttpOnly: true, Secure: idp.config.CookieSecure,
		SameSite: http.SameSiteLaxMode, MaxAge: int(ttl.Seconds()), Expires: time.Now().UTC().Add(ttl),
	})
}

func (idp *runtimeFirstPartyIdP) clearAuthCookie(writer http.ResponseWriter) {
	http.SetCookie(writer, &http.Cookie{
		Name: idp.authCookieName(), Value: "", Path: "/", HttpOnly: true, Secure: idp.config.CookieSecure,
		SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0).UTC(),
	})
}

func writeOAuthError(writer http.ResponseWriter, status int, code, description string) {
	writeJSON(writer, status, map[string]any{"error": code, "error_description": description})
}

func scopeContains(scope, expected string) bool {
	for _, value := range strings.Fields(scope) {
		if value == expected {
			return true
		}
	}
	return false
}

func parseRSAPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("biz runtime: invalid first-party IdP signing key PEM")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	value, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("biz runtime: invalid first-party IdP RSA private key")
	}
	key, ok := value.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("biz runtime: first-party IdP signing key must be RSA")
	}
	return key, nil
}

func parseRSAPublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("biz runtime: invalid previous first-party IdP signing key PEM")
	}
	if value, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if key, ok := value.(*rsa.PublicKey); ok {
			return key, nil
		}
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := parseRSAPrivateKey(data); err == nil {
		return &key.PublicKey, nil
	}
	return nil, errors.New("biz runtime: previous first-party IdP signing key must be RSA")
}

func rsaKeyID(key *rsa.PublicKey, configured string) (string, error) {
	if configured = strings.TrimSpace(configured); configured != "" {
		return configured, nil
	}
	encoded, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return base64.RawURLEncoding.EncodeToString(digest[:12]), nil
}

func rsaExponentBytes(exponent int) []byte {
	if exponent <= 0 {
		return nil
	}
	return new(big.Int).SetInt64(int64(exponent)).Bytes()
}

var firstPartyLoginTemplate = template.Must(template.New("first-party-idp-login").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>CoffeeLink 登录</title>
<style>
:root{font-family:Inter,"PingFang SC","Microsoft YaHei",sans-serif;color:#172033;background:#f5f7fb}*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;background:radial-gradient(circle at top,#fff 0,#f5f7fb 54%,#eef2f7 100%)}main{width:min(420px,calc(100vw - 32px));background:#fff;border:1px solid #e7eaf0;border-radius:16px;padding:32px;box-shadow:0 20px 60px rgba(22,34,51,.10)}.brand{display:flex;align-items:center;gap:10px;font-weight:700;font-size:18px}.mark{width:32px;height:32px;border-radius:10px;background:#fc610e;display:grid;place-items:center;color:white}h1{font-size:24px;margin:28px 0 8px}p{margin:0 0 24px;color:#6b7280;font-size:14px}.field{display:grid;gap:8px;margin:16px 0}label{font-size:13px;font-weight:600}input{width:100%;border:1px solid #d9dee8;border-radius:8px;padding:11px 12px;font:inherit;outline:none}input:focus{border-color:#fc610e;box-shadow:0 0 0 3px rgba(252,97,14,.12)}button{width:100%;margin-top:20px;border:0;border-radius:8px;background:#fc610e;color:#fff;padding:12px;font:inherit;font-weight:700;cursor:pointer}.error{padding:10px 12px;border-radius:8px;background:#fff1eb;color:#b53c00;font-size:13px;margin-bottom:12px}
</style>
</head>
<body>
<main>
<div class="brand"><div class="mark">C</div><span>CoffeeLink</span></div>
<h1>CoffeeLink 登录</h1>
<p>使用企业成员账号继续访问。</p>
{{if .Message}}<div class="error" role="alert">{{.Message}}</div>{{end}}
<form method="post" action="/idp/login" autocomplete="on">
<input type="hidden" name="request_id" value="{{.RequestID}}">
<input type="hidden" name="csrf_token" value="{{.CSRF}}">
<div class="field"><label for="email">邮箱</label><input id="email" name="email" type="email" value="{{.Email}}" autocomplete="username" required></div>
<div class="field"><label for="password">密码</label><input id="password" name="password" type="password" autocomplete="current-password" required></div>
<button type="submit">登录</button>
</form>
</main>
</body>
</html>`))

func (idp *runtimeFirstPartyIdP) ensureReady(ctx context.Context) error {
	if !idp.enabled() {
		return nil
	}
	if idp.currentStore() == nil {
		return fmt.Errorf("biz runtime: first-party IdP store unavailable")
	}
	return nil
}
