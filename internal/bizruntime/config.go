package bizruntime

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	commercialports "github.com/hvritual/biz/internal/commercial/ports"
	"github.com/hvritual/biz/modules/deviceops"
	"yunka.io/gateway/authz"
)

// PlatformBootstrap is Biz-owned process bootstrap data for a tenantless
// control-plane principal. It is intentionally outside the Yunka module DSL.
type PlatformBootstrap struct {
	Subject     string
	Token       string
	Permissions []authz.PermissionKey
}

func (bootstrap PlatformBootstrap) Enabled() bool { return strings.TrimSpace(bootstrap.Token) != "" }

func (bootstrap PlatformBootstrap) Validate() error {
	if !bootstrap.Enabled() {
		return nil
	}
	if strings.TrimSpace(bootstrap.Subject) == "" {
		return errors.New("biz runtime: platform bootstrap subject is required")
	}
	if len(bootstrap.Permissions) == 0 {
		return errors.New("biz runtime: platform bootstrap permissions are required")
	}
	return nil
}

// WebAuthConfig enables the CE-12 browser trust boundary. The browser receives
// only HttpOnly session cookies; OIDC provider tokens never become browser
// authority for Biz APIs.
type WebAuthConfig struct {
	IssuerURL             string
	ClientID              string
	ClientSecret          string
	RedirectURL           string
	PostLogoutRedirectURL string
	Scopes                []string
	SessionTTL            time.Duration
	FlowTTL               time.Duration
	CookieSecure          bool

	// Optional explicit mapping for a tenantless platform operator. The OIDC
	// external subject is bound to an existing platform IAM subject; it never
	// turns a tenant user into a platform principal.
	PlatformExternalSubject string
	PlatformSubject         string
	PlatformEmail           string
}

func (config WebAuthConfig) Enabled() bool { return strings.TrimSpace(config.IssuerURL) != "" }

func (config WebAuthConfig) Validate() error {
	if !config.Enabled() {
		return nil
	}
	if strings.TrimSpace(config.ClientID) == "" {
		return errors.New("biz runtime: OIDC client id is required")
	}
	if strings.TrimSpace(config.RedirectURL) == "" {
		return errors.New("biz runtime: OIDC redirect URL is required")
	}
	if err := requireHTTPSOrLoopback(config.IssuerURL); err != nil {
		return err
	}
	if err := requireHTTPSOrLoopback(config.RedirectURL); err != nil {
		return err
	}
	if config.PostLogoutRedirectURL != "" {
		if err := requireHTTPSOrLoopback(config.PostLogoutRedirectURL); err != nil {
			return err
		}
	}
	if config.SessionTTL <= 0 || config.FlowTTL <= 0 {
		return errors.New("biz runtime: OIDC session and flow TTL must be positive")
	}
	if !containsString(config.Scopes, "openid") {
		return errors.New("biz runtime: OIDC scopes must include openid")
	}
	redirect, _ := url.Parse(config.RedirectURL)
	if redirect.Scheme == "https" && !config.CookieSecure {
		return errors.New("biz runtime: secure OIDC redirect requires Secure cookies")
	}
	if strings.TrimSpace(config.PlatformExternalSubject) != "" || strings.TrimSpace(config.PlatformSubject) != "" {
		if strings.TrimSpace(config.PlatformExternalSubject) == "" || strings.TrimSpace(config.PlatformSubject) == "" {
			return errors.New("biz runtime: OIDC platform external subject and platform subject must be configured together")
		}
	}
	return nil
}

// FirstPartyIdPConfig turns the existing Biz user account records into an OIDC
// identity provider. It is intentionally a single-client provider for the Biz
// BFF: member lifecycle owns users; this provider only owns password credentials
// and short-lived OIDC protocol state.
type FirstPartyIdPVerificationKey struct {
	PEM   string
	KeyID string
}

type FirstPartyIdPConfig struct {
	PublicURL             string
	ClientID              string
	RedirectURL           string
	PostLogoutRedirectURL string
	SigningKeyPEM         string
	SigningKeyID          string
	PreviousSigningKeys   []FirstPartyIdPVerificationKey
	LoginTTL              time.Duration
	CodeTTL               time.Duration
	TokenTTL              time.Duration
	CookieSecure          bool
}

func (config FirstPartyIdPConfig) Enabled() bool { return strings.TrimSpace(config.PublicURL) != "" }

func (config FirstPartyIdPConfig) IssuerURL() string {
	if !config.Enabled() {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(config.PublicURL), "/") + "/idp"
}

