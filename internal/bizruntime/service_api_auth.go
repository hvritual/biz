package bizruntime

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	accessauthorization "github.com/hvritual/biz/internal/access/authorization"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"yunka.io/framework/core/identity"
)

const (
	serviceAPIKeyIDHeader     = "X-Biz-Service-Key-Id"
	serviceAPITimestampHeader = "X-Biz-Service-Timestamp"
	serviceAPINonceHeader     = "X-Biz-Service-Nonce"
	serviceAPISignatureHeader = "X-Biz-Service-Signature"
	serviceAPITraceHeader     = "X-Biz-Trace-Id"
	serviceAPIFailureHeader   = "X-Biz-Auth-Failure"
	serviceAPISignaturePrefix = "BIZ-HMAC-SHA256-V1"
	serviceAPIMaxBodyBytes    = int64(16 << 20)
)

var (
	errServiceAPIMissingSignature = errors.New("service api: signature required")
	errServiceAPIInvalidRequest   = errors.New("service api: invalid signed request")
	errServiceAPIStaleRequest     = errors.New("service api: timestamp outside allowed window")
	errServiceAPIReplay           = errors.New("service api: nonce replay")
	errServiceAPIUnavailable      = errors.New("service api: verifier unavailable")
)

type ServiceAPICredentialConfig struct {
	KeyID      string
	Subject    string
	TenantID   string
	Secret     []byte
	Operations []string
	NotBefore  *time.Time
	ExpiresAt  *time.Time
}

type ServiceAPIAuthConfig struct {
	ClockSkew   time.Duration
	Credentials []ServiceAPICredentialConfig
}

func (config ServiceAPIAuthConfig) Enabled() bool { return len(config.Credentials) > 0 }

func (config ServiceAPIAuthConfig) Validate() error {
	if !config.Enabled() {
		if config.ClockSkew < 0 {
			return errors.New("biz runtime: service api clock skew must not be negative")
		}
		return nil
	}
	if config.ClockSkew <= 0 || config.ClockSkew > 15*time.Minute {
		return errors.New("biz runtime: service api clock skew must be explicitly set between 1ns and 15m")
	}
	catalog := accessauthorization.Catalog()
	actions := make(map[string]accessauthorization.Action, len(catalog))
	for _, action := range catalog {
		actions[action.Code] = action
	}
	seen := map[string]struct{}{}
	for _, credential := range config.Credentials {
		keyID := strings.TrimSpace(credential.KeyID)
		subject := strings.TrimSpace(credential.Subject)
		if !validServiceAPIIdentifier(keyID, 1, 128) || subject == "" || len(credential.Secret) < 32 {
			return errors.New("biz runtime: service api credentials require key id, subject and at least 32 secret bytes")
		}
		if _, exists := seen[keyID]; exists {
			return fmt.Errorf("biz runtime: duplicate service api key id %q", keyID)
		}
		seen[keyID] = struct{}{}
		if credential.NotBefore != nil && credential.ExpiresAt != nil && !credential.ExpiresAt.After(*credential.NotBefore) {
			return fmt.Errorf("biz runtime: service api key %q expiry must be after not-before", keyID)
		}
		operations := canonicalStringSet(credential.Operations)
		if len(operations) == 0 {
			return fmt.Errorf("biz runtime: service api key %q requires at least one operation", keyID)
		}
		for _, operation := range operations {
			action, ok := actions[operation]
			if !ok || !containsExact(action.Authentication, "api-key") || len(action.HTTP) == 0 {
				return fmt.Errorf("biz runtime: service api operation %q is not an HTTP api-key operation", operation)
			}
			if action.TenantRequired {
				return fmt.Errorf("biz runtime: service api operation %q requires a user tenant principal and cannot be assigned to a service credential", operation)
			}
			if strings.TrimSpace(credential.TenantID) != "" && !actionHTTPHasTenantParameter(action) {
				return fmt.Errorf("biz runtime: tenant-scoped service api key %q cannot bind operation %q without tenant_id path authority", keyID, operation)
			}
		}
	}
	return nil
}

type serviceAPIAuthenticator struct {
	mu          sync.RWMutex
	store       *accesspersistence.Store
	clockSkew   time.Duration
	credentials map[string]ServiceAPICredentialConfig
	required    map[string]struct{}
	routes      []serviceAPIRoute
	now         func() time.Time
}

type serviceAPIRoute struct {
	Operation string
	Method    string
	Template  string
}

type serviceAPIResolvedRoute struct {
	Operation string
	TenantID  string
}

