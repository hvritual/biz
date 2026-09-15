package usecase

import (
	"context"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/requestscope"
)

func (s *service) PreviewSubscriptionChange(ctx context.Context, r *v1.PreviewSubscriptionChangeRequest) (*v1.SubscriptionChangePreviewDTO, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	i, err := input(r)
	if err != nil {
		return nil, expose(err)
	}
	return s.preview(ctx, a, i, false)
}

func (s *service) preview(ctx context.Context, actorID string, input change.Input, tenantSelfService bool) (*v1.SubscriptionChangePreviewDTO, error) {
	if !validKey(ctx, input.RequestID) {
		return nil, expose(change.ErrInvalid)
	}
	id := change.ID(actorID, input.TenantID, input.RequestID)
	result, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionChangeRepositories]) (change.Preview, error) {
		repos, call := sc.Repositories(), sc.Context()
		repo := repos.Changes
		raw, err := repo.LockTenant(call, input.TenantID)
		if err != nil {
			return change.Preview{}, err
		}
		i := input
		if tenantSelfService {
			i, err = normalizeTenantPreviewInput(i, raw)
			if err != nil {
				return change.Preview{}, err
			}
		} else if err = i.Validate(); err != nil {
			return change.Preview{}, err
		}
		existing, err := repo.PreviewForRequest(call, actorID, i.RequestID, change.Digest(i))
		if err != nil {
			return change.Preview{}, err
		}
		if existing != nil {
			return *existing, nil
		}
		if raw.PendingChangeID != "" {
			return change.Preview{}, change.ErrPending
		}
		if i.Action == change.StopRenewal && raw.RenewalStopped {
			return change.Preview{}, change.ErrConflict
		}
		var material material
		if tenantSelfService {
			material, err = s.captureTenant(call, repos, raw, i)
		} else {
			material, err = s.capture(call, repos, raw, i)
		}
		if err != nil {
			return change.Preview{}, err
		}
		now, err := repo.Now(call)
		if err != nil {
			return change.Preview{}, err
		}
		classification := i.Action
		if i.Action == change.Switch {
			classification = change.Classify(material.old.Terms, material.target.Terms)
		}
		mode, at, end, err := change.Period(i, material.before, classification, now, material.target.Terms)
		if err != nil {
			return change.Preview{}, err
		}
		expires := now.Add(10 * time.Minute)
		if boundary := material.current.ValidUntil; boundary != nil && boundary.Before(expires) {
			expires = *boundary
		}
		if !expires.After(now) {
			return change.Preview{}, change.ErrExpired
		}
		projected := material.current
		quotas := []change.QuotaImpact{}
		deferred := false
		if i.Action != change.StopRenewal {
			sources, err := change.ProjectSources(material.before, material.old, material.target, material.state.Sources, id, at, end)
			if err != nil {
				return change.Preview{}, err
			}
			projected, err = entitlement.Resolve(i.TenantID, material.state.Version+1, at, material.catalog, sources, nil)
			if err != nil {
				return change.Preview{}, err
			}
			requested := change.QuotaChanges(material.old.Terms, material.target.Terms)
			quotas, err = s.quotas.Evaluate(call, ports.QuotaChangeInput{TenantID: i.TenantID, ChangeID: id, Mode: mode, EffectiveAt: at, Changes: requested})
			if err != nil {
				return change.Preview{}, err
			}
			deferred, err = change.CheckQuotaReport(requested, quotas, mode)
			if err != nil {
				return change.Preview{}, err
			}
		}
		// A projection is not a new immutable snapshot and must never be cached as one.
		projected.EntitlementVersion = 0
		projected.CatalogRevision = material.current.CatalogRevision
		pricing := "NO_PRICE_REFERENCE"
		if i.Action != change.StopRenewal && material.target.Terms.PriceRef != "" {
			pricing = "PLATFORM_MANUAL_APPROVAL_REQUIRED"
		}
		impacts := []string{"Existing tenant data is preserved; this operation never deletes resources.", "Projected rights include existing overrides and safety restrictions.", "No resource consumption or output-field enforcement is added by CE-09."}
		if tenantSelfService {
			impacts = append(impacts, "Tenant self-service preview only: subscription and entitlement authority remain unchanged until a separate confirmation succeeds.")
			if pricing != "NO_PRICE_REFERENCE" {
				impacts = append(impacts, "This target carries a price reference. Tenant confirmation is fail-closed until the external commercial/payment approval authority has admitted the change.")
			}
		}
		if mode == change.Scheduled {
			impacts = append(impacts, "Scheduled intent only: existing rights remain unchanged until a future validated executor applies it.")
		}
		if deferred {
			impacts = append(impacts, "Quota usage is unknown or over target: execution requires authoritative revalidation; no zero usage is assumed.")
		}
		if i.Action == change.StopRenewal {
			impacts = append(impacts, "Stops renewal intent only; does not shorten the current entitlement period or cancel external billing.")
		}
		requirements, err := s.preparationRequirements(call, i.Action, material.target)
		if err != nil {
			return change.Preview{}, err
		}
		if len(requirements) > 0 {
			impacts = append(impacts, "External preparation is required. Confirmation reserves intent; effective rights remain unchanged until actual readiness and authoritative activation.")
		}
		preview := change.Preview{ProvisioningRequirements: requirements, ChangeID: id, ActorID: actorID, Input: i, Fingerprint: change.Digest(i), Before: material.before, Target: material.target, Current: material.current, Projected: projected, Classification: classification, Mode: mode, CreatedAt: now, ExpiresAt: expires, EffectiveAt: at, EntitlementExpiresAt: end, Dependencies: material.dependencies, Quotas: quotas, Impacts: impacts, QuotaValidationRequired: deferred, PricingBasis: pricing}.Seal()
		if err = repo.SavePreview(call, preview); err != nil {
			return preview, err
		}
		return preview, nil
	})
	if err != nil {
		return nil, expose(err)
	}
	return previewDTO(result), nil
}