func (config FirstPartyIdPConfig) Validate() error {
	if !config.Enabled() {
		return nil
	}
	if err := requireHTTPSOrLoopback(config.PublicURL); err != nil {
		return err
	}
	if strings.TrimSpace(config.ClientID) == "" || strings.TrimSpace(config.RedirectURL) == "" {
		return errors.New("biz runtime: first-party IdP client id and redirect URL are required")
	}
	if err := requireHTTPSOrLoopback(config.RedirectURL); err != nil {
		return err
	}
	if strings.TrimSpace(config.SigningKeyPEM) == "" {
		return errors.New("biz runtime: first-party IdP RSA signing key is required")
	}
	if config.PostLogoutRedirectURL != "" {
		if err := requireHTTPSOrLoopback(config.PostLogoutRedirectURL); err != nil {
			return err
		}
		parsed, _ := url.Parse(config.PostLogoutRedirectURL)
		if parsed.Fragment != "" {
			return errors.New("biz runtime: first-party IdP post logout redirect must not contain a fragment")
		}
	}
	if len(config.PreviousSigningKeys) > 4 {
		return errors.New("biz runtime: at most four previous IdP signing keys are supported")
	}
	for _, previous := range config.PreviousSigningKeys {
		if strings.TrimSpace(previous.PEM) == "" {
			return errors.New("biz runtime: previous IdP signing key PEM must not be empty")
		}
	}
	if config.LoginTTL <= 0 || config.CodeTTL <= 0 || config.TokenTTL <= 0 {
		return errors.New("biz runtime: first-party IdP TTLs must be positive")
	}
	publicURL, _ := url.Parse(config.PublicURL)
	if publicURL.Scheme == "https" && !config.CookieSecure {
		return errors.New("biz runtime: secure first-party IdP URL requires Secure cookies")
	}
	return nil
}

type Options struct {
	ProvisioningPolicy  commercialports.ProvisioningPolicy
	PreparationAdapters []commercialports.RegisteredPreparation
	ProvisioningWorker  ProvisioningWorkerOptions
	// CommercialLifecycle is trusted runtime configuration for explicit trial
	// plan versions and the optional post-expiry grace interval.
	CommercialLifecycle subscription.LifecyclePolicy
	// A local trusted adapter only; nil conservatively defers quota reductions.
	QuotaChangePolicy commercialports.QuotaChangePolicy
	// Disable only the derived cache; authority reads and write barriers remain on.
	DisableEntitlementCache bool
	DeviceOps               deviceops.Config
	PlatformBootstrap       PlatformBootstrap
	WebAuth                 WebAuthConfig
	FirstPartyIdP           FirstPartyIdPConfig
}

func (options Options) Validate() error {
	if err := options.DeviceOps.Validate(); err != nil {
		return err
	}
	if err := options.ProvisioningWorker.Validate(); err != nil {
		return err
	}
	if err := options.CommercialLifecycle.Validate(); err != nil {
		return err
	}
	if err := options.PlatformBootstrap.Validate(); err != nil {
		return err
	}
	if err := options.WebAuth.Validate(); err != nil {
		return err
	}
	if err := options.FirstPartyIdP.Validate(); err != nil {
		return err
	}
	if options.FirstPartyIdP.Enabled() {
		if !options.WebAuth.Enabled() {
			return errors.New("biz runtime: first-party IdP requires the WebAuth BFF")
		}
		if strings.TrimRight(options.WebAuth.IssuerURL, "/") != options.FirstPartyIdP.IssuerURL() {
			return errors.New("biz runtime: first-party IdP issuer must match WebAuth issuer")
		}
		if options.WebAuth.ClientID != options.FirstPartyIdP.ClientID || options.WebAuth.RedirectURL != options.FirstPartyIdP.RedirectURL {
			return errors.New("biz runtime: first-party IdP client registration must match WebAuth")
		}
		if strings.TrimSpace(options.WebAuth.ClientSecret) != "" {
			return errors.New("biz runtime: first-party IdP uses public PKCE client registration; client secret must be empty")
		}
		if options.WebAuth.CookieSecure != options.FirstPartyIdP.CookieSecure {
			return errors.New("biz runtime: first-party IdP and WebAuth cookie security must match")
		}
	}
	return nil
}

// ConfigProvider keeps Biz-owned process configuration explicit while satisfying
// generated module descriptors. Access currently declares no config capability.
type ConfigProvider struct {
	DeviceOps deviceops.Config
}

func (provider ConfigProvider) Decode(moduleName, key string, target any) error {
	if moduleName != deviceops.ModuleName || key != "modules.deviceops" {
		return fmt.Errorf("unsupported module config %s/%s", moduleName, key)
	}
	config, ok := target.(*deviceops.Config)
	if !ok || config == nil {
		return errors.New("deviceops config target must be *deviceops.Config")
	}
	*config = provider.DeviceOps
	return nil
}
