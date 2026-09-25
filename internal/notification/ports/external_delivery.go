package ports

import (
	"context"

	accessports "github.com/hvritual/biz/internal/access/ports"
	"github.com/hvritual/biz/internal/notification/domain"
)

type ExternalTaskRepository interface {
	ClaimNextExternalTask(context.Context, string, domain.ExternalDeliveryPolicy) (domain.ExternalTaskClaim, domain.ExternalTaskReceipt, error)
	CompleteExternalTask(context.Context, domain.ExternalTaskClaim, domain.ExternalProviderResult) (domain.ExternalTaskReceipt, error)
	FailExternalTask(context.Context, domain.ExternalTaskClaim, string, bool, domain.ExternalDeliveryPolicy) (domain.ExternalTaskReceipt, error)
	TerminalExternalTask(context.Context, domain.ExternalTaskClaim, string, string) (domain.ExternalTaskReceipt, error)
}

type ExternalNotificationProvider interface {
	SendExternalNotification(context.Context, domain.ExternalProviderRequest) (domain.ExternalProviderResult, error)
}

type ExternalNotificationProviderRetrySafety interface {
	ExternalNotificationIdempotent() bool
}

type ExternalNotificationProviderFailure interface {
	error
	FailureCode() string
	Retryable() bool
	OutcomeKnown() bool
}

type ExternalDeliveryDependencies struct {
	Tasks      ExternalTaskRepository
	Admission  accessports.OptionalNotificationDeliveryAdmitter
	Provider   ExternalNotificationProvider
}
