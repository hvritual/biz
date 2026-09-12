package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	app "github.com/hvritual/biz/internal/commercial/application"
	projection "github.com/hvritual/biz/internal/commercial/application/planprojection"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	pvmodel "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	"github.com/hvritual/biz/internal/commercial/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"strings"
	"time"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
)

type service struct {
	provisioningPolicy ports.ProvisioningPolicy
	repositories       requestscope.RepositoryFactory[ports.SubscriptionRepositories]
	capabilities       app.SubscriptionManagementCapabilities
}

func New(r requestscope.RepositoryFactory[ports.SubscriptionRepositories], c app.SubscriptionManagementCapabilities, policies ...ports.ProvisioningPolicy) (app.SubscriptionManagementApplication, error) {
	if r == nil || c == nil || c.CommercialPlanManagement() == nil {
		return nil, errors.New("subscriptions: repositories and plan capability required")
	}
	var policy ports.ProvisioningPolicy = ports.DatabaseOnlyProvisioning{}
	if len(policies) > 1 {
		return nil, errors.New("subscriptions: one local preparation policy required")
	}
	if len(policies) == 1 && policies[0] != nil {
		policy = policies[0]
	}
	return &service{repositories: r, capabilities: c, provisioningPolicy: policy}, nil
}
func actor(ctx context.Context) (string, error) {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.Subject == "" || p.TenantID != "" {
		return "", subscription.ErrScope
	}
	return p.Subject, nil
}
func exposed(e error) error {
	if e == nil {
		return nil
	}
	c := codes.InvalidArgument
	switch {
	case errors.Is(e, subscription.ErrNotFound):
		c = codes.NotFound
	case errors.Is(e, subscription.ErrScope):
		c = codes.PermissionDenied
	case errors.Is(e, subscription.ErrConflict), errors.Is(e, subscription.ErrRequestConflict):
		c = codes.Aborted
	case errors.Is(e, subscription.ErrNoEligibleDefault):
		c = codes.FailedPrecondition
	default:
		if !errors.Is(e, subscription.ErrInvalid) {
			return status.Error(codes.Unavailable, "SUBSCRIPTION_AUTHORITY_UNAVAILABLE")
		}
	}
	return status.Error(c, e.Error())
}
func fp(actor, op string, m proto.Message) string {
	b, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(m)
	h := sha256.Sum256(append([]byte(op+"\x00"+actor+"\x00"), b...))
	return hex.EncodeToString(h[:])
}
func dto(v subscription.Subscription) *v1.TenantSubscriptionDTO { return projection.SubscriptionDTO(v) }
func ruleDTO(v subscription.Rule) *v1.DefaultSubscriptionRuleDTO {
	return &v1.DefaultSubscriptionRuleDTO{RuleId: v.RuleID, Version: v.Version, Priority: v.Priority, SalesScope: v.SalesScope, PlanCode: v.PlanCode, PlanVersion: v.PlanVersion, Enabled: v.Enabled, Reason: v.Reason, ActorId: v.ActorID, UpdatedAt: v.UpdatedAt.Format(time.RFC3339Nano)}
}
func (s *service) PutDefaultSubscriptionRule(ctx context.Context, r *v1.PutDefaultSubscriptionRuleRequest) (*v1.DefaultSubscriptionRuleDTO, error) {
	a, e := actor(ctx)
	if e != nil {
		return nil, exposed(e)
	}
	if r == nil || !subscription.ValidCode(r.RuleId) || (r.SalesScope != "*" && !subscription.ValidCode(r.SalesScope)) || !subscription.ValidCode(r.PlanCode) || r.PlanVersion == 0 || r.RequestId == "" || strings.TrimSpace(r.Reason) == "" {
		return nil, exposed(subscription.ErrInvalid)
	}
	out, e := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionRepositories]) (subscription.Rule, error) {
		repo := sc.Repositories().Subscriptions
		call := sc.Context()
		if e := repo.LockRules(call); e != nil {
			return subscription.Rule{}, e
		}
		replay, e := repo.RuleReceipt(call, r.RuleId, r.RequestId, fp(a, "rule", r))
		if e != nil || replay != nil {
			if replay != nil {
				return *replay, nil
			}
			return subscription.Rule{}, e
		}
		old, e := repo.GetRule(call, r.RuleId, true)
		expected := r.ExpectedVersion
		if errors.Is(e, subscription.ErrNotFound) {
			if expected != 0 {
				return subscription.Rule{}, subscription.ErrConflict
			}
			old = subscription.Rule{}
		} else if e != nil {
			return subscription.Rule{}, e
		} else if old.Version != expected {
			return subscription.Rule{}, subscription.ErrConflict
		}
		if !r.Enabled && old.Version > 0 {
			if old.PlanCode != r.PlanCode || old.PlanVersion != r.PlanVersion || old.SalesScope != r.SalesScope {
				return subscription.Rule{}, subscription.ErrConflict
			}
		} else {
			eligibilityScope := r.SalesScope
			if eligibilityScope == "*" {
				// Wildcard rules are allowed only when the referenced immutable plan is
				// itself globally applicable. Eligibility still receives a concrete scope.
				eligibilityScope = "default"
			}
			elig, e := s.capabilities.CommercialPlanManagement().CheckPlanEligibility(ctx, &v1.CheckPlanEligibilityRequest{PlanCode: r.PlanCode, Version: r.PlanVersion, SalesScope: eligibilityScope})
			if e != nil || elig == nil || !elig.Eligible || elig.Version == nil {
				return subscription.Rule{}, subscription.ErrNoEligibleDefault
			}
			if r.SalesScope == "*" {
				terms := elig.Version.GetTerms()
				if terms == nil || len(terms.GetSalesScope()) != 1 || terms.GetSalesScope()[0] != "*" {
					return subscription.Rule{}, subscription.ErrNoEligibleDefault
				}
			}
		}
		now, e := repo.Now(call)
		if e != nil {
			return subscription.Rule{}, e
		}
		v := subscription.Rule{RuleID: r.RuleId, Version: expected + 1, Priority: r.Priority, SalesScope: r.SalesScope, PlanCode: r.PlanCode, PlanVersion: r.PlanVersion, Enabled: r.Enabled, Reason: r.Reason, ActorID: a, UpdatedAt: now}
		if e = v.Validate(); e != nil {
			return v, e
		}
		if e = repo.SaveRule(call, v, expected); e != nil {
			return v, e
		}
		if e = repo.SaveRuleReceipt(call, r.RuleId, r.RequestId, fp(a, "rule", r), v); e != nil {
			return v, e
		}
		return v, nil
	})
	if e != nil {
		return nil, exposed(e)
	}
	return ruleDTO(out), nil
}
func (s *service) ListDefaultSubscriptionRules(ctx context.Context, _ *v1.ListDefaultSubscriptionRulesRequest) (*v1.ListDefaultSubscriptionRulesResponse, error) {
	if _, e := actor(ctx); e != nil {
		return nil, exposed(e)
	}
	out, e := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionRepositories]) ([]subscription.Rule, error) {
		return sc.Repositories().Subscriptions.ListRules(sc.Context(), false)
	})
	if e != nil {
		return nil, exposed(e)
	}
	res := &v1.ListDefaultSubscriptionRulesResponse{}
	for _, v := range subscription.Ordered(out) {
		res.Rules = append(res.Rules, ruleDTO(v))
	}
	return res, nil
}
func planSources(tenant, sub string, at time.Time, p *v1.PlanVersionDTO) []entitlement.Source {
	terms, err := projection.Terms(p.GetTerms())
	if err != nil {
		return nil
	}
	return subscription.Sources(tenant, sub, at, terms)
}
func (s *service) BootstrapBaseSubscription(ctx context.Context, r *v1.BootstrapTenantSubscriptionRequest) (*v1.BootstrapTenantSubscriptionResult, error) {
	a, e := actor(ctx)
	if e != nil {
		return nil, exposed(e)
	}
	if r == nil || r.RequestId == "" || r.TenantId == "" || !subscription.ValidCode(r.SalesScope) {
		return nil, exposed(subscription.ErrInvalid)
	}
	hash := fp(a, "bootstrap", r)
	out, e := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionRepositories]) (subscription.Subscription, error) {
		repo := sc.Repositories().Subscriptions
		call := sc.Context()
		if e := repo.LockRules(call); e != nil {
			return subscription.Subscription{}, e
		}
		if replay, e := repo.BootstrapReceipt(call, r.TenantId, r.RequestId, hash); e != nil || replay != nil {
			if replay != nil {
				return *replay, nil
			}
			return subscription.Subscription{}, e
		}
		if existing, e := repo.GetBase(call, r.TenantId, true); e == nil {
			if existing.SalesScope != r.SalesScope {
				return subscription.Subscription{}, subscription.ErrRequestConflict
			}
			if e := repo.SaveBootstrapReceipt(call, r.TenantId, r.RequestId, hash, existing); e != nil {
				return subscription.Subscription{}, e
			}
			return existing, nil
		} else if !errors.Is(e, subscription.ErrNotFound) {
			return subscription.Subscription{}, e
		}
		rules, e := repo.ListRules(call, true)
		if e != nil {
			return subscription.Subscription{}, e
		}
		var chosen subscription.Rule
		var pv *v1.PlanVersionDTO
		for _, rule := range subscription.Ordered(rules) {
			if !subscription.Matches(rule, r.SalesScope) {
				continue
			}
			elig, e := s.capabilities.CommercialPlanManagement().CheckPlanEligibility(ctx, &v1.CheckPlanEligibilityRequest{PlanCode: rule.PlanCode, Version: rule.PlanVersion, SalesScope: r.SalesScope})
			if e != nil {
				if status.Code(e) == codes.NotFound {
					continue
				}
				return subscription.Subscription{}, e
			}
			if elig == nil {
				return subscription.Subscription{}, errors.New("subscription: missing authoritative eligibility")
			}
			if elig.Eligible && elig.Version != nil {
				// CE-08 cannot bypass actual external preparation via a default plan.
				version, err := projection.Version(elig.Version)
				if err != nil {
					return subscription.Subscription{}, err
				}
				requirements, err := s.provisioningPolicy.Requirements(call, version)
				if err != nil {
					return subscription.Subscription{}, err
				}
				if err = pvmodel.ValidateRequirements(requirements); err != nil {
					return subscription.Subscription{}, err
				}
				if len(requirements) > 0 {
					continue
				}
				chosen = rule
				pv = elig.Version
				break
			}
		}
		if pv == nil {
			return subscription.Subscription{}, subscription.ErrNoEligibleDefault
		}
		now, e := repo.Now(call)
		if e != nil {
			return subscription.Subscription{}, e
		}
		state, e := sc.Repositories().Entitlements.Lock(call, r.TenantId)
		if e != nil {
			return subscription.Subscription{}, e
		}
		sidSum := sha256.Sum256([]byte(r.TenantId))
		sid := "sub-" + hex.EncodeToString(sidSum[:12])
		for _, src := range planSources(r.TenantId, sid, now, pv) {
			if e := src.ValidateShape(); e != nil {
				return subscription.Subscription{}, e
			}
			if e := sc.Repositories().Entitlements.Insert(call, src); e != nil {
				return subscription.Subscription{}, e
			}
		}
		if e := sc.Repositories().Entitlements.Advance(call, r.TenantId, state.Version); e != nil {
			return subscription.Subscription{}, e
		}
		v := subscription.Subscription{ID: sid, TenantID: r.TenantId, Kind: subscription.KindBase, State: subscription.StateActive, PlanCode: chosen.PlanCode, PlanVersion: chosen.PlanVersion, RuleID: chosen.RuleID, RuleVersion: chosen.Version, SalesScope: r.SalesScope, EntitlementSourceVersion: state.Version + 1, CreatedAt: now, MatchExplanation: fmt.Sprintf("rule=%s@%d priority=%d scope=%s", chosen.RuleID, chosen.Version, chosen.Priority, chosen.SalesScope)}
		v.Revision = 1
		v.PeriodStart = now
		v.SourceNamespace = sid
		if pv.Terms.ValidityMode == "fixed_days" {
			end := now.AddDate(0, 0, int(pv.Terms.ValidityDays))
			v.PeriodEnd = &end
		}

		if e = v.Validate(); e != nil {
			return v, e
		}
		if e = repo.SaveBase(call, v); e != nil {
			return v, e
		}
		if e = repo.SaveBootstrapReceipt(call, r.TenantId, r.RequestId, hash, v); e != nil {
			return v, e
		}
		if e = repo.Audit(call, ports.SubscriptionAudit{TenantID: r.TenantId, RequestID: r.RequestId, ActorID: a, Action: "bootstrap_base", Reason: "tenant creation default", Subscription: v, At: now}); e != nil {
			return v, e
		}
		return v, nil
	})
	if e != nil {
		return nil, exposed(e)
	}
	return &v1.BootstrapTenantSubscriptionResult{Subscription: dto(out)}, nil
}
func (s *service) GetTenantSubscription(ctx context.Context, r *v1.GetTenantSubscriptionRequest) (*v1.TenantSubscriptionDTO, error) {
	if _, e := actor(ctx); e != nil {
		return nil, exposed(e)
	}
	if r == nil || r.TenantId == "" {
		return nil, exposed(subscription.ErrInvalid)
	}
	v, e := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.SubscriptionRepositories]) (subscription.Subscription, error) {
		return sc.Repositories().Subscriptions.GetBase(sc.Context(), r.TenantId, false)
	})
	if e != nil {
		return nil, exposed(e)
	}
	return dto(v), nil
}
