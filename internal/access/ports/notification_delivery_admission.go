package ports

import (
	"context"

	"github.com/hvritual/biz/internal/access/domain"
)

// OptionalNotificationDeliveryAdmitter re-checks the current member,
// preference and tenant-scoped contact immediately before an external provider
// call. Plain destinations are transient process values and must not be logged,
// audited or persisted by Notification.
type OptionalNotificationDeliveryAdmitter interface {
	PrepareOptionalNotificationDelivery(context.Context, domain.NotificationPreferenceOwner, domain.NotificationPreferenceChannel) (domain.OptionalNotificationDeliveryAdmission, error)
}
