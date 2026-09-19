package application

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
)

type VerificationService struct {
	repository ports.VerificationRepository
	sender     ports.SecurityNotificationSender
	policy     domain.VerificationPolicy
}

func NewVerificationService(repository ports.VerificationRepository, sender ports.SecurityNotificationSender, policy domain.VerificationPolicy) (*VerificationService, error) {
	if repository == nil {
		return nil, errors.New("access application: verification repository is required")
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &VerificationService{repository: repository, sender: sender, policy: policy}, nil
}

func (service *VerificationService) SendVerificationCode(ctx context.Context, request domain.VerificationChallengeRequest) (domain.VerificationChallengeReceipt, domain.NotificationDeliveryReceipt, error) {
	challenge, err := service.repository.CreateVerificationChallenge(ctx, request, service.policy)
	if err != nil {
		return domain.VerificationChallengeReceipt{}, domain.NotificationDeliveryReceipt{}, err
	}
	delivery, err := service.DeliverSecurityNotification(ctx, challenge.NotificationEventID)
	return challenge, delivery, err
}

func (service *VerificationService) VerifyCode(ctx context.Context, request domain.VerifyChallengeRequest) (domain.OneTimeAuthorization, error) {
	return service.repository.VerifyChallenge(ctx, request, service.policy)
}

func (service *VerificationService) ConsumeAuthorization(ctx context.Context, request domain.ConsumeAuthorizationRequest) (domain.AuthorizationConsumptionReceipt, error) {
	return service.repository.ConsumeOneTimeAuthorization(ctx, request)
}

func (service *VerificationService) QueueSecurityNotification(ctx context.Context, request domain.SecurityNotificationRequest) (domain.NotificationDeliveryReceipt, error) {
	return service.repository.EnqueueSecurityNotification(ctx, request)
}

func (service *VerificationService) DeliverSecurityNotification(ctx context.Context, eventID string) (domain.NotificationDeliveryReceipt, error) {
	claim, receipt, err := service.repository.ClaimSecurityNotification(ctx, eventID)
	if err != nil {
		return receipt, err
	}
	if claim.EventID == "" {
		return receipt, nil
	}
	if service.sender == nil {
		failed, completeErr := service.repository.CompleteSecurityNotification(ctx, claim, "", "CHANNEL_UNAVAILABLE", false)
		if completeErr != nil {
			return failed, completeErr
		}
		return failed, domain.ErrNotificationUnavailable
	}
	providerReceipt, sendErr := service.sender.SendSecurityNotification(ctx, claim)
	if sendErr != nil {
		failed, completeErr := service.repository.CompleteSecurityNotification(ctx, claim, "", "DELIVERY_FAILED", false)
		if completeErr != nil {
			return failed, completeErr
		}
		return failed, sendErr
	}
	return service.repository.CompleteSecurityNotification(ctx, claim, providerReceipt, "", true)
}
