package usecase

import (
	"context"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	projection "github.com/hvritual/biz/internal/commercial/application/planprojection"
	pv "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/requestscope"
)

// This private typed operation cannot create commercial approval. It consumes
// an immutable approval only after each compiled adapter has durable READY proof.
func (s *service) CompletePreparedSubscriptionChange(ctx context.Context, r *v1.PreparedSubscriptionChangeRequest) (*v1.ProvisioningCompletionDTO, error) {
	if _, err := actor(ctx); err != nil {
		return nil, expose(err)
	}
	if r == nil || !pv.Tenant(r.TenantId) || !pv.Key(r.TaskId) || !pv.Key(r.WorkerId) || r.LeaseToken == 0 {
		return nil, expose(pv.ErrInvalid)
	}
	out, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionChangeRepositories]) (pv.Completion, error) {
		repos, call := sc.Repositories(), sc.Context()
		raw, e := repos.Changes.LockTenant(call, r.TenantId)
		if e != nil {
			return pv.Completion{}, e
		}
		task, e := repos.Tasks.LockTask(call, r.TenantId, r.TaskId)
		if e != nil {
			return pv.Completion{}, e
		}
		now, e := repos.Changes.Now(call)
		if e != nil {
			return pv.Completion{}, e
		}
		if !task.Owns(r.WorkerId, r.LeaseToken, now) || task.Stage != pv.ActivateStage || task.StepIndex != len(task.Steps) {
			return pv.Completion{}, pv.ErrLease
		}
		if raw.PendingChangeID != task.Approval.ChangeID || raw.Revision != task.Approval.SubscriptionRevision || !now.Before(task.Deadline) {
			return pv.Completion{}, pv.ErrStale
		}
		p, e := repos.Changes.Preview(call, r.TenantId, task.Approval.ChangeID, true)
		if e != nil {
			return pv.Completion{}, e
		}
		receipt, e := repos.Changes.Receipt(call, r.TenantId, task.Approval.ChangeID, true)
		if e != nil {
			return pv.Completion{}, e
		}
		if p == nil || receipt == nil || p.Hash != task.Approval.PreviewHash || p.ActorID != task.Approval.ActorID || receipt.ActorID != task.Approval.ActorID || receipt.Status != change.Provisioning || receipt.ProvisioningTaskID != task.ID || (receipt.Mode != change.Immediate && receipt.Mode != change.Scheduled) {
			return pv.Completion{}, pv.ErrCorrupt
		}
		if change.Digest(raw) != change.Digest(receipt.After) {
			return pv.Completion{}, pv.ErrStale
		}
		m, e := s.capture(call, repos, raw, p.Input)
		if e != nil {
			return pv.Completion{}, e
		}
		if m.state.Version != task.Approval.SourceVersion || m.current.EntitlementVersion != task.Approval.EntitlementVersion || m.current.CatalogRevision != task.Approval.CatalogRevision || m.target.ContentSHA256 != task.Approval.TargetHash {
			return pv.Completion{}, pv.ErrStale
		}
		requirements, e := s.preparationRequirements(call, p.Input.Action, m.target)
		if e != nil {
			return pv.Completion{}, e
		}
		if change.Digest(requirements) != change.Digest(p.ProvisioningRequirements) || len(requirements) != len(task.Steps) {
			return pv.Completion{}, pv.ErrStale
		}
		for i, step := range task.Steps {
			if step.State != pv.ReadyStep || step.Effect != pv.ReadyStep || step.Evidence == "" || step.Requirement != requirements[i] {
				return pv.Completion{}, pv.ErrCorrupt
			}
		}

		mode := receipt.Mode
		at := receipt.EffectiveAt
		end := receipt.EntitlementExpiresAt
		if mode == change.Immediate {
			_, at, end, e = change.Period(p.Input, m.before, p.Classification, now, m.target.Terms)
			if e != nil {
				return pv.Completion{}, e
			}
		} else if at.After(now) {
			return pv.Completion{}, pv.ErrStale
		}
		requested := change.QuotaChanges(m.old.Terms, m.target.Terms)
		if p.Input.Action == change.StopRenewal {
			requested = nil
		}
		// Provisioning completion always represents a present-time activation.
		// Scheduled reductions were allowed to defer usage evidence at approval;
		// they must pass the immediate policy now.
		report, e := s.quotas.Evaluate(call, ports.QuotaChangeInput{TenantID: r.TenantId, ChangeID: task.Approval.ChangeID, Mode: change.Immediate, EffectiveAt: at, Changes: requested})
		if e != nil {
			return pv.Completion{}, e
		}
		deferred, e := change.CheckQuotaReport(requested, report, change.Immediate)
		if e != nil {
			return pv.Completion{}, e
		}
		if deferred {
			return pv.Completion{}, change.ErrQuota
		}
		admitted, e := repos.Changes.Now(call)
		if e != nil {
			return pv.Completion{}, e
		}
		if !task.Owns(r.WorkerId, r.LeaseToken, admitted) || !admitted.Before(task.Deadline) {
			return pv.Completion{}, pv.ErrLease
		}
		if mode == change.Immediate {
			// Delay in a local policy cannot extend expired old rights.
			if m.current.ValidUntil != nil && !admitted.Before(*m.current.ValidUntil) {
				return pv.Completion{}, pv.ErrStale
			}
			_, at, end, e = change.Period(p.Input, m.before, p.Classification, admitted, m.target.Terms)
			if e != nil {
				return pv.Completion{}, e
			}
		}
		after := m.before
		after.Revision++
		after.PendingChangeID = ""
		after.State = s.lifecycle.StateFor(m.target.PlanCode, m.target.Number)
		source, ent, e := s.applySources(call, repos, m, &after, task.Approval.ChangeID, at, end)
		if e != nil {
			return pv.Completion{}, e
		}
		if e = repos.Changes.SaveCurrent(call, m.before, after); e != nil {
			return pv.Completion{}, e
		}
		nextReceipt := *receipt
		nextReceipt.Status = change.Applied
		nextReceipt.After = after
		nextReceipt.AfterSourceVersion = source
		nextReceipt.AfterEntitlementVersion = ent
		nextReceipt.Quotas = report
		nextReceipt.QuotaValidationRequired = false
		nextReceipt = nextReceipt.Seal()
		if e = repos.Changes.UpdateReceipt(call, *receipt, nextReceipt); e != nil {
			return pv.Completion{}, e
		}
		if e = repos.Events.Append(call, pv.Event{TenantID: r.TenantId, AggregateID: after.ID, AggregateVersion: after.Revision, ChangeID: task.Approval.ChangeID, TaskID: task.ID, Status: pv.Applied, SourceVersion: source, EntitlementVersion: ent, OccurredAt: admitted}.Seal()); e != nil {
			return pv.Completion{}, e
		}
		return pv.Completion{ChangeID: task.Approval.ChangeID, SubscriptionRevision: after.Revision, SourceVersion: source, EntitlementVersion: ent, AppliedAt: admitted}, nil
	})
	if err != nil {
		return nil, expose(err)
	}
	return completionDTO(out), nil
}
func completionDTO(v pv.Completion) *v1.ProvisioningCompletionDTO {
	return &v1.ProvisioningCompletionDTO{ChangeId: v.ChangeID, SubscriptionRevision: v.SubscriptionRevision, SourceVersion: v.SourceVersion, EntitlementVersion: v.EntitlementVersion, AppliedAt: projection.Instant(&v.AppliedAt)}
}

