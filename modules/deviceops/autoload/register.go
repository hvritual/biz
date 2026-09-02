package autoload

import (
	module "github.com/hvritual/biz/modules/deviceops"
	"github.com/hvritual/yunka.io/framework/core/modulecatalog"
)

func init() { modulecatalog.MustRegister(module.GeneratedDescriptor()) }
