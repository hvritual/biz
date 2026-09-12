package usecase

import (
	"context"
	"errors"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	pv "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	transition "github.com/hvritual/biz/internal/commercial/domain/timetransition"
	"github.com/hvritual/biz/internal/commercial/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"yunka.io/framework/requestscope"
)

func timeTransitionDTO(t transition.Task) *v1.CommercialTimeTransitionDTO {
	return &v1.CommercialTimeTransitionDTO{
		TransitionId: t.ID, Kind: t.Kind, TenantId: t.TenantID,
		AuthorityId: t.AuthorityID, AuthorityVersion: t.AuthorityVersion,
		DueAt: t.DueAt.Format(time.RFC3339Nano), State: t.State,
		Revision: t.Revision, Outcome: t.Outcome, BusinessTimezone: t.BusinessTimezone,
	}
}

func (s *service) ClaimCommercialTimeTransition(ctx context.Context, r *v1.ClaimCommercialTimeTransitionRequest) (*v1.ClaimCommercialTimeTransitionResponse, error) {
	if _, err := actor(ctx); err != nil {
		return nil, expose(err)
	}
	if r == nil || !transition.Key(r.WorkerId) || r.LeaseSeconds < 5 || r.LeaseSeconds > 300 {
		return nil, status.Error(codes.InvalidArgument, transition.ErrInvalid.Error())
	}
	value, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionChangeRepositories]) (*transition.Task, error) {
		repo := sc.Repositories().Transitions
		if repo == nil {
			return nil, transition.ErrCorrupt
		}
		now, err := repo.Now(sc.Context())
		if err != nil {
			return nil, err
		}
		return repo.ClaimDue(sc.Context(), r.WorkerId, now, time.Duration(r.LeaseSeconds)*time.Second)
	})
	if err != nil {
		return nil, exposeTransition(err)
	}
	if value == nil {
		return &v1.ClaimCommercialTimeTransitionResponse{}, nil
	}
	return &v1.ClaimCommercialTimeTransitionResponse{Found: true, Transition: timeTransitionDTO(*value), LeaseToken: value.LeaseToken}, nil
}

func exposeTransition(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, transition.ErrInvalid):
		return status.Error(codes.InvalidArgument, transition.ErrInvalid.Error())
	case errors.Is(err, transition.ErrNotFound):
		return status.Error(codes.NotFound, transition.ErrNotFound.Error())
	case errors.Is(err, transition.ErrConflict), errors.Is(err, transition.ErrLease):
		return status.Error(codes.Aborted, err.Error())
	case errors.Is(err, transition.ErrCorrupt):
		return status.Error(codes.DataLoss, transition.ErrCorrupt.Error())
	default:
		return status.Error(codes.Unavailable, "TIME_TRANSITION_AUTHORITY_UNAVAILABLE")
	}
}

type transitionResult struct {
	Task                 transition.Task
	SubscriptionRevision uint64
	SourceVersion        uint64
	EntitlementVersion   uint64
	ProvisioningTaskID   string
}

func transitionResultDTO(v transitionResult) *v1.CompleteCommercialTimeTransitionResponse {
	return &v1.CompleteCommercialTimeTransitionResponse{
		TransitionId: v.Task.ID, State: v.Task.State, Outcome: v.Task.Outcome,
		SubscriptionRevision: v.SubscriptionRevision, SourceVersion: v.SourceVersion,
		EntitlementVersion: v.EntitlementVersion, ProvisioningTaskId: v.ProvisioningTaskID,
	}
}

func finishTransition(ctx context.Context, repo ports.TimeTransitionRepository, current transition.Task, worker string, token uint64, state, outcome string, now time.Time) (transition.Task, error) {
	after := current
	if err := after.Finish(worker, token, state, outcome, now); err != nil {
		return transition.Task{}, err
	}
	if err := repo.Save(ctx, current, after); err != nil {
		return transition.Task{}, err
	}
	return after, nil
}

