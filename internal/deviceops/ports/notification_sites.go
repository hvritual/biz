package ports

import (
	"context"
	"errors"
)

var (
	ErrNotificationSiteDirectoryUnavailable = errors.New("deviceops: notification site directory unavailable")
	ErrNotificationSiteQueryInvalid         = errors.New("deviceops: invalid notification site directory query")
	ErrNotificationSiteScopeDenied          = errors.New("deviceops: notification site scope denied")
)

// NotificationSite is a minimal resource-owned projection. The display name is
// not a group identity; Version is the resource's version, not an IAM version.
type NotificationSite struct {
	ID      string
	Name    string
	Version uint64
}

// NotificationSites resolves the resource side of the approved notification
// group=site mapping. It never owns Access facts or configuration records.
type NotificationSites interface {
	CurrentIDs(context.Context) ([]string, error)
	List(context.Context, string, int, int) ([]NotificationSite, uint64, error)
	Resolve(context.Context, []string, bool) ([]NotificationSite, error)
}
