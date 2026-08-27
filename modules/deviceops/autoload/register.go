package autoload

import (
	module "github.com/hvritual/biz/modules/deviceops"
	"yunka.io/framework/core/modulecatalog"
)

func init() { modulecatalog.MustRegister(module.GeneratedDescriptor()) }