func terminalTransitionError(err error) (string, bool) {
	switch {
	case errors.Is(err, change.ErrTarget):
		return "TARGET_NO_LONGER_ELIGIBLE", true
	case errors.Is(err, change.ErrQuota):
		return "QUOTA_REVALIDATION_REQUIRED", true
	case errors.Is(err, change.ErrConflict), errors.Is(err, change.ErrCorrupt):
		return "AUTHORITY_REVALIDATION_REQUIRED", true
	case status.Code(err) == codes.NotFound || status.Code(err) == codes.FailedPrecondition || status.Code(err) == codes.InvalidArgument || status.Code(err) == codes.Aborted || status.Code(err) == codes.DataLoss:
		return "DEPENDENCY_REVALIDATION_REQUIRED", true
	default:
		return "", false
	}
}

func (s *service) CompleteCommercialTimeTransition(ctx context.Context, r *v1.CompleteCommercialTimeTransitionRequest) (*v1.CompleteCommercialTimeTransitionResponse, error) {
	if _, err := actor(ctx); err != nil {
		return nil, expose(err)
	}
	if r == nil || !transition.Key(r.TransitionId) || !transition.Key(r.WorkerId) || r.LeaseToken == 0 {
		return nil, status.Error(codes.InvalidArgument, transition.ErrInvalid.Error())
	}
	result, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionChangeRepositories]) (transitionResult, error) {
		repos, call := sc.Repositories(), sc.Context()
		if repos.Transitions == nil {
			return transitionResult{}, transition.ErrCorrupt
		}
		hint, err := repos.Transitions.Get(call, r.TransitionId)
		if err != nil {
			return transitionResult{}, err
		}
		switch hint.Kind {
		case transition.EntitlementExpiry:
			return s.completeEntitlementExpiry(call, repos, *hint, r.WorkerId, r.LeaseToken)
		case transition.SubscriptionBoundary:
			return s.completeSubscriptionBoundary(call, repos, *hint, r.WorkerId, r.LeaseToken)
		case transition.ScheduledChange:
			return s.completeScheduledChange(call, repos, *hint, r.WorkerId, r.LeaseToken)
		default:
			return transitionResult{}, transition.ErrCorrupt
		}
	})
	if err != nil {
		return nil, exposeTransition(err)
	}
	return transitionResultDTO(result), nil
}

func (s *service) completeEntitlementExpiry(ctx context.Context, repos ports.SubscriptionChangeRepositories, hint transition.Task, worker string, token uint64) (transitionResult, error) {
	state, err := repos.Entitlements.Lock(ctx, hint.TenantID)
	if err != nil {
		return transitionResult{}, err
	}
	view, err := s.snapshots.ReadSnapshot(ctx, hint.TenantID, nil)
	if err != nil {
		return transitionResult{}, err
	}
	current, err := repos.Transitions.Lock(ctx, hint.ID)
	if err != nil {
		return transitionResult{}, err
	}
	now, err := repos.Transitions.Now(ctx)
	if err != nil {
		return transitionResult{}, err
	}
	if !current.Owns(worker, token, now) {
		return transitionResult{}, transition.ErrLease
	}
	valid := false
	for _, source := range state.Sources {
		if source.ID == current.AuthorityID && source.Version == current.AuthorityVersion && source.ExpiresAt != nil && source.ExpiresAt.Equal(current.DueAt) && (source.RevokedAt == nil || !source.RevokedAt.Before(current.DueAt)) {
			valid = true
			break
		}
	}
	outcome := "ENTITLEMENT_EXPIRY_APPLIED"
	terminal := transition.Applied
	if !valid {
		outcome = "ENTITLEMENT_AUTHORITY_SUPERSEDED"
		terminal = transition.Superseded
	}
	finished, err := finishTransition(ctx, repos.Transitions, *current, worker, token, terminal, outcome, now)
	return transitionResult{Task: finished, SourceVersion: state.Version, EntitlementVersion: view.EntitlementVersion}, err
}