// Cancellation only releases untouched or positively absent preparation.
// It does not guess provider cleanup or erase successful external actions.
func (s *service) CancelPreparedSubscriptionChange(ctx context.Context, r *v1.PreparedSubscriptionChangeRequest) (*v1.ProvisioningCompletionDTO, error) {
	if _, err := actor(ctx); err != nil {
		return nil, expose(err)
	}
	if r == nil || !pv.Tenant(r.TenantId) || !pv.Key(r.TaskId) || r.TaskRevision == 0 {
		return nil, expose(pv.ErrInvalid)
	}
	out, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionChangeRepositories]) (pv.Completion, error) {
		repos, call := sc.Repositories(), sc.Context()
		raw, e := repos.Changes.LockTenant(call, r.TenantId)
		if e != nil {
			return pv.Completion{}, e
		}
		t, e := repos.Tasks.LockTask(call, r.TenantId, r.TaskId)
		if e != nil {
			return pv.Completion{}, e
		}
		if t.Revision != r.TaskRevision || !t.Cancellable() {
			return pv.Completion{}, pv.ErrCancellation
		}
		if raw.PendingChangeID != t.Approval.ChangeID || raw.Revision != t.Approval.SubscriptionRevision {
			return pv.Completion{}, pv.ErrStale
		}
		now, e := repos.Changes.Now(call)
		if e != nil {
			return pv.Completion{}, e
		}
		view, e := s.snapshots.ReadSnapshot(call, r.TenantId, nil)
		if e != nil {
			return pv.Completion{}, e
		}
		after := raw
		after.PendingChangeID = ""
		after.Revision++
		after.EntitlementSourceVersion = view.SourceVersion
		// Once a scheduled boundary has passed, cancellation must not revive the
		// expired old plan. The commercial subscription remains restricted until a
		// new trusted change is approved.
		if raw.State == subscription.StateRestricted {
			after.State = subscription.StateRestricted
		}
		if e = repos.Changes.SaveCurrent(call, raw, after); e != nil {
			return pv.Completion{}, e
		}
		if e = repos.Events.Append(call, pv.Event{TenantID: r.TenantId, AggregateID: after.ID, AggregateVersion: after.Revision, ChangeID: t.Approval.ChangeID, TaskID: t.ID, Status: pv.Cancelled, SourceVersion: view.SourceVersion, EntitlementVersion: view.EntitlementVersion, OccurredAt: now}.Seal()); e != nil {
			return pv.Completion{}, e
		}
		return pv.Completion{ChangeID: t.Approval.ChangeID, SubscriptionRevision: after.Revision, SourceVersion: view.SourceVersion, EntitlementVersion: view.EntitlementVersion, AppliedAt: now}, nil
	})
	if err != nil {
		return nil, expose(err)
	}
	return completionDTO(out), nil
}
