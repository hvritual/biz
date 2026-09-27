package ports

import (
	"context"
	accessports "github.com/hvritual/biz/internal/access/ports"
	deviceports "github.com/hvritual/biz/internal/deviceops/ports"
)

// CurrentAuthorization consumes current Access facts, not a second grant store.
type CurrentAuthorization interface {
	Require(context.Context, string, bool) error
}

// ConfigurationRepositories is assembled in the same borrowed root transaction.
// Application code never opens a database or crosses another domain's tables.
type ConfigurationRepositories struct {
	Configurations ConfigurationRepository
	Authority      CurrentAuthorization
	Recipients     accessports.NotificationRecipients
	ReadGroups     deviceports.NotificationSites
	CreateGroups   deviceports.NotificationSites
	UpdateGroups   deviceports.NotificationSites
	DeleteGroups   deviceports.NotificationSites
	Audit          accessports.AuditEventAppender
}
