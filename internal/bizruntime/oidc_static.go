package bizruntime

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// newOIDCClientFromProvider breaks the local IdP/BFF startup cycle without
// weakening runtime verification. Discovery remains publicly available and is
// exercised by E2E; the BFF still performs the real HTTP code exchange and JWKS
// signature verification after the server is running.
func newOIDCClientFromProvider(config WebAuthConfig, provider oidcProviderMetadata) (*oidcClient, error) {
	if provider.Issuer != strings.TrimRight(config.IssuerURL, "/") {
		return nil, fmt.Errorf("biz runtime: OIDC provider issuer mismatch: got %q", provider.Issuer)
	}
	if strings.TrimSpace(provider.AuthorizationEndpoint) == "" || strings.TrimSpace(provider.TokenEndpoint) == "" || strings.TrimSpace(provider.JWKSURI) == "" {
		return nil, errors.New("biz runtime: OIDC provider metadata is missing required endpoints")
	}
	if !containsString(provider.IDTokenSigningAlgsSupported, "RS256") {
		return nil, errors.New("biz runtime: OIDC provider must support RS256 ID tokens")
	}
	for _, endpoint := range []string{provider.AuthorizationEndpoint, provider.TokenEndpoint, provider.JWKSURI} {
		if err := requireHTTPSOrLoopback(endpoint); err != nil {
			return nil, err
		}
	}
	return &oidcClient{config: config, provider: provider, http: &http.Client{Timeout: 10 * time.Second}}, nil
}
