package access

import "gorm.io/gorm"

// Dependencies is the complete compiler-checked capability view for this module.
// It contains no lookup, connection construction, or global runtime access.
type Dependencies struct {
	PrimaryDatabase *gorm.DB
}
