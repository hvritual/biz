package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	app "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	"github.com/hvritual/biz/internal/commercial/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"regexp"
	"strings"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
)

type service struct {
	repositories requestscope.RepositoryFactory[ports.PlanRepositories]
	capabilities app.PlanManagementCapabilities
}

func New(r requestscope.RepositoryFactory[ports.PlanRepositories], c app.PlanManagementCapabilities) (app.PlanManagementApplication, error) {
	if r == nil || c == nil || c.CommercialModuleCatalog() == nil {
		return nil, errors.New("plans: repositories and typed catalog required")
	}
	return &service{r, c}, nil
}
func actor(ctx context.Context) (string, error) {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.Subject == "" || len(p.Subject) > 200 || p.TenantID != "" {
		return "", plan.ErrScope
	}
	return p.Subject, nil
}

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func exposed(err error) error {
	if err == nil {
		return nil
	}
	c := codes.InvalidArgument
	switch {
	case errors.Is(err, plan.ErrNotFound):
		c = codes.NotFound
	case errors.Is(err, plan.ErrScope):
		c = codes.PermissionDenied
	case errors.Is(err, plan.ErrConflict), errors.Is(err, plan.ErrRequestConflict):
		c = codes.Aborted
	case errors.Is(err, plan.ErrImmutable), errors.Is(err, plan.ErrCatalog):
		c = codes.FailedPrecondition
	case errors.Is(err, plan.ErrCorrupt):
		c = codes.DataLoss
	default:
		if !errors.Is(err, plan.ErrInvalid) {
			return status.Error(codes.Unavailable, "PLAN_AUTHORITY_UNAVAILABLE")
		}
	}
	return status.Error(c, err.Error())
}
func (s *service) catalog(ctx context.Context) (plan.Catalog, error) {
	response, err := s.capabilities.CommercialModuleCatalog().ReadPlanCatalog(ctx, &v1.ListModulesRequest{})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, plan.ErrCatalog
	}
	out := plan.Catalog{}
	base := entitlement.Catalog{}
	for _, m := range response.Modules {
		if m == nil {
			return nil, plan.ErrCatalog
		}
		tech := "not_ready"
		if m.TechnicalStatus == v1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY {
			tech = "ready"
		}
		if m.TechnicalStatus == v1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_DISABLED {
			tech = "disabled"
		}
		sales := "retired"
		if m.SalesStatus == v1.ModuleSalesStatus_MODULE_SALES_STATUS_SELLABLE {
			sales = "sellable"
		}
		d := entitlement.ModuleDefinition{Code: m.ModuleCode, Version: m.Version, TechnicalStatus: tech, SalesStatus: sales, Capabilities: m.CapabilityCodes, QuotaKeys: m.QuotaSchemaKeys, FieldKeys: m.FieldPolicySchemaKeys, Dependencies: m.Dependencies}
		base = append(base, d)
		out = append(out, plan.Definition{ModuleDefinition: d, SalesScope: m.SalesScope})
	}
	if err := base.Validate(); err != nil {
		return nil, plan.ErrCatalog
	}
	return out, nil
}

type command struct {
	operation, key, code, name, reason string
	number, expected                   uint64
	terms                              *v1.PlanTerms
	request                            proto.Message
}

