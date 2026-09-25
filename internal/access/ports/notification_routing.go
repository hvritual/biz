package ports

import "context"

type NotificationRouteRecipient struct {
	UserID string
	Active bool
}

type NotificationRouteRecipients interface {
	ResolveNotificationRouteRecipients(context.Context, string, []string) ([]NotificationRouteRecipient, error)
}
