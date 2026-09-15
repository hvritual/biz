package usecase

import (
	"context"
	"strings"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
)

func tenantChangeActor(ctx context.Context) (identity.Principal, error) {
	principal, ok := identity.FromContext(ctx)
	if !ok || !principal.Authenticated || principal.Subject == "" || len(principal.Subject) > 200 || principal.TenantID == "" || !change.Tenant(principal.TenantID) {
		return identity.Principal{}, change.ErrScope
	}
	return principal, nil
}

func (s *service) ListMySubscriptionChangeTargets(ctx context.Context, _ *v1.ListMySubscriptionChangeTargetsRequest) (*v1.ListMySubscriptionChangeTargetsResponse, error) {
	principal, err := tenantChangeActor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	response, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.SubscriptionChangeRepositories]) (*v1.ListMySubscriptionChangeTargetsResponse, error) {
		raw, err := scope.Repositories().Changes.LockTenant(scope.Context(), principal.TenantID)
		if err != nil {
			return nil, err
		}
		targets, err := s.capabilities.CommercialPlanManagement().ListTenantChangeTargets(scope.Context(), &v1.ListTenantChangeTargetsRequest{SalesScope: raw.SalesScope})
		if err != nil {
			return nil, err
		}
		out := &v1.ListMySubscriptionChangeTargetsResponse{SalesScope: raw.SalesScope}
		if targets == nil {
			return out, nil
		}
		for _, target := range targets.Targets {
			if target == nil {
				return nil, change.ErrCorrupt
			}
			if target.PlanCode == raw.PlanCode && target.Version == raw.PlanVersion {
				continue
			}
			out.Targets = append(out.Targets, target)
		}
		return out, nil
	})
	if err != nil {
		return nil, expose(err)
	}
	return response, nil
}

func (s *service) PreviewMySubscriptionChange(ctx context.Context, request *v1.PreviewMySubscriptionChangeRequest) (*v1.SubscriptionChangePreviewDTO, error) {
	principal, err := tenantChangeActor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	input, err := tenantPreviewInput(principal.TenantID, request)
	if err != nil {
		return nil, expose(err)
	}
	return s.preview(ctx, principal.Subject, input, true)
}

func (s *service) GetMySubscriptionChangePreview(ctx context.Context, request *v1.ReadMySubscriptionChangePreviewRequest) (*v1.SubscriptionChangePreviewDTO, error) {
	principal, err := tenantChangeActor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if request == nil || !change.Key(request.ChangeId) {
		return nil, expose(change.ErrInvalid)
	}
	preview, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.SubscriptionChangeRepositories]) (*change.Preview, error) {
		return scope.Repositories().Changes.Preview(scope.Context(), principal.TenantID, request.ChangeId, false)
	})
	if err != nil {
		return nil, expose(err)
	}
	if preview == nil {
		return nil, expose(change.ErrNotFound)
	}
	if preview.ActorID != principal.Subject || preview.Input.TenantID != principal.TenantID {
		return nil, expose(change.ErrScope)
	}
	return previewDTO(*preview), nil
}

func tenantPreviewInput(tenantID string, request *v1.PreviewMySubscriptionChangeRequest) (change.Input, error) {
	if request == nil || !change.Tenant(tenantID) {
		return change.Input{}, change.ErrInvalid
	}
	input := change.Input{
		TenantID:          tenantID,
		RequestID:         request.RequestId,
		Action:            request.Action,
		TargetPlanCode:    request.TargetPlanCode,
		TargetPlanVersion: request.TargetPlanVersion,
		Reason:            strings.TrimSpace(request.Reason),
	}
	if request.EffectiveAt != "" {
		value, err := time.Parse(time.RFC3339Nano, request.EffectiveAt)
		if err != nil {
			return input, change.ErrInvalid
		}
		canonical := change.CanonicalTime(value)
		input.EffectiveAt = &canonical
	}
	// SWITCH validates immediately. RENEW needs the locked current plan before
	// the existing domain Input can be completed. STOP_RENEWAL must remain targetless.
	switch input.Action {
	case change.Switch:
		return input, input.Validate()
	case change.Renew:
		if input.TargetPlanCode != "" || input.TargetPlanVersion != 0 {
			return input, change.ErrInvalid
		}
		return input, nil
	case change.StopRenewal:
		return input, input.Validate()
	default:
		return input, change.ErrInvalid
	}
}

func normalizeTenantPreviewInput(input change.Input, current subscription.Subscription) (change.Input, error) {
	if input.TenantID != current.TenantID {
		return input, change.ErrScope
	}
	switch input.Action {
	case change.Renew:
		if input.TargetPlanCode != "" || input.TargetPlanVersion != 0 {
			return input, change.ErrInvalid
		}
		input.TargetPlanCode = current.PlanCode
		input.TargetPlanVersion = current.PlanVersion
	case change.Switch:
		// Target is intentionally client-selected, then resolved against the
		// server-owned sales scope by the private Plan capability.
	case change.StopRenewal:
		// Domain validation requires no target and no effective_at.
	default:
		return input, change.ErrInvalid
	}
	return input, input.Validate()
}