func (s *service) mutate(ctx context.Context, c command) (*v1.PlanVersionDTO, error) {
	a, err := actor(ctx)
	if err != nil {
		return nil, exposed(err)
	}
	if !plan.Code(c.code) || !keyPattern.MatchString(c.key) || strings.TrimSpace(c.reason) == "" || len(c.reason) > 512 {
		return nil, exposed(plan.ErrInvalid)
	}
	b, err := (proto.MarshalOptions{Deterministic: true}).Marshal(c.request)
	if err != nil {
		return nil, exposed(plan.ErrInvalid)
	}
	h := sha256.Sum256(append([]byte(c.operation+"\x00"+a+"\x00"), b...))
	hash := hex.EncodeToString(h[:])
	// This generated child takes the catalog SHARE lock before the plan aggregate.
	catalog, err := s.catalog(ctx)
	if err != nil {
		return nil, exposed(err)
	}
	out, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.PlanRepositories]) (plan.Version, error) {
		r := scope.Repositories().Plans
		call := scope.Context()
		head, err := r.Lock(call, c.code)
		if err != nil {
			return plan.Version{}, err
		}
		replay, err := r.Receipt(call, c.code, c.key, hash)
		if err != nil {
			return plan.Version{}, err
		}
		if replay != nil {
			return *replay, nil
		}
		if head.Revision == ^uint64(0) || head.Latest == ^uint64(0) {
			return plan.Version{}, plan.ErrConflict
		}
		now, err := r.Now(call)
		if err != nil {
			return plan.Version{}, err
		}
		var before *plan.Version
		var v plan.Version
		switch c.operation {
		case "create":
			if head.Latest != 0 {
				return v, plan.ErrConflict
			}
			v = plan.Version{PlanCode: c.code, Number: 1, Revision: 1, State: plan.Draft, CreatedAt: now}
			v.Name = c.name
			v.Terms, err = fromTerms(c.terms)
		case "clone":
			if c.expected == 0 || head.Revision != c.expected {
				return v, plan.ErrConflict
			}
			previous, e := r.Get(call, c.code, c.number, true)
			if e != nil {
				return v, e
			}
			if previous.State == plan.Draft {
				return v, plan.ErrImmutable
			}
			v = plan.Version{PlanCode: c.code, Number: head.Latest + 1, Revision: 1, State: plan.Draft, CreatedAt: now, Name: previous.Name, Terms: previous.Terms.Canonical()}
		default:
			current, e := r.Get(call, c.code, c.number, true)
			if e != nil {
				return v, e
			}
			if c.expected == 0 || current.Revision != c.expected || current.Revision == ^uint64(0) {
				return v, plan.ErrConflict
			}
			before = &current
			v = current
			v.Revision++
			switch c.operation {
			case "update":
				if current.State != plan.Draft {
					return v, plan.ErrImmutable
				}
				v.Name = c.name
				v.Terms, err = fromTerms(c.terms)
			case "publish":
				if current.State != plan.Draft {
					return v, plan.ErrImmutable
				}
				v.State = plan.Published
				v.PublishedAt = &now
			case "retire":
				if current.State != plan.Published {
					return v, plan.ErrImmutable
				}
				v.State = plan.Retired
				v.RetiredAt = &now
			default:
				return v, plan.ErrInvalid
			}
		}
		if err != nil {
			return v, err
		}
		if strings.TrimSpace(v.Name) == "" || len(v.Name) > 160 {
			return v, plan.ErrInvalid
		}
		if c.operation != "retire" {
			if err := v.Terms.Validate(catalog); err != nil {
				return v, err
			}
		}
		v.Terms = v.Terms.Canonical()
		v.ContentSHA256 = plan.Hash(v.Name, v.Terms)
		v.ActorID = a
		v.Reason = c.reason
		v.PlanRevision = head.Revision + 1
		if err := r.Save(call, head, before, v); err != nil {
			return v, err
		}
		if err := r.Complete(call, c.operation, c.key, hash, before, v, now); err != nil {
			return v, err
		}
		return v, nil
	})
	if err != nil {
		return nil, exposed(err)
	}
	return dto(out), nil
}
func (s *service) CreatePlanDraft(ctx context.Context, r *v1.CreatePlanDraftRequest) (*v1.PlanVersionDTO, error) {
	if r == nil {
		return nil, exposed(plan.ErrInvalid)
	}
	return s.mutate(ctx, command{operation: "create", key: r.RequestId, code: r.PlanCode, name: r.Name, terms: r.Terms, reason: r.Reason, request: r})
}
func (s *service) CreatePlanVersion(ctx context.Context, r *v1.CreatePlanVersionRequest) (*v1.PlanVersionDTO, error) {
	if r == nil {
		return nil, exposed(plan.ErrInvalid)
	}
	return s.mutate(ctx, command{operation: "clone", key: r.RequestId, code: r.PlanCode, number: r.FromVersion, expected: r.ExpectedPlanRevision, reason: r.Reason, request: r})
}
func (s *service) UpdatePlanDraft(ctx context.Context, r *v1.UpdatePlanDraftRequest) (*v1.PlanVersionDTO, error) {
	if r == nil {
		return nil, exposed(plan.ErrInvalid)
	}
	return s.mutate(ctx, command{operation: "update", key: r.RequestId, code: r.PlanCode, number: r.Version, expected: r.ExpectedRevision, name: r.Name, terms: r.Terms, reason: r.Reason, request: r})
}
func (s *service) PublishPlanVersion(ctx context.Context, r *v1.ChangePlanVersionStateRequest) (*v1.PlanVersionDTO, error) {
	if r == nil {
		return nil, exposed(plan.ErrInvalid)
	}
	return s.mutate(ctx, command{operation: "publish", key: r.RequestId, code: r.PlanCode, number: r.Version, expected: r.ExpectedRevision, reason: r.Reason, request: r})
}
func (s *service) RetirePlanVersion(ctx context.Context, r *v1.ChangePlanVersionStateRequest) (*v1.PlanVersionDTO, error) {
	if r == nil {
		return nil, exposed(plan.ErrInvalid)
	}
	return s.mutate(ctx, command{operation: "retire", key: r.RequestId, code: r.PlanCode, number: r.Version, expected: r.ExpectedRevision, reason: r.Reason, request: r})
}
func (s *service) GetPlanVersion(ctx context.Context, req *v1.GetPlanVersionRequest) (*v1.PlanVersionDTO, error) {
	if _, err := actor(ctx); err != nil {
		return nil, exposed(err)
	}
	if req == nil || !plan.Code(req.PlanCode) || req.Version == 0 {
		return nil, exposed(plan.ErrInvalid)
	}
	v, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.PlanRepositories]) (plan.Version, error) {
		return scope.Repositories().Plans.Get(scope.Context(), req.PlanCode, req.Version, false)
	})
	if err != nil {
		return nil, exposed(err)
	}
	return dto(v), nil
}
func (s *service) ListPlanVersions(ctx context.Context, req *v1.ListPlanVersionsRequest) (*v1.ListPlanVersionsResponse, error) {
	if _, err := actor(ctx); err != nil {
		return nil, exposed(err)
	}
	if req == nil || !plan.Code(req.PlanCode) || req.PageSize > 100 {
		return nil, exposed(plan.ErrInvalid)
	}
	size := int(req.PageSize)
	if size == 0 {
		size = 20
	}
	values, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.PlanRepositories]) ([]plan.Version, error) {
		return scope.Repositories().Plans.List(scope.Context(), req.PlanCode, req.AfterVersion, size+1)
	})
	if err != nil {
		return nil, exposed(err)
	}
	out := &v1.ListPlanVersionsResponse{}
	if len(values) > size {
		values = values[:size]
		out.NextAfterVersion = values[len(values)-1].Number
	}
	for _, v := range values {
		out.Versions = append(out.Versions, dto(v))
	}
	return out, nil
}
func (s *service) CheckPlanEligibility(ctx context.Context, req *v1.CheckPlanEligibilityRequest) (*v1.PlanEligibilityDTO, error) {
	if _, err := actor(ctx); err != nil {
		return nil, exposed(err)
	}
	if req == nil || !plan.Code(req.PlanCode) || req.Version == 0 || !plan.Code(req.SalesScope) {
		return nil, exposed(plan.ErrInvalid)
	}
	catalog, err := s.catalog(ctx)
	if err != nil {
		return nil, exposed(err)
	}
	v, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.PlanRepositories]) (plan.Version, error) {
		return scope.Repositories().Plans.Get(scope.Context(), req.PlanCode, req.Version, true)
	})
	if err != nil {
		return nil, exposed(err)
	}
	reason := v.Eligibility(req.SalesScope, catalog)
	return &v1.PlanEligibilityDTO{Eligible: reason == "ELIGIBLE", Reason: reason, Version: dto(v)}, nil
}
