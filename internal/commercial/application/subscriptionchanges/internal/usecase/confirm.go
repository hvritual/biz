package usecase

import (
	"context"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	pv "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"github.com/hvritual/biz/internal/commercial/ports"
	"strings"
	"yunka.io/framework/requestscope"
)

func (s *service) ConfirmSubscriptionChange(ctx context.Context, r *v1.ConfirmSubscriptionChangeRequest) (*v1.SubscriptionChangeReceiptDTO, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if r == nil || !change.Tenant(r.TenantId) || !change.Key(r.ChangeId) || !validKey(ctx, r.RequestId) || len(r.PreviewHash) != 64 || !change.Reason(r.Reason) {
		return nil, expose(change.ErrInvalid)
	}
	reason := strings.TrimSpace(r.Reason)
	fingerprint := change.Digest([]string{a, r.TenantId, r.RequestId, r.ChangeId, r.PreviewHash, reason})
	result, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionChangeRepositories]) (change.Receipt, error) {
		repos, call := sc.Repositories(), sc.Context()
		repo := repos.Changes
		raw, e := repo.LockTenant(call, r.TenantId)
		if e != nil {
			return change.Receipt{}, e
		}
		replay, e := repo.ReceiptForRequest(call, r.TenantId, a, r.RequestId, fingerprint)
		if e != nil {
			return change.Receipt{}, e
		}
		if replay != nil {
			return *replay, nil
		}
		already, e := repo.Receipt(call, r.TenantId, r.ChangeId, true)
		if e != nil {
			return change.Receipt{}, e
		}
		if already != nil {
			return change.Receipt{}, change.ErrRequestConflict
		}
		p, e := repo.Preview(call, r.TenantId, r.ChangeId, true)
		if e != nil {
			return change.Receipt{}, e
		}
		if p == nil {
			return change.Receipt{}, change.ErrNotFound
		}
		if p.ActorID != a {
			return change.Receipt{}, change.ErrScope
		}
		if p.Hash != r.PreviewHash {
			return change.Receipt{}, change.ErrConflict
		}
		now, e := repo.Now(call)
		if e != nil {
			return change.Receipt{}, e
		}
		if !now.Before(p.ExpiresAt) {
			return change.Receipt{}, change.ErrExpired
		}
		if raw.PendingChangeID != "" {
			return change.Receipt{}, change.ErrPending
		}
		m, e := s.capture(call, repos, raw, p.Input)
		if e != nil {
			return change.Receipt{}, e
		}
		if change.Digest(m.before) != change.Digest(p.Before) || m.current.SourceVersion != p.Current.SourceVersion || m.current.EntitlementVersion != p.Current.EntitlementVersion || m.current.CatalogRevision != p.Current.CatalogRevision || m.target.ContentSHA256 != p.Target.ContentSHA256 {
			return change.Receipt{}, change.ErrConflict
		}
		mode, at, end, e := change.Period(p.Input, m.before, p.Classification, now, m.target.Terms)
		if e != nil {
			return change.Receipt{}, e
		}
		if mode != p.Mode || (mode == change.Scheduled && !at.Equal(p.EffectiveAt)) {
			return change.Receipt{}, change.ErrConflict
		}
		if mode == change.Scheduled && !at.After(now) {
			return change.Receipt{}, change.ErrExpired
		}
		requested := change.QuotaChanges(m.old.Terms, m.target.Terms)
		if p.Input.Action == change.StopRenewal {
			requested = nil
		}
		quotas, e := s.quotas.Evaluate(call, ports.QuotaChangeInput{TenantID: r.TenantId, ChangeID: r.ChangeId, Mode: mode, EffectiveAt: at, Changes: requested})
		if e != nil {
			return change.Receipt{}, e
		}
		deferred, e := change.CheckQuotaReport(requested, quotas, mode)
		if e != nil {
			return change.Receipt{}, e
		}
		requirements, e := s.preparationRequirements(call, p.Input.Action, m.target)
		if e != nil {
			return change.Receipt{}, e
		}
		if change.Digest(requirements) != change.Digest(p.ProvisioningRequirements) {
			return change.Receipt{}, change.ErrConflict
		}
		// The last database clock check is the admission point. No external I/O is
		// performed here; expired previews cannot ride an earlier preflight check.
		admitted, e := repo.Now(call)
		if e != nil {
			return change.Receipt{}, e
		}
		if !admitted.Before(p.ExpiresAt) {
			return change.Receipt{}, change.ErrExpired
		}
		if mode == change.Immediate {
			_, at, end, e = change.Period(p.Input, m.before, p.Classification, admitted, m.target.Terms)
			if e != nil {
				return change.Receipt{}, e
			}
		}
		after := m.before
		after.Revision++
		after.EntitlementSourceVersion = m.state.Version
		if p.Input.Action != change.StopRenewal {
			after.State = s.lifecycle.StateFor(m.target.PlanCode, m.target.Number)
		}
		v := change.Receipt{ChangeID: r.ChangeId, TenantID: r.TenantId, ActorID: a, RequestID: r.RequestId, Fingerprint: fingerprint, PreviewHash: p.Hash, Action: p.Input.Action, Status: change.Applied, Mode: mode, ConfirmedAt: admitted, EffectiveAt: at, EntitlementExpiresAt: end, Reason: reason, Before: m.before, BeforeSourceVersion: m.state.Version, AfterSourceVersion: m.state.Version, BeforeEntitlementVersion: m.current.EntitlementVersion, AfterEntitlementVersion: m.current.EntitlementVersion, QuotaValidationRequired: deferred, Quotas: quotas, PricingAuthority: "PLATFORM_MANUAL_APPROVAL"}
		if mode == change.Immediate && len(requirements) > 0 {
			after.PendingChangeID = r.ChangeId
			task, e := pv.New(r.TenantId, pv.Approval{ChangeID: r.ChangeId, ActorID: a, PreviewHash: p.Hash, TargetHash: m.target.ContentSHA256, TargetPlanCode: m.target.PlanCode, TargetPlanVersion: m.target.Number, SubscriptionRevision: after.Revision, SourceVersion: m.state.Version, EntitlementVersion: m.current.EntitlementVersion, CatalogRevision: m.current.CatalogRevision}, requirements, admitted)
			if e != nil {
				return v, e
			}
			if e = repos.Tasks.Insert(call, task); e != nil {
				return v, e
			}
			v.Status = change.Provisioning
			v.ProvisioningTaskID = task.ID
		} else if mode == change.Scheduled {
			// Reservation changes the subscription revision, not effective rights.
			// A second confirmation with an old preview cannot reserve another change.
			after.PendingChangeID = r.ChangeId
			v.AfterSourceVersion, v.AfterEntitlementVersion, e = s.installTimeFence(call, repos, m, &after, r.ChangeId, at, admitted)
			if e != nil {
				return v, e
			}
			v.Status = change.Scheduled
		} else if p.Input.Action == change.StopRenewal {
			after.RenewalStopped = true
		} else {
			v.AfterSourceVersion, v.AfterEntitlementVersion, e = s.applySources(call, repos, m, &after, r.ChangeId, at, end)
			if e != nil {
				return v, e
			}
		}
		if e = repo.SaveCurrent(call, m.before, after); e != nil {
			return v, e
		}
		v.After = after
		v = v.Seal()
		if e = repo.Complete(call, v); e != nil {
			return v, e
		}
		return v, nil
	})
	if err != nil {
		return nil, expose(err)
	}
	return receiptDTO(result), nil
}
