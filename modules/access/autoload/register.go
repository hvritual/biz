package autoload

import (
	module "github.com/hvritual/biz/modules/access"
	"yunka.io/framework/core/modulecatalog"
)

func init() { modulecatalog.MustRegister(module.GeneratedDescriptor()) }
