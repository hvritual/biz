package bizruntime

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

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

type Options struct {
	ProvisioningPolicy  commercialports.ProvisioningPolicy
	PreparationAdapters []commercialports.RegisteredPreparation
	ProvisioningWorker  ProvisioningWorkerOptions
	// A local trusted adapter only; nil conservatively defers quota reductions.
	QuotaChangePolicy commercialports.QuotaChangePolicy
	// Disable only the derived cache; authority reads and write barriers remain on.
	DisableEntitlementCache bool
	DeviceOps               deviceops.Config
	PlatformBootstrap       PlatformBootstrap
	WebAuth                 WebAuthConfig
}

func (options Options) Validate() error {
	if err := options.DeviceOps.Validate(); err != nil {
		return err
	}
	if err := options.ProvisioningWorker.Validate(); err != nil {
		return err
	}
	if err := options.PlatformBootstrap.Validate(); err != nil {
		return err
	}
	return options.WebAuth.Validate()
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
