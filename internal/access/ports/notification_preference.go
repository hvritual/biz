package ports

import (
	"context"

	"github.com/hvritual/biz/internal/access/domain"
)

// OptionalNotificationPreferenceReader is the only preference read seam for
// notification routing. The recipient must come from an authorized tenant
// recipient resolver. Errors are fail-closed; callers must not enqueue on error
// or explicit deny. This port does not approve any security-message exemption.
// #185 must re-read this port at task admission and before external delivery.
type OptionalNotificationPreferenceReader interface {
	ReadNotificationPreference(context.Context, domain.NotificationPreferenceOwner, domain.NotificationPreferenceChannel) (domain.NotificationPreference, error)
}

// SelfNotificationPreferences receives the authenticated owner from BFF only.
// A replay returns its original receipt, not a claim about the current value;
// clients must perform a fresh self read after a successful/uncertain mutation.
type SelfNotificationPreferences interface {
	ReadNotificationPreferences(context.Context, domain.NotificationPreferenceOwner) (domain.NotificationPreferenceSet, error)
	ChangeNotificationPreference(context.Context, domain.NotificationPreferenceOwner, domain.NotificationPreferenceChange) (domain.NotificationPreferenceReceipt, error)
}