func newServiceAPIAuthenticator(config ServiceAPIAuthConfig) (*serviceAPIAuthenticator, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if !config.Enabled() {
		return nil, nil
	}
	credentials := make(map[string]ServiceAPICredentialConfig, len(config.Credentials))
	required := map[string]struct{}{}
	for _, raw := range config.Credentials {
		value := raw
		value.KeyID = strings.TrimSpace(value.KeyID)
		value.Subject = strings.TrimSpace(value.Subject)
		value.TenantID = strings.TrimSpace(value.TenantID)
		value.Secret = append([]byte(nil), value.Secret...)
		value.Operations = canonicalStringSet(value.Operations)
		value.NotBefore = cloneRuntimeTime(value.NotBefore)
		value.ExpiresAt = cloneRuntimeTime(value.ExpiresAt)
		credentials[value.KeyID] = value
		for _, operation := range value.Operations {
			required[operation] = struct{}{}
		}
	}
	routes := make([]serviceAPIRoute, 0)
	for _, action := range accessauthorization.Catalog() {
		if _, ok := required[action.Code]; !ok {
			continue
		}
		for _, binding := range action.HTTP {
			routes = append(routes, serviceAPIRoute{Operation: action.Code, Method: strings.ToUpper(binding.Method), Template: binding.Path})
		}
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Method == routes[j].Method {
			return routes[i].Template < routes[j].Template
		}
		return routes[i].Method < routes[j].Method
	})
	return &serviceAPIAuthenticator{
		clockSkew: config.ClockSkew, credentials: credentials, required: required, routes: routes, now: time.Now,
	}, nil
}

func (auth *serviceAPIAuthenticator) bind(ctx context.Context, store *accesspersistence.Store) error {
	if auth == nil {
		return nil
	}
	if store == nil {
		return errServiceAPIUnavailable
	}
	values := make([]accesspersistence.ServiceAPICredentialBootstrap, 0, len(auth.credentials))
	for _, credential := range auth.credentials {
		values = append(values, accesspersistence.ServiceAPICredentialBootstrap{
			KeyID: credential.KeyID, Subject: credential.Subject, TenantID: credential.TenantID,
			SecretDigest: accesspersistence.ServiceAPISecretDigest(credential.Secret), Operations: credential.Operations,
			NotBefore: cloneRuntimeTime(credential.NotBefore), ExpiresAt: cloneRuntimeTime(credential.ExpiresAt),
		})
	}
	sort.Slice(values, func(i, j int) bool { return values[i].KeyID < values[j].KeyID })
	if err := store.BootstrapServiceAPICredentials(ctx, values); err != nil {
		return err
	}
	auth.mu.Lock()
	auth.store = store
	auth.mu.Unlock()
	return nil
}

func serviceAPISignedHeadersPresent(request *http.Request) bool {
	if request == nil {
		return false
	}
	return strings.TrimSpace(request.Header.Get(serviceAPIKeyIDHeader)) != "" ||
		strings.TrimSpace(request.Header.Get(serviceAPITimestampHeader)) != "" ||
		strings.TrimSpace(request.Header.Get(serviceAPINonceHeader)) != "" ||
		strings.TrimSpace(request.Header.Get(serviceAPISignatureHeader)) != ""
}

func (auth *serviceAPIAuthenticator) requiresSignature(request *http.Request) bool {
	if auth == nil || request == nil {
		return false
	}
	resolved, err := auth.resolveRoute(request.Method, request.URL.Path)
	if err != nil {
		return false
	}
	_, ok := auth.required[resolved.Operation]
	return ok
}

