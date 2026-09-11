package bizruntime

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type oidcProviderMetadata struct {
	Issuer                      string   `json:"issuer"`
	AuthorizationEndpoint       string   `json:"authorization_endpoint"`
	TokenEndpoint               string   `json:"token_endpoint"`
	JWKSURI                     string   `json:"jwks_uri"`
	EndSessionEndpoint          string   `json:"end_session_endpoint"`
	IDTokenSigningAlgsSupported []string `json:"id_token_signing_alg_values_supported"`
}

type oidcTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

type oidcClaims struct {
	Issuer          string          `json:"iss"`
	Subject         string          `json:"sub"`
	Audience        json.RawMessage `json:"aud"`
	AuthorizedParty string          `json:"azp"`
	ExpiresAt       int64           `json:"exp"`
	IssuedAt        int64           `json:"iat"`
	NotBefore       int64           `json:"nbf"`
	Nonce           string          `json:"nonce"`
	Email           string          `json:"email"`
	EmailVerified   bool            `json:"email_verified"`
	Name            string          `json:"name"`
}

type oidcVerifiedIdentity struct {
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

type oidcJWKSet struct {
	Keys []oidcJWK `json:"keys"`
}

type oidcJWK struct {
	KTY string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	KID string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type oidcClient struct {
	config   WebAuthConfig
	provider oidcProviderMetadata
	http     *http.Client
}

func newOIDCClient(ctx context.Context, config WebAuthConfig) (*oidcClient, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	provider, err := discoverOIDCProvider(ctx, client, config.IssuerURL)
	if err != nil {
		return nil, err
	}
	if provider.Issuer != strings.TrimRight(config.IssuerURL, "/") {
		return nil, fmt.Errorf("biz runtime: OIDC discovery issuer mismatch: got %q", provider.Issuer)
	}
	if !containsString(provider.IDTokenSigningAlgsSupported, "RS256") {
		return nil, errors.New("biz runtime: OIDC provider must support RS256 ID tokens")
	}
	return &oidcClient{config: config, provider: provider, http: client}, nil
}

func discoverOIDCProvider(ctx context.Context, client *http.Client, issuer string) (oidcProviderMetadata, error) {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return oidcProviderMetadata{}, err
	}
	response, err := client.Do(request)
	if err != nil {
		return oidcProviderMetadata{}, fmt.Errorf("biz runtime: OIDC discovery: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return oidcProviderMetadata{}, fmt.Errorf("biz runtime: OIDC discovery status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return oidcProviderMetadata{}, err
	}
	var provider oidcProviderMetadata
	if err := json.Unmarshal(body, &provider); err != nil {
		return oidcProviderMetadata{}, fmt.Errorf("biz runtime: OIDC discovery decode: %w", err)
	}
	if strings.TrimSpace(provider.Issuer) == "" || strings.TrimSpace(provider.AuthorizationEndpoint) == "" || strings.TrimSpace(provider.TokenEndpoint) == "" || strings.TrimSpace(provider.JWKSURI) == "" {
		return oidcProviderMetadata{}, errors.New("biz runtime: OIDC discovery is missing required endpoints")
	}
	for _, endpoint := range []string{provider.AuthorizationEndpoint, provider.TokenEndpoint, provider.JWKSURI} {
		if err := requireHTTPSOrLoopback(endpoint); err != nil {
			return oidcProviderMetadata{}, err
		}
	}
	return provider, nil
}

func (client *oidcClient) authorizationURL(state, nonce, codeChallenge string) string {
	values := url.Values{}
	values.Set("response_type", "code")
	values.Set("client_id", client.config.ClientID)
	values.Set("redirect_uri", client.config.RedirectURL)
	values.Set("scope", strings.Join(client.config.Scopes, " "))
	values.Set("state", state)
	values.Set("nonce", nonce)
	values.Set("code_challenge", codeChallenge)
	values.Set("code_challenge_method", "S256")
	return client.provider.AuthorizationEndpoint + "?" + values.Encode()
}

func (client *oidcClient) exchange(ctx context.Context, code, codeVerifier string) (oidcTokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", client.config.RedirectURL)
	values.Set("client_id", client.config.ClientID)
	values.Set("code_verifier", codeVerifier)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.provider.TokenEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return oidcTokenResponse{}, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	if client.config.ClientSecret != "" {
		request.SetBasicAuth(client.config.ClientID, client.config.ClientSecret)
	}
	response, err := client.http.Do(request)
	if err != nil {
		return oidcTokenResponse{}, fmt.Errorf("biz runtime: OIDC token exchange: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return oidcTokenResponse{}, err
	}
	var token oidcTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return oidcTokenResponse{}, fmt.Errorf("biz runtime: OIDC token response decode: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || token.Error != "" {
		return oidcTokenResponse{}, fmt.Errorf("biz runtime: OIDC token exchange rejected: %s %s", token.Error, token.Description)
	}
	if strings.TrimSpace(token.IDToken) == "" {
		return oidcTokenResponse{}, errors.New("biz runtime: OIDC token response missing id_token")
	}
	return token, nil
}

func (client *oidcClient) verifyIDToken(ctx context.Context, rawToken, expectedNonceHash string) (oidcVerifiedIdentity, error) {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return oidcVerifiedIdentity{}, errors.New("biz runtime: malformed OIDC id_token")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oidcVerifiedIdentity{}, errors.New("biz runtime: malformed OIDC id_token header")
	}
	var header struct {
		Alg string `json:"alg"`
		KID string `json:"kid"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return oidcVerifiedIdentity{}, errors.New("biz runtime: malformed OIDC id_token header")
	}
	if header.Alg != "RS256" || strings.TrimSpace(header.KID) == "" {
		return oidcVerifiedIdentity{}, errors.New("biz runtime: OIDC id_token must use RS256 with kid")
	}
	key, err := client.fetchRSAKey(ctx, header.KID)
	if err != nil {
		return oidcVerifiedIdentity{}, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return oidcVerifiedIdentity{}, errors.New("biz runtime: malformed OIDC id_token signature")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return oidcVerifiedIdentity{}, errors.New("biz runtime: OIDC id_token signature invalid")
	}
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return oidcVerifiedIdentity{}, errors.New("biz runtime: malformed OIDC id_token claims")
	}
	var claims oidcClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return oidcVerifiedIdentity{}, errors.New("biz runtime: malformed OIDC id_token claims")
	}
	if err := client.validateClaims(claims, expectedNonceHash, time.Now().UTC()); err != nil {
		return oidcVerifiedIdentity{}, err
	}
	return oidcVerifiedIdentity{Issuer: claims.Issuer, Subject: claims.Subject, Email: claims.Email, EmailVerified: claims.EmailVerified, Name: claims.Name}, nil
}

func (client *oidcClient) validateClaims(claims oidcClaims, expectedNonceHash string, now time.Time) error {
	const skew = 60 * time.Second
	if claims.Issuer != strings.TrimRight(client.config.IssuerURL, "/") || strings.TrimSpace(claims.Subject) == "" {
		return errors.New("biz runtime: OIDC id_token issuer or subject invalid")
	}
	audiences, err := decodeAudience(claims.Audience)
	if err != nil || !containsString(audiences, client.config.ClientID) {
		return errors.New("biz runtime: OIDC id_token audience invalid")
	}
	if len(audiences) > 1 && claims.AuthorizedParty != client.config.ClientID {
		return errors.New("biz runtime: OIDC id_token azp invalid")
	}
	if claims.ExpiresAt == 0 || !time.Unix(claims.ExpiresAt, 0).After(now.Add(-skew)) {
		return errors.New("biz runtime: OIDC id_token expired")
	}
	if claims.NotBefore != 0 && time.Unix(claims.NotBefore, 0).After(now.Add(skew)) {
		return errors.New("biz runtime: OIDC id_token not active")
	}
	if claims.IssuedAt != 0 && time.Unix(claims.IssuedAt, 0).After(now.Add(skew)) {
		return errors.New("biz runtime: OIDC id_token issued in the future")
	}
	if strings.TrimSpace(claims.Nonce) == "" || TokenHashForWeb(claims.Nonce) != expectedNonceHash {
		return errors.New("biz runtime: OIDC id_token nonce invalid")
	}
	return nil
}

func (client *oidcClient) fetchRSAKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.provider.JWKSURI, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("biz runtime: OIDC JWKS fetch: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("biz runtime: OIDC JWKS status %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var set oidcJWKSet
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("biz runtime: OIDC JWKS decode: %w", err)
	}
	for _, candidate := range set.Keys {
		if candidate.KID != kid || candidate.KTY != "RSA" || (candidate.Use != "" && candidate.Use != "sig") || (candidate.Alg != "" && candidate.Alg != "RS256") {
			continue
		}
		key, err := rsaKeyFromJWK(candidate)
		if err != nil {
			return nil, err
		}
		return key, nil
	}
	return nil, errors.New("biz runtime: OIDC signing key not found")
}

func rsaKeyFromJWK(jwk oidcJWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil || len(nBytes) == 0 {
		return nil, errors.New("biz runtime: OIDC RSA modulus invalid")
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil || len(eBytes) == 0 || len(eBytes) > 4 {
		return nil, errors.New("biz runtime: OIDC RSA exponent invalid")
	}
	exponent := 0
	for _, value := range eBytes {
		exponent = exponent<<8 + int(value)
	}
	if exponent < 3 {
		return nil, errors.New("biz runtime: OIDC RSA exponent invalid")
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: exponent}, nil
}

func decodeAudience(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, errors.New("missing audience")
	}
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		if strings.TrimSpace(single) == "" {
			return nil, errors.New("empty audience")
		}
		return []string{single}, nil
	}
	var multiple []string
	if err := json.Unmarshal(raw, &multiple); err != nil || len(multiple) == 0 {
		return nil, errors.New("invalid audience")
	}
	return multiple, nil
}

func pkceChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func requireHTTPSOrLoopback(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("biz runtime: invalid secure URL %q", raw)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" {
		host := parsed.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
	}
	return fmt.Errorf("biz runtime: URL must use HTTPS outside loopback: %q", raw)
}

func parseDurationSeconds(value string, fallback time.Duration) time.Duration {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func TokenHashForWeb(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", digest[:])
}
