package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	commercialapp "github.com/hvritual/biz/internal/commercial/application"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/ports"
	"google.golang.org/protobuf/proto"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
)

type service struct {
	repositories requestscope.RepositoryFactory[ports.EntitlementRepositories]
	capabilities commercialapp.EntitlementManagementCapabilities
	providers    []ports.EntitlementSourceProvider
	snapshots    ports.EntitlementSnapshotReader
	permissions  ports.PermissionVersionReader
}

func New(repositories requestscope.RepositoryFactory[ports.EntitlementRepositories], capabilities commercialapp.EntitlementManagementCapabilities, providers []ports.EntitlementSourceProvider, snapshots ports.EntitlementSnapshotReader, permissions ports.PermissionVersionReader) (commercialapp.EntitlementManagementApplication, error) {
	if snapshots == nil || permissions == nil || repositories == nil || capabilities == nil || capabilities.CommercialModuleCatalog() == nil || capabilities.AccessTenantLifecycle() == nil {
		return nil, errors.New("commercial: entitlement repositories and typed child capabilities required")
	}
	if len(providers) != 0 {
		return nil, errors.New("commercial: source providers must join the versioned snapshot protocol before activation")
	}
	for _, p := range providers {
		if p == nil || (p.Kind() != entitlement.PlanSource && p.Kind() != entitlement.AddonSource) {
			return nil, errors.New("commercial: invalid entitlement provider")
		}
	}
	return &service{repositories: repositories, capabilities: capabilities, providers: append([]ports.EntitlementSourceProvider(nil), providers...), snapshots: snapshots, permissions: permissions}, nil
}

var tokenPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)

func token(s string, max int) bool { return len(s) > 0 && len(s) <= max && tokenPattern.MatchString(s) }
func actor(ctx context.Context, platform bool) (identity.Principal, error) {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.Subject == "" {
		return identity.Principal{}, entitlement.ErrScope
	}
	if platform {
		if p.TenantID != "" {
			return identity.Principal{}, entitlement.ErrScope
		}
	} else if !token(p.TenantID, 64) {
		return identity.Principal{}, entitlement.ErrScope
	}
	return p, nil
}
func (s *service) checkTenant(ctx context.Context, tenant string) error {
	if !token(tenant, 64) {
		return entitlement.ErrInvalid
	}
	// Cross-owner access only via the generated, edge-owned child capability.
	value, err := s.capabilities.AccessTenantLifecycle().GetTenant(ctx, &accessv1.GetTenantRequest{Id: tenant})
	if err != nil {
		return err
	}
	if value == nil || value.Id != tenant {
		return entitlement.ErrScope
	}
	return nil
}
func (s *service) catalog(ctx context.Context) (entitlement.Catalog, error) {
	response, err := s.capabilities.CommercialModuleCatalog().ReadEntitlementCatalog(ctx, &commercialv1.ListModulesRequest{})
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, entitlement.ErrCatalog
	}
	out := make(entitlement.Catalog, 0, len(response.Modules))
	for _, m := range response.Modules {
		if m == nil {
			return nil, entitlement.ErrCatalog
		}
		status := ""
		switch m.TechnicalStatus {
		case commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY:
			status = "ready"
		case commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_NOT_READY:
			status = "not_ready"
		case commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_DISABLED:
			status = "disabled"
		}
		out = append(out, entitlement.ModuleDefinition{Code: m.ModuleCode, TechnicalStatus: status, SalesStatus: m.SalesStatus.String(), Version: m.Version, Capabilities: m.CapabilityCodes, QuotaKeys: m.QuotaSchemaKeys, FieldKeys: m.FieldPolicySchemaKeys, Dependencies: m.Dependencies})
	}
	return out, out.Validate()
}
func fingerprint(actorID, operation string, request proto.Message) (string, error) {
	payload, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(operation+"\x00"+actorID+"\x00"), payload...))
	return hex.EncodeToString(sum[:]), nil
}
func now() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func validWrite(key, reason string) bool {
	return token(key, 128) && strings.TrimSpace(reason) != "" && len(reason) <= 512
}