func (s *service) completeSubscriptionBoundary(ctx context.Context, repos ports.SubscriptionChangeRepositories, hint transition.Task, worker string, token uint64) (transitionResult, error) {
	raw, err := repos.Changes.LockTenant(ctx, hint.TenantID)
	if err != nil {
		return transitionResult{}, err
	}
	view, err := s.snapshots.ReadSnapshot(ctx, hint.TenantID, nil)
	if err != nil {
		return transitionResult{}, err
	}
	current, err := repos.Transitions.Lock(ctx, hint.ID)
	if err != nil {
		return transitionResult{}, err
	}
	now, err := repos.Transitions.Now(ctx)
	if err != nil {
		return transitionResult{}, err
	}
	if !current.Owns(worker, token, now) {
		return transitionResult{}, transition.ErrLease
	}
	if raw.ID != current.AuthorityID || raw.Revision != current.AuthorityVersion || raw.PeriodEnd == nil || !raw.PeriodEnd.Equal(current.DueAt) {
		finished, err := finishTransition(ctx, repos.Transitions, *current, worker, token, transition.Superseded, "SUBSCRIPTION_REVISION_SUPERSEDED", now)
		return transitionResult{Task: finished, SubscriptionRevision: raw.Revision, SourceVersion: view.SourceVersion, EntitlementVersion: view.EntitlementVersion}, err
	}
	if raw.PendingChangeID != "" {
		finished, err := finishTransition(ctx, repos.Transitions, *current, worker, token, transition.Applied, "RIGHTS_EXPIRED_PENDING_APPROVED_CHANGE", now)
		return transitionResult{Task: finished, SubscriptionRevision: raw.Revision, SourceVersion: view.SourceVersion, EntitlementVersion: view.EntitlementVersion}, err
	}
	after := raw
	after.Revision++
	outcome := "PAYMENT_OR_RENEWAL_CONFIRMATION_REQUIRED"
	after.State = subscription.StateRestricted
	if raw.RenewalStopped {
		after.State = subscription.StateEnded
		outcome = "RENEWAL_STOPPED_ENDED"
	} else if raw.State != subscription.StateGrace && s.lifecycle.GraceDuration > 0 {
		// Grace is anchored at the authoritative boundary, never worker pickup.
		// A delayed scheduler therefore cannot extend an expired entitlement.
		graceEnd := transition.CanonicalTime(current.DueAt.Add(s.lifecycle.GraceDuration))
		if graceEnd.After(now) {
			after.State = subscription.StateGrace
			after.PeriodEnd = &graceEnd
			outcome = "TRIAL_EXPIRED_GRACE"
			if raw.State != subscription.StateTrial {
				outcome = "PAYMENT_OR_RENEWAL_CONFIRMATION_GRACE"
			}
		} else if raw.State == subscription.StateTrial {
			outcome = "TRIAL_EXPIRED_GRACE_ELAPSED"
		} else {
			outcome = "PAYMENT_OR_RENEWAL_CONFIRMATION_GRACE_ELAPSED"
		}
	}
	after.EntitlementSourceVersion = view.SourceVersion
	if err := repos.Changes.SaveCurrent(ctx, raw, after); err != nil {
		return transitionResult{}, err
	}
	if after.State == subscription.StateGrace {
		next, err := transition.NewInTimezone(transition.SubscriptionBoundary, after.TenantID, after.ID, after.Revision, *after.PeriodEnd, now, current.BusinessTimezone)
		if err != nil {
			return transitionResult{}, err
		}
		if err = repos.Transitions.Insert(ctx, next); err != nil {
			return transitionResult{}, err
		}
	}
	finished, err := finishTransition(ctx, repos.Transitions, *current, worker, token, transition.Applied, outcome, now)
	return transitionResult{Task: finished, SubscriptionRevision: after.Revision, SourceVersion: view.SourceVersion, EntitlementVersion: view.EntitlementVersion}, err
}

