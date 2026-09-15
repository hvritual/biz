package usecase

import (
	"context"
	"strings"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	pv "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/requestscope"
)

func (s *service) ConfirmSubscriptionChange(ctx context.Context, r *v1.ConfirmSubscriptionChangeRequest) (*v1.SubscriptionChangeReceiptDTO, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if r == nil {
		return nil, expose(change.ErrInvalid)
	}
	return s.confirm(ctx, a, r.TenantId, r.ChangeId, r.RequestId, r.PreviewHash, r.Reason, false)
}

func (s *service) confirm(ctx context.Context, actorID, tenantID, changeID, requestID, previewHash, reason string, tenantSelfService bool) (*v1.SubscriptionChangeReceiptDTO, error) {
	if !change.Tenant(tenantID) || !change.Key(changeID) || !validKey(ctx, requestID) || len(previewHash) != 64 || !change.Reason(reason) {
		return nil, expose(change.ErrInvalid)
	}
	reason = strings.TrimSpace(reason)
	fingerprint := change.Digest([]string{actorID, tenantID, requestID, changeID, previewHash, reason})
	result, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionChangeRepositories]) (change.Receipt, error) {
		repos, call := sc.Repositories(), sc.Context()
		repo := repos.Changes
		raw, err := repo.LockTenant(call, tenantID)
		if err != nil {
			return change.Receipt{}, err
		}
		replay, err := repo.ReceiptForRequest(call, tenantID, actorID, requestID, fingerprint)
		if err != nil {
			return change.Receipt{}, err
		}
		if replay != nil {
			return *replay, nil
		}
		already, err := repo.Receipt(call, tenantID, changeID, true)
		if err != nil {
			return change.Receipt{}, err
		}
		if already != nil {
			return change.Receipt{}, change.ErrRequestConflict
		}
		preview, err := repo.Preview(call, tenantID, changeID, true)
		if err != nil {
			return change.Receipt{}, err
		}
		if preview == nil {
			return change.Receipt{}, change.ErrNotFound
		}
		if preview.ActorID != actorID || preview.Input.TenantID != tenantID {
			return change.Receipt{}, change.ErrScope
		}
		if preview.Hash != previewHash {
			return change.Receipt{}, change.ErrConflict
		}
		if tenantSelfService && preview.PricingBasis != "NO_PRICE_REFERENCE" {
			return change.Receipt{}, errExternalApprovalRequired
		}
		now, err := repo.Now(call)
		if err != nil {
			return change.Receipt{}, err
		}
		if !now.Before(preview.ExpiresAt) {
			return change.Receipt{}, change.ErrExpired
		}
		if raw.PendingChangeID != "" {
			return change.Receipt{}, change.ErrPending
		}
		var material material
		if tenantSelfService {
			material, err = s.captureTenant(call, repos, raw, preview.Input)
		} else {
			material, err = s.capture(call, repos, raw, preview.Input)
		}
		if err != nil {
			return change.Receipt{}, err
		}
		if change.Digest(material.before) != change.Digest(preview.Before) || material.current.SourceVersion != preview.Current.SourceVersion || material.current.EntitlementVersion != preview.Current.EntitlementVersion || material.current.CatalogRevision != preview.Current.CatalogRevision || material.target.ContentSHA256 != preview.Target.ContentSHA256 {
			return change.Receipt{}, change.ErrConflict
		}
		mode, at, end, err := change.Period(preview.Input, material.before, preview.Classification, now, material.target.Terms)
		if err != nil {
			return change.Receipt{}, err
		}
		if mode != preview.Mode || (mode == change.Scheduled && !at.Equal(preview.EffectiveAt)) {
			return change.Receipt{}, change.ErrConflict
		}
		if mode == change.Scheduled && !at.After(now) {
			return change.Receipt{}, change.ErrExpired
		}
		requested := change.QuotaChanges(material.old.Terms, material.target.Terms)
		if preview.Input.Action == change.StopRenewal {
			requested = nil
		}
		quotas, err := s.quotas.Evaluate(call, ports.QuotaChangeInput{TenantID: tenantID, ChangeID: changeID, Mode: mode, EffectiveAt: at, Changes: requested})
		if err != nil {
			return change.Receipt{}, err
		}
		deferred, err := change.CheckQuotaReport(requested, quotas, mode)
		if err != nil {
			return change.Receipt{}, err
		}
		requirements, err := s.preparationRequirements(call, preview.Input.Action, material.target)
		if err != nil {
			return change.Receipt{}, err
		}
		if change.Digest(requirements) != change.Digest(preview.ProvisioningRequirements) {
			return change.Receipt{}, change.ErrConflict
		}
		// The last database clock check is the admission point. No external I/O is
		// performed here; expired previews cannot ride an earlier preflight check.
		admitted, err := repo.Now(call)
		if err != nil {
			return change.Receipt{}, err
		}
		if !admitted.Before(preview.ExpiresAt) {
			return change.Receipt{}, change.ErrExpired
		}
		if mode == change.Immediate {
			_, at, end, err = change.Period(preview.Input, material.before, preview.Classification, admitted, material.target.Terms)
			if err != nil {
				return change.Receipt{}, err
			}
		}
		after := material.before
		after.Revision++
		after.EntitlementSourceVersion = material.state.Version
		if preview.Input.Action != change.StopRenewal {
			after.State = s.lifecycle.StateFor(material.target.PlanCode, material.target.Number)
		}
		pricingAuthority := "PLATFORM_MANUAL_APPROVAL"
		if tenantSelfService {
			pricingAuthority = "TENANT_SELF_SERVICE_NO_PRICE_REFERENCE"
		}
		receipt := change.Receipt{ChangeID: changeID, TenantID: tenantID, ActorID: actorID, RequestID: requestID, Fingerprint: fingerprint, PreviewHash: preview.Hash, Action: preview.Input.Action, Status: change.Applied, Mode: mode, ConfirmedAt: admitted, EffectiveAt: at, EntitlementExpiresAt: end, Reason: reason, Before: material.before, BeforeSourceVersion: material.state.Version, AfterSourceVersion: material.state.Version, BeforeEntitlementVersion: material.current.EntitlementVersion, AfterEntitlementVersion: material.current.EntitlementVersion, QuotaValidationRequired: deferred, Quotas: quotas, PricingAuthority: pricingAuthority}
		if mode == change.Immediate && len(requirements) > 0 {
			after.PendingChangeID = changeID
			task, err := pv.New(tenantID, pv.Approval{ChangeID: changeID, ActorID: actorID, PreviewHash: preview.Hash, TargetHash: material.target.ContentSHA256, TargetPlanCode: material.target.PlanCode, TargetPlanVersion: material.target.Number, SubscriptionRevision: after.Revision, SourceVersion: material.state.Version, EntitlementVersion: material.current.EntitlementVersion, CatalogRevision: material.current.CatalogRevision}, requirements, admitted)
			if err != nil {
				return receipt, err
			}
			if err = repos.Tasks.Insert(call, task); err != nil {
				return receipt, err
			}
			receipt.Status = change.Provisioning
			receipt.ProvisioningTaskID = task.ID
		} else if mode == change.Scheduled {
			// Reservation changes the subscription revision, not effective rights.
			// A second confirmation with an old preview cannot reserve another change.
			after.PendingChangeID = changeID
			receipt.AfterSourceVersion, receipt.AfterEntitlementVersion, err = s.installTimeFence(call, repos, material, &after, changeID, at, admitted)
			if err != nil {
				return receipt, err
			}
			receipt.Status = change.Scheduled
		} else if preview.Input.Action == change.StopRenewal {
			after.RenewalStopped = true
		} else {
			receipt.AfterSourceVersion, receipt.AfterEntitlementVersion, err = s.applySources(call, repos, material, &after, changeID, at, end)
			if err != nil {
				return receipt, err
			}
		}
		if err = repo.SaveCurrent(call, material.before, after); err != nil {
			return receipt, err
		}
		receipt.After = after
		receipt = receipt.Seal()
		if err = repo.Complete(call, receipt); err != nil {
			return receipt, err
		}
		return receipt, nil
	})
	if err != nil {
		return nil, expose(err)
	}
	return receiptDTO(result), nil
}