func (s *service) CreateEntitlementOverride(ctx context.Context, req *commercialv1.CreateEntitlementOverrideRequest) (*commercialv1.EntitlementOverrideReceipt, error) {
	p, err := actor(ctx, true)
	if err != nil {
		return nil, err
	}
	if req == nil || !validWrite(req.RequestId, req.Reason) {
		return nil, entitlement.ErrInvalid
	}
	if err := s.checkTenant(ctx, req.TenantId); err != nil {
		return nil, err
	}
	catalog, err := s.catalog(ctx)
	if err != nil {
		return nil, err
	}
	hash, err := fingerprint(p.Subject, "create", req)
	if err != nil {
		return nil, err
	}
	receipt, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.EntitlementRepositories]) (ports.OverrideReceipt, error) {
		repo := scope.Repositories().Entitlements
		call := scope.Context()
		state, err := repo.Lock(call, req.TenantId)
		if err != nil {
			return ports.OverrideReceipt{}, err
		}
		replay, err := repo.Receipt(call, req.TenantId, req.RequestId, hash)
		if err != nil {
			return ports.OverrideReceipt{}, err
		}
		if replay != nil {
			return *replay, nil
		}
		if state.Version != req.ExpectedVersion {
			return ports.OverrideReceipt{}, entitlement.ErrConflict
		}
		at := now()
		source, err := fromCreate(req, p.Subject, at)
		if err != nil {
			return ports.OverrideReceipt{}, err
		}
		if err := entitlement.ValidateAppend(state.Sources, source, catalog); err != nil {
			return ports.OverrideReceipt{}, err
		}
		if err := repo.Insert(call, source); err != nil {
			return ports.OverrideReceipt{}, err
		}
		if err := repo.Advance(call, source.TenantID, state.Version); err != nil {
			return ports.OverrideReceipt{}, err
		}
		if err := repo.Audit(call, ports.EntitlementAudit{TenantID: source.TenantID, ActorID: p.Subject, Action: "create", Reason: req.Reason, RequestID: req.RequestId, BeforeVersion: state.Version, AfterVersion: state.Version + 1, After: source, At: at}); err != nil {
			return ports.OverrideReceipt{}, err
		}
		result := ports.OverrideReceipt{Source: source, SourceVersion: state.Version + 1}
		if err := repo.SaveReceipt(call, source.TenantID, req.RequestId, hash, result); err != nil {
			return ports.OverrideReceipt{}, err
		}
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	return receiptDTO(receipt), nil
}
func (s *service) RevokeEntitlementOverride(ctx context.Context, req *commercialv1.RevokeEntitlementOverrideRequest) (*commercialv1.EntitlementOverrideReceipt, error) {
	p, err := actor(ctx, true)
	if err != nil {
		return nil, err
	}
	if req == nil || !validWrite(req.RequestId, req.Reason) || !token(req.Id, 128) {
		return nil, entitlement.ErrInvalid
	}
	if err := s.checkTenant(ctx, req.TenantId); err != nil {
		return nil, err
	}
	hash, err := fingerprint(p.Subject, "revoke", req)
	if err != nil {
		return nil, err
	}
	receipt, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.EntitlementRepositories]) (ports.OverrideReceipt, error) {
		repo := scope.Repositories().Entitlements
		call := scope.Context()
		state, err := repo.Lock(call, req.TenantId)
		if err != nil {
			return ports.OverrideReceipt{}, err
		}
		replay, err := repo.Receipt(call, req.TenantId, req.RequestId, hash)
		if err != nil {
			return ports.OverrideReceipt{}, err
		}
		if replay != nil {
			return *replay, nil
		}
		if state.Version != req.ExpectedVersion {
			return ports.OverrideReceipt{}, entitlement.ErrConflict
		}
		for _, current := range state.Sources {
			if current.ID != req.Id || current.SourceKind != entitlement.OverrideSource {
				continue
			}
			if current.RevokedAt != nil {
				return ports.OverrideReceipt{}, entitlement.ErrRevoked
			}
			before := current
			at := now()
			current.RevokedAt = &at
			current.Version++
			if err := repo.Revoke(call, current, before.Version); err != nil {
				return ports.OverrideReceipt{}, err
			}
			if err := repo.Advance(call, req.TenantId, state.Version); err != nil {
				return ports.OverrideReceipt{}, err
			}
			if err := repo.Audit(call, ports.EntitlementAudit{TenantID: req.TenantId, ActorID: p.Subject, Action: "revoke", Reason: req.Reason, RequestID: req.RequestId, BeforeVersion: state.Version, AfterVersion: state.Version + 1, Before: &before, After: current, At: at}); err != nil {
				return ports.OverrideReceipt{}, err
			}
			result := ports.OverrideReceipt{Source: current, SourceVersion: state.Version + 1}
			if err := repo.SaveReceipt(call, req.TenantId, req.RequestId, hash, result); err != nil {
				return ports.OverrideReceipt{}, err
			}
			return result, nil
		}
		return ports.OverrideReceipt{}, entitlement.ErrNotFound
	})
	if err != nil {
		return nil, err
	}
	return receiptDTO(receipt), nil
}
func (s *service) ListEntitlementOverrides(ctx context.Context, req *commercialv1.ListEntitlementOverridesRequest) (*commercialv1.ListEntitlementOverridesResponse, error) {
	if _, err := actor(ctx, true); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, entitlement.ErrInvalid
	}
	if err := s.checkTenant(ctx, req.TenantId); err != nil {
		return nil, err
	}
	state, err := s.read(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	response := &commercialv1.ListEntitlementOverridesResponse{SourceVersion: state.Version, Sources: []*commercialv1.EntitlementOverrideDTO{}}
	for _, source := range state.Sources {
		if source.SourceKind != entitlement.OverrideSource {
			continue
		}
		response.Sources = append(response.Sources, sourceDTO(source))
	}
	return response, nil
}
func (s *service) ExplainEntitlements(ctx context.Context, req *commercialv1.ExplainEntitlementsRequest) (*commercialv1.EntitlementView, error) {
	if _, err := actor(ctx, true); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, entitlement.ErrInvalid
	}
	return s.resolve(ctx, req.TenantId, req.CapabilityCodes, false)
}
func (s *service) GetMyEntitlements(ctx context.Context, req *commercialv1.GetMyEntitlementsRequest) (*commercialv1.EntitlementView, error) {
	p, err := actor(ctx, false)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, entitlement.ErrInvalid
	}
	return s.resolve(ctx, p.TenantID, req.CapabilityCodes, true)
}
func (s *service) read(ctx context.Context, tenant string) (ports.EntitlementState, error) {
	return requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.EntitlementRepositories]) (ports.EntitlementState, error) {
		return scope.Repositories().Entitlements.Read(scope.Context(), tenant)
	})
}
func (s *service) resolve(ctx context.Context, tenant string, requested []string, redact bool) (*commercialv1.EntitlementView, error) {
	if len(requested) > 128 {
		return nil, entitlement.ErrInvalid
	}
	// Tenant reads already have an active tenant/member resolved by Access
	// authentication. Do not call the platform tenant.get operation from them.
	if !redact {
		if err := s.checkTenant(ctx, tenant); err != nil {
			return nil, err
		}
	}
	// Preserve the declared typed catalog dependency before the current read.
	if _, err := s.catalog(ctx); err != nil {
		return nil, err
	}
	result, err := s.snapshots.ReadSnapshot(ctx, tenant, requested)
	if err != nil {
		return nil, err
	}
	if redact {
		result.PermissionVersion, err = s.permissions.PermissionVersion(ctx)
		if err != nil {
			return nil, err
		}
		p, _ := identity.FromContext(ctx)
		result.PermissionSubject = p.Subject
	}
	return resultDTO(result, redact), nil
}