func (auth *serviceAPIAuthenticator) authenticate(request *http.Request) (identity.Principal, error) {
	if auth == nil || request == nil {
		return identity.Principal{}, errServiceAPIUnavailable
	}
	if strings.TrimSpace(request.Header.Get("Authorization")) != "" {
		return identity.Principal{}, errServiceAPIInvalidRequest
	}
	keyID, keyOK := singleServiceAPIHeader(request, serviceAPIKeyIDHeader)
	timestampRaw, timestampOK := singleServiceAPIHeader(request, serviceAPITimestampHeader)
	nonce, nonceOK := singleServiceAPIHeader(request, serviceAPINonceHeader)
	signature, signatureOK := singleServiceAPIHeader(request, serviceAPISignatureHeader)
	if !keyOK || !timestampOK || !nonceOK || !signatureOK ||
		!validServiceAPIIdentifier(keyID, 1, 128) || !validServiceAPIIdentifier(nonce, 16, 128) || !validLowerHex(signature, 64) {
		return identity.Principal{}, errServiceAPIInvalidRequest
	}
	credential, ok := auth.credentials[keyID]
	if !ok {
		return identity.Principal{}, accesspersistence.ErrUnauthorized
	}
	resolved, err := auth.resolveRoute(request.Method, request.URL.Path)
	if err != nil {
		return identity.Principal{}, errServiceAPIInvalidRequest
	}
	if _, required := auth.required[resolved.Operation]; !required {
		return identity.Principal{}, accesspersistence.ErrUnauthorized
	}
	timestampUnix, err := strconv.ParseInt(timestampRaw, 10, 64)
	if err != nil || strconv.FormatInt(timestampUnix, 10) != timestampRaw {
		return identity.Principal{}, errServiceAPIInvalidRequest
	}
	requestTime := time.Unix(timestampUnix, 0).UTC()
	now := auth.now().UTC()
	if requestTime.Before(now.Add(-auth.clockSkew)) || requestTime.After(now.Add(auth.clockSkew)) {
		return identity.Principal{}, errServiceAPIStaleRequest
	}
	canonicalPath, err := canonicalServiceAPIPath(request.URL)
	if err != nil {
		return identity.Principal{}, errServiceAPIInvalidRequest
	}
	canonicalQuery, err := canonicalServiceAPIQuery(request.URL.RawQuery)
	if err != nil {
		return identity.Principal{}, errServiceAPIInvalidRequest
	}
	bodyDigest, err := digestAndRestoreServiceAPIBody(request)
	if err != nil {
		return identity.Principal{}, errServiceAPIInvalidRequest
	}
	canonical := serviceAPICanonicalString(request.Method, canonicalPath, canonicalQuery, timestampRaw, nonce, bodyDigest)
	expected := serviceAPISignature(credential.Secret, canonical)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return identity.Principal{}, accesspersistence.ErrUnauthorized
	}

	auth.mu.RLock()
	store := auth.store
	auth.mu.RUnlock()
	if store == nil {
		return identity.Principal{}, errServiceAPIUnavailable
	}
	fact, err := store.ResolveServiceAPICredential(
		request.Context(), keyID, accesspersistence.ServiceAPISecretDigest(credential.Secret), resolved.Operation, resolved.TenantID, now,
	)
	if err != nil {
		if errors.Is(err, accesspersistence.ErrServiceAPICredentialUnavailable) {
			return identity.Principal{}, errServiceAPIUnavailable
		}
		return identity.Principal{}, accesspersistence.ErrUnauthorized
	}
	expiresAt := requestTime.Add(auth.clockSkew)
	if err := store.ConsumeServiceAPINonce(request.Context(), keyID, nonce, expiresAt); err != nil {
		if errors.Is(err, accesspersistence.ErrServiceAPIReplay) {
			return identity.Principal{}, errServiceAPIReplay
		}
		if errors.Is(err, accesspersistence.ErrServiceAPICredentialUnavailable) {
			return identity.Principal{}, errServiceAPIUnavailable
		}
		return identity.Principal{}, accesspersistence.ErrUnauthorized
	}
	return identity.Principal{Subject: fact.Subject, AuthMethod: identity.AuthMethodAPIKey, Authenticated: true}, nil
}

func (auth *serviceAPIAuthenticator) resolveRoute(method, requestPath string) (serviceAPIResolvedRoute, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	var found *serviceAPIResolvedRoute
	for _, route := range auth.routes {
		if route.Method != method {
			continue
		}
		params, ok := matchServiceAPIRoute(route.Template, requestPath)
		if !ok {
			continue
		}
		candidate := serviceAPIResolvedRoute{Operation: route.Operation, TenantID: params["tenant_id"]}
		if found != nil && found.Operation != candidate.Operation {
			return serviceAPIResolvedRoute{}, errors.New("service api: ambiguous operation route")
		}
		copy := candidate
		found = &copy
	}
	if found == nil {
		return serviceAPIResolvedRoute{}, errors.New("service api: operation route not found")
	}
	return *found, nil
}

func serviceAPICanonicalString(method, canonicalPath, canonicalQuery, timestamp, nonce, bodyDigest string) string {
	return strings.Join([]string{
		serviceAPISignaturePrefix,
		strings.ToUpper(strings.TrimSpace(method)),
		canonicalPath,
		canonicalQuery,
		timestamp,
		nonce,
		bodyDigest,
	}, "\n")
}

