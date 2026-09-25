package application

import (
	"context"
	"errors"
	"strings"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accessports "github.com/hvritual/biz/internal/access/ports"
)

type SecurityDeliveryWorker struct {
	Repository accessports.ReliableSecurityNotificationRepository
	Sender     accessports.SecurityNotificationSender
	Policy     accessdomain.NotificationRetryPolicy
	WorkerID   string
}

func (worker SecurityDeliveryWorker) Validate() error {
	if worker.Repository == nil || worker.Sender == nil || strings.TrimSpace(worker.WorkerID) == "" ||
		len(worker.WorkerID) > 96 || worker.Policy.Validate() != nil {
		return accessdomain.ErrNotificationUnavailable
	}
	return nil
}

func (worker SecurityDeliveryWorker) RunOnce(ctx context.Context) (accessdomain.NotificationDeliveryAttemptResult, error) {
	var result accessdomain.NotificationDeliveryAttemptResult
	if err := worker.Validate(); err != nil {
		return result, err
	}
	claim, _, err := worker.Repository.ClaimNextSecurityNotification(ctx, worker.WorkerID, worker.Policy)
	if err != nil {
		return result, err
	}
	if claim.EventID == "" {
		return result, nil
	}
	result.EventID, result.Attempt = claim.EventID, claim.Attempt
	sendCtx, cancel := context.WithTimeout(ctx, worker.Policy.SendTimeout)
	providerReceipt, sendErr := worker.Sender.SendSecurityNotification(sendCtx, claim.SecurityNotificationClaim)
	cancel()
	if sendErr == nil {
		completed, err := worker.Repository.CompleteReliableSecurityNotification(ctx, claim, providerReceipt)
		if err != nil {
			return result, err
		}
		result.State, result.FailureCode = completed.State, completed.FailureCode
		return result, nil
	}
	if ctx.Err() != nil {
		// Shutdown/caller cancellation leaves the fenced SENDING claim intact.
		// A later worker may recover it only after lease expiry.
		return result, ctx.Err()
	}
	code, retryable := classifySecurityDeliveryFailure(worker.Sender, sendErr)
	failed, err := worker.Repository.FailReliableSecurityNotification(ctx, claim, code, retryable, worker.Policy)
	if err != nil {
		return result, err
	}
	result.State, result.FailureCode = failed.State, failed.FailureCode
	return result, nil
}

func classifySecurityDeliveryFailure(sender accessports.SecurityNotificationSender, err error) (string, bool) {
	code := "PROVIDER_FAILURE_UNCLASSIFIED"
	retryable, knownOutcome := true, false
	var failure accessports.SecurityNotificationDeliveryFailure
	if errors.As(err, &failure) {
		code = failure.FailureCode()
		retryable = failure.Retryable()
		knownOutcome = failure.OutcomeKnown()
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		code, knownOutcome, retryable = "DELIVERY_OUTCOME_UNKNOWN", false, true
	}
	if !knownOutcome {
		safe, ok := sender.(accessports.SecurityNotificationRetrySafety)
		retryable = retryable && ok && safe.SecurityNotificationIdempotent()
	}
	return code, retryable
}