func (s *service) completeScheduledChange(ctx context.Context, repos ports.SubscriptionChangeRepositories, hint transition.Task, worker string, token uint64) (transitionResult, error) {
	raw, err := repos.Changes.LockTenant(ctx, hint.TenantID)
	if err != nil {
		return transitionResult{}, err
	}
	preview, err := repos.Changes.Preview(ctx, hint.TenantID, hint.AuthorityID, true)
	if err != nil {
		return transitionResult{}, err
	}
	receipt, err := repos.Changes.Receipt(ctx, hint.TenantID, hint.AuthorityID, true)
	if err != nil {
		return transitionResult{}, err
	}
	if preview == nil || receipt == nil {
		return transitionResult{}, change.ErrCorrupt
	}
	if raw.PendingChangeID != hint.AuthorityID || raw.Revision != hint.AuthorityVersion || receipt.Status != change.Scheduled || receipt.Mode != change.Scheduled || !receipt.EffectiveAt.Equal(hint.DueAt) || receipt.After.Revision != raw.Revision {
		current, lockErr := repos.Transitions.Lock(ctx, hint.ID)
		if lockErr != nil {
			return transitionResult{}, lockErr
		}
		now, clockErr := repos.Transitions.Now(ctx)
		if clockErr != nil {
			return transitionResult{}, clockErr
		}
		if !current.Owns(worker, token, now) {
			return transitionResult{}, transition.ErrLease
		}
		finished, finishErr := finishTransition(ctx, repos.Transitions, *current, worker, token, transition.Superseded, "SCHEDULED_CHANGE_AUTHORITY_SUPERSEDED", now)
		return transitionResult{Task: finished, SubscriptionRevision: raw.Revision, SourceVersion: raw.EntitlementSourceVersion}, finishErr
	}

	material, materialErr := s.capture(ctx, repos, raw, preview.Input)
	if materialErr != nil {
		if reason, terminal := terminalTransitionError(materialErr); terminal {
			return s.failScheduledTransition(ctx, repos, hint, *receipt, raw, worker, token, reason)
		}
		return transitionResult{}, materialErr
	}
	requested := change.QuotaChanges(material.old.Terms, material.target.Terms)
	if preview.Input.Action == change.StopRenewal {
		requested = nil
	}
	report, err := s.quotas.Evaluate(ctx, ports.QuotaChangeInput{TenantID: hint.TenantID, ChangeID: hint.AuthorityID, Mode: change.Immediate, EffectiveAt: hint.DueAt, Changes: requested})
	if err != nil {
		return transitionResult{}, err
	}
	deferred, err := change.CheckQuotaReport(requested, report, change.Immediate)
	if err != nil || deferred {
		return s.failScheduledTransition(ctx, repos, hint, *receipt, raw, worker, token, "QUOTA_REVALIDATION_REQUIRED")
	}
	requirements, err := s.preparationRequirements(ctx, preview.Input.Action, material.target)
	if err != nil {
		return transitionResult{}, err
	}
	if change.Digest(requirements) != change.Digest(preview.ProvisioningRequirements) {
		return s.failScheduledTransition(ctx, repos, hint, *receipt, raw, worker, token, "PREPARATION_REQUIREMENTS_CHANGED")
	}
	current, err := repos.Transitions.Lock(ctx, hint.ID)
	if err != nil {
		return transitionResult{}, err
	}
	now, err := repos.Transitions.Now(ctx)
	if err != nil {
		return transitionResult{}, err
	}
	if !current.Owns(worker, token, now) || now.Before(current.DueAt) {
		return transitionResult{}, transition.ErrLease
	}

	if len(requirements) > 0 {
		after := raw
		after.Revision++
		after.State = subscription.StateRestricted
		task, err := pv.New(hint.TenantID, pv.Approval{ChangeID: hint.AuthorityID, ActorID: receipt.ActorID, PreviewHash: preview.Hash, TargetHash: material.target.ContentSHA256, TargetPlanCode: material.target.PlanCode, TargetPlanVersion: material.target.Number, SubscriptionRevision: after.Revision, SourceVersion: material.state.Version, EntitlementVersion: material.current.EntitlementVersion, CatalogRevision: material.current.CatalogRevision}, requirements, now)
		if err != nil {
			return transitionResult{}, err
		}
		if err := repos.Tasks.Insert(ctx, task); err != nil {
			return transitionResult{}, err
		}
		if err := repos.Changes.SaveCurrent(ctx, raw, after); err != nil {
			return transitionResult{}, err
		}
		next := *receipt
		next.Status = change.Provisioning
		next.ProvisioningTaskID = task.ID
		next.After = after
		next.AfterSourceVersion = material.state.Version
		next.AfterEntitlementVersion = material.current.EntitlementVersion
		next.Quotas = report
		next.QuotaValidationRequired = false
		next = next.Seal()
		if err := repos.Changes.UpdateReceipt(ctx, *receipt, next); err != nil {
			return transitionResult{}, err
		}
		finished, err := finishTransition(ctx, repos.Transitions, *current, worker, token, transition.Applied, "PROVISIONING_STARTED", now)
		return transitionResult{Task: finished, SubscriptionRevision: after.Revision, SourceVersion: material.state.Version, EntitlementVersion: material.current.EntitlementVersion, ProvisioningTaskID: task.ID}, err
	}

	after := raw
	after.Revision++
	after.PendingChangeID = ""
	after.State = s.lifecycle.StateFor(material.target.PlanCode, material.target.Number)
	sourceVersion, entitlementVersion, err := s.applySources(ctx, repos, material, &after, hint.AuthorityID, receipt.EffectiveAt, receipt.EntitlementExpiresAt)
	if err != nil {
		if reason, terminal := terminalTransitionError(err); terminal {
			return s.failScheduledTransition(ctx, repos, hint, *receipt, raw, worker, token, reason)
		}
		return transitionResult{}, err
	}
	if err := repos.Changes.SaveCurrent(ctx, raw, after); err != nil {
		return transitionResult{}, err
	}
	next := *receipt
	next.Status = change.Applied
	next.After = after
	next.AfterSourceVersion = sourceVersion
	next.AfterEntitlementVersion = entitlementVersion
	next.Quotas = report
	next.QuotaValidationRequired = false
	next = next.Seal()
	if err := repos.Changes.UpdateReceipt(ctx, *receipt, next); err != nil {
		return transitionResult{}, err
	}
	if err := repos.Events.Append(ctx, pv.Event{TenantID: hint.TenantID, AggregateID: after.ID, AggregateVersion: after.Revision, ChangeID: hint.AuthorityID, Status: pv.Applied, SourceVersion: sourceVersion, EntitlementVersion: entitlementVersion, OccurredAt: now}.Seal()); err != nil {
		return transitionResult{}, err
	}
	finished, err := finishTransition(ctx, repos.Transitions, *current, worker, token, transition.Applied, "SCHEDULED_CHANGE_APPLIED", now)
	return transitionResult{Task: finished, SubscriptionRevision: after.Revision, SourceVersion: sourceVersion, EntitlementVersion: entitlementVersion}, err
}

