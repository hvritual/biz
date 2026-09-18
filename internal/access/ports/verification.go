package ports

import (
	"context"

	"github.com/hvritual/biz/internal/access/domain"
)

type VerificationRepository interface {
	CreateVerificationChallenge(context.Context, domain.VerificationChallengeRequest, domain.VerificationPolicy) (domain.VerificationChallengeReceipt, error)
	VerifyChallenge(context.Context, domain.VerifyChallengeRequest, domain.VerificationPolicy) (domain.OneTimeAuthorization, error)
	ConsumeOneTimeAuthorization(context.Context, domain.ConsumeAuthorizationRequest) (domain.AuthorizationConsumptionReceipt, error)
	EnqueueSecurityNotification(context.Context, domain.SecurityNotificationRequest) (domain.NotificationDeliveryReceipt, error)
	ClaimSecurityNotification(context.Context, string) (domain.SecurityNotificationClaim, domain.NotificationDeliveryReceipt, error)
	CompleteSecurityNotification(context.Context, domain.SecurityNotificationClaim, string, string, bool) (domain.NotificationDeliveryReceipt, error)
}

type SecurityNotificationSender interface {
	SendSecurityNotification(context.Context, domain.SecurityNotificationClaim) (providerReceipt string, err error)
}
