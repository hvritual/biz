package bizruntime

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hvritual/biz/modules/deviceops"
	"github.com/hvritual/yunka.io/gateway/authz"
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

type Options struct {
	DeviceOps         deviceops.Config
	PlatformBootstrap PlatformBootstrap
}

func (options Options) Validate() error {
	if err := options.DeviceOps.Validate(); err != nil {
		return err
	}
	return options.PlatformBootstrap.Validate()
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
