package ports

import (
	"context"

	"github.com/hvritual/biz/internal/access/domain"
)

type ReliableSecurityNotificationRepository interface {
	EnsureReliableSecurityNotificationSchema(context.Context) error
	ClaimNextSecurityNotification(context.Context, string, domain.NotificationRetryPolicy) (domain.ReliableSecurityNotificationClaim, domain.NotificationDeliveryReceipt, error)
	CompleteReliableSecurityNotification(context.Context, domain.ReliableSecurityNotificationClaim, string) (domain.NotificationDeliveryReceipt, error)
	FailReliableSecurityNotification(context.Context, domain.ReliableSecurityNotificationClaim, string, bool, domain.NotificationRetryPolicy) (domain.NotificationDeliveryReceipt, error)
}

type SecurityNotificationRetrySafety interface {
	SecurityNotificationIdempotent() bool
}

type SecurityNotificationDeliveryFailure interface {
	error
	FailureCode() string
	Retryable() bool
	OutcomeKnown() bool
}
