package ports

import (
	"context"
	"time"

	accessports "github.com/hvritual/biz/internal/access/ports"
	deviceports "github.com/hvritual/biz/internal/deviceops/ports"
	"github.com/hvritual/biz/internal/notification/domain"
)

type BusinessEventPublisher interface {
	AppendBusinessEvent(context.Context, domain.BusinessEvent) error
}

type RoutingQueue interface {
	ClaimNextBusinessEvent(context.Context, string, time.Duration) (domain.RoutingClaim, error)
	CompleteBusinessEventRoute(context.Context, domain.RoutingClaim, domain.RoutePlan) (domain.RoutingResult, error)
}

type RoutingConfigurationReader interface {
	FindRoutingConfiguration(context.Context, string, string, domain.MessageLevel) (domain.Configuration, bool, error)
}

type RoutingDependencies struct {
	Queue          RoutingQueue
	Configurations RoutingConfigurationReader
	Recipients     accessports.NotificationRouteRecipients
	Groups         deviceports.NotificationRouteGroups
	Preferences    accessports.OptionalNotificationPreferenceReader
}
