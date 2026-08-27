package deviceops

import (
	"gorm.io/gorm"
	"yunka.io/pkg/logExt"
)

// Dependencies is the complete compiler-checked capability view for this module.
// It contains no lookup, connection construction, or global runtime access.
type Dependencies struct {
	Config          Config
	Logger          logExt.Logger
	PrimaryDatabase *gorm.DB
}
