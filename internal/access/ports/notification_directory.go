package ports

import "context"

// NotificationRecipient is a minimal Access-owned identity projection. Contact
// destinations, credentials and profile internals are deliberately not exposed.
type NotificationRecipient struct {
	UserID string
	Name   string
}
type NotificationRecipients interface {
	List(context.Context, string, string, int, int) ([]NotificationRecipient, uint64, error)
	Validate(context.Context, string, []string, bool) error
}