func (s *service) failScheduledTransition(ctx context.Context, repos ports.SubscriptionChangeRepositories, hint transition.Task, receipt change.Receipt, raw subscription.Subscription, worker string, token uint64, reason string) (transitionResult, error) {
	current, err := repos.Transitions.Lock(ctx, hint.ID)
	if err != nil {
		return transitionResult{}, err
	}
	now, err := repos.Transitions.Now(ctx)
	if err != nil {
		return transitionResult{}, err
	}
	if !current.Owns(worker, token, now) {
		return transitionResult{}, transition.ErrLease
	}
	next := receipt
	next.Status = change.Failed
	next.After = raw
	next.AfterSourceVersion = raw.EntitlementSourceVersion
	view, err := s.snapshots.ReadSnapshot(ctx, raw.TenantID, nil)
	if err != nil {
		return transitionResult{}, err
	}
	next.AfterEntitlementVersion = view.EntitlementVersion
	next.QuotaValidationRequired = false
	next = next.Seal()
	if err := repos.Changes.UpdateReceipt(ctx, receipt, next); err != nil {
		return transitionResult{}, err
	}
	finished, err := finishTransition(ctx, repos.Transitions, *current, worker, token, transition.ReconcileRequired, reason, now)
	return transitionResult{Task: finished, SubscriptionRevision: raw.Revision, SourceVersion: view.SourceVersion, EntitlementVersion: view.EntitlementVersion}, err
}