func serviceAPISignature(secret []byte, canonical string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

func canonicalServiceAPIPath(value *url.URL) (string, error) {
	if value == nil {
		return "", errors.New("missing url")
	}
	escaped := value.EscapedPath()
	if escaped == "" {
		escaped = "/"
	}
	if !strings.HasPrefix(escaped, "/") || strings.Contains(strings.ToLower(escaped), "%2f") || strings.Contains(strings.ToLower(escaped), "%5c") {
		return "", errors.New("ambiguous path")
	}
	segments := strings.Split(escaped, "/")
	canonical := make([]string, len(segments))
	for i, segment := range segments {
		if i == 0 {
			if segment != "" {
				return "", errors.New("path must be absolute")
			}
			continue
		}
		if segment == "" {
			if i == len(segments)-1 && len(segments) == 2 {
				continue
			}
			return "", errors.New("empty path segment")
		}
		decoded, err := url.PathUnescape(segment)
		if err != nil || decoded == "." || decoded == ".." || strings.ContainsAny(decoded, "/\\") {
			return "", errors.New("invalid path segment")
		}
		canonical[i] = url.PathEscape(decoded)
	}
	result := strings.Join(canonical, "/")
	if result == "" {
		return "/", nil
	}
	return result, nil
}

func canonicalServiceAPIQuery(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "", err
	}
	for key := range values {
		sort.Strings(values[key])
	}
	return values.Encode(), nil
}

func digestAndRestoreServiceAPIBody(request *http.Request) (string, error) {
	if request == nil || request.Body == nil {
		sum := sha256.Sum256(nil)
		return hex.EncodeToString(sum[:]), nil
	}
	limited := io.LimitReader(request.Body, serviceAPIMaxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	if int64(len(body)) > serviceAPIMaxBodyBytes {
		return "", errors.New("signed body exceeds limit")
	}
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func matchServiceAPIRoute(template, requestPath string) (map[string]string, bool) {
	if template == "" || requestPath == "" || strings.Contains(requestPath, "//") {
		return nil, false
	}
	templateParts := strings.Split(strings.Trim(template, "/"), "/")
	pathParts := strings.Split(strings.Trim(requestPath, "/"), "/")
	if len(templateParts) != len(pathParts) {
		return nil, false
	}
	params := map[string]string{}
	for i, expected := range templateParts {
		actual := pathParts[i]
		if actual == "" || actual == "." || actual == ".." {
			return nil, false
		}
		if strings.HasPrefix(expected, "{") && strings.HasSuffix(expected, "}") {
			name := strings.TrimSpace(expected[1 : len(expected)-1])
			if name == "" {
				return nil, false
			}
			params[name] = actual
			continue
		}
		if expected != actual {
			return nil, false
		}
	}
	return params, true
}

func singleServiceAPIHeader(request *http.Request, name string) (string, bool) {
	if request == nil {
		return "", false
	}
	values := request.Header.Values(name)
	if len(values) != 1 {
		return "", false
	}
	value := strings.TrimSpace(values[0])
	return value, value != ""
}

func validServiceAPIIdentifier(value string, min, max int) bool {
	if len(value) < min || len(value) > max || strings.TrimSpace(value) != value {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.' || char == ':' {
			continue
		}
		return false
	}
	return true
}

func validLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, char := range value {
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') {
			continue
		}
		return false
	}
	return true
}

func canonicalStringSet(values []string) []string {
	unique := map[string]struct{}{}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			unique[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func containsExact(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func actionHTTPHasTenantParameter(action accessauthorization.Action) bool {
	for _, binding := range action.HTTP {
		if strings.Contains(binding.Path, "{tenant_id}") {
			return true
		}
	}
	return false
}

func cloneRuntimeTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func writeServiceAPIAuthenticationFailure(writer http.ResponseWriter, err error) {
	if writer == nil {
		return
	}
	trace := make([]byte, 16)
	if _, randomErr := rand.Read(trace); randomErr == nil {
		writer.Header().Set(serviceAPITraceHeader, hex.EncodeToString(trace))
	}
	reason := "invalid_authentication"
	switch {
	case errors.Is(err, errServiceAPIMissingSignature):
		reason = "signature_required"
	case errors.Is(err, errServiceAPIStaleRequest):
		reason = "timestamp_invalid"
	case errors.Is(err, errServiceAPIReplay):
		reason = "replay_rejected"
	case errors.Is(err, errServiceAPIUnavailable):
		reason = "authentication_unavailable"
	case errors.Is(err, errServiceAPIInvalidRequest):
		reason = "signed_request_invalid"
	}
	writer.Header().Set(serviceAPIFailureHeader, reason)
	if errors.Is(err, errServiceAPIUnavailable) {
		http.Error(writer, "authentication unavailable", http.StatusServiceUnavailable)
		return
	}
	http.Error(writer, "Unauthorized", http.StatusUnauthorized)
}
