package ports

import "context"

// NotificationRouteGroups validates only current resource existence/tenant
// ownership for asynchronous routing. User authorization is resolved when a
// configuration is created/edited; routing must still reject retired resources.
type NotificationRouteGroups interface {
	NotificationRouteGroupActive(context.Context, string, string) (bool, error)
}
