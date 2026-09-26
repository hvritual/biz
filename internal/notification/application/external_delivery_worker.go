package application

import (
	"context"
	"errors"
	"strings"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
)

type ExternalDeliveryWorker struct {
	deps     ports.ExternalDeliveryDependencies
	policy   domain.ExternalDeliveryPolicy
	workerID string
}

func NewExternalDeliveryWorker(
	deps ports.ExternalDeliveryDependencies,
	policy domain.ExternalDeliveryPolicy,
	workerID string,
) (*ExternalDeliveryWorker, error) {
	if deps.Tasks == nil || deps.Admission == nil || deps.Provider == nil ||
		policy.Validate() != nil || !domain.ValidConfigurationID(workerID, 96) {
		return nil, domain.ErrExternalDeliveryInvalid
	}
	return &ExternalDeliveryWorker{deps: deps, policy: policy, workerID: workerID}, nil
}

func (worker *ExternalDeliveryWorker) RunOnce(ctx context.Context) (domain.ExternalTaskReceipt, error) {
	if worker == nil {
		return domain.ExternalTaskReceipt{}, domain.ErrExternalDeliveryInvalid
	}
	claim, receipt, err := worker.deps.Tasks.ClaimNextExternalTask(ctx, worker.workerID, worker.policy)
	if err != nil {
		return receipt, err
	}
	if claim.TaskID == "" {
		return receipt, nil
	}
	if claim.RecoveredUnknownOutcome {
		safe, ok := worker.deps.Provider.(ports.ExternalNotificationProviderRetrySafety)
		if !ok || !safe.ExternalNotificationIdempotent() {
			return worker.deps.Tasks.TerminalExternalTask(ctx, claim, domain.ExternalTaskStateManualReview, "DELIVERY_OUTCOME_UNKNOWN")
		}
	}

	preferenceChannel, ok := optionalPreferenceChannel(claim.Channel)
	if !ok {
		return worker.deps.Tasks.TerminalExternalTask(ctx, claim, domain.ExternalTaskStateManualReview, "CHANNEL_UNSUPPORTED")
	}
	owner := accessdomain.NotificationPreferenceOwner{TenantID: claim.TenantID, UserID: claim.UserID}
	admission, err := worker.deps.Admission.PrepareOptionalNotificationDelivery(ctx, owner, preferenceChannel)
	switch {
	case errors.Is(err, accessdomain.ErrNotificationPreferenceForbidden):
		return worker.deps.Tasks.TerminalExternalTask(ctx, claim, domain.ExternalTaskStateCancelled, "RECIPIENT_INACTIVE")
	case errors.Is(err, accessdomain.ErrNotificationDeliveryContactUnavailable):
		return worker.deps.Tasks.TerminalExternalTask(ctx, claim, domain.ExternalTaskStateManualReview, "CONTACT_UNAVAILABLE")
	case err != nil:
		// Infrastructure/key failures leave the lease intact. Another worker may
		// retry only after the lease expires; do not convert authority uncertainty
		// into an external send.
		return receipt, err
	case !admission.Allowed:
		return worker.deps.Tasks.TerminalExternalTask(ctx, claim, domain.ExternalTaskStateCancelled, "PREFERENCE_DENIED")
	case strings.TrimSpace(admission.Destination) == "":
		return worker.deps.Tasks.TerminalExternalTask(ctx, claim, domain.ExternalTaskStateManualReview, "CONTACT_UNAVAILABLE")
	}

	request := domain.ExternalProviderRequest{
		TaskID: claim.TaskID, EventID: claim.EventID, TenantID: claim.TenantID, UserID: claim.UserID,
		Channel: claim.Channel, Destination: admission.Destination, TypeCode: claim.TypeCode, Level: claim.Level,
		TraceID: claim.TraceID, ReferenceKind: claim.ReferenceKind, ReferenceID: claim.ReferenceID,
	}
	if err := request.Validate(); err != nil {
		return worker.deps.Tasks.TerminalExternalTask(ctx, claim, domain.ExternalTaskStateManualReview, "DELIVERY_REQUEST_INVALID")
	}

	sendCtx, cancel := context.WithTimeout(ctx, worker.policy.SendTimeout)
	result, sendErr := worker.deps.Provider.SendExternalNotification(sendCtx, request)
	cancel()
	if sendErr == nil {
		if err := result.Validate(); err != nil {
			return worker.deps.Tasks.TerminalExternalTask(ctx, claim, domain.ExternalTaskStateManualReview, "PROVIDER_INVALID_RECEIPT")
		}
		return worker.deps.Tasks.CompleteExternalTask(ctx, claim, result)
	}
	if ctx.Err() != nil {
		// Process shutdown leaves the fenced lease intact. Recovery is only
		// possible after expiry, preserving outcome uncertainty.
		return receipt, ctx.Err()
	}
	code, retryable := classifyExternalProviderFailure(worker.deps.Provider, sendErr)
	return worker.deps.Tasks.FailExternalTask(ctx, claim, code, retryable, worker.policy)
}

func classifyExternalProviderFailure(provider ports.ExternalNotificationProvider, err error) (string, bool) {
	code := "PROVIDER_FAILURE_UNCLASSIFIED"
	retryable, knownOutcome := true, false
	var failure ports.ExternalNotificationProviderFailure
	if errors.As(err, &failure) {
		code = failure.FailureCode()
		retryable = failure.Retryable()
		knownOutcome = failure.OutcomeKnown()
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		code = "DELIVERY_OUTCOME_UNKNOWN"
		retryable = true
		knownOutcome = false
	}
	if !knownOutcome {
		safe, ok := provider.(ports.ExternalNotificationProviderRetrySafety)
		retryable = retryable && ok && safe.ExternalNotificationIdempotent()
	}
	return code, retryable
}
