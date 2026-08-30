package bizruntime

import (
	"errors"
	"fmt"

	"github.com/hvritual/biz/modules/deviceops"
)

// ConfigProvider keeps Biz-owned process configuration explicit while satisfying
// the generated deviceops module descriptor. Assembly generation does not own
// environment loading or business configuration values.
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
