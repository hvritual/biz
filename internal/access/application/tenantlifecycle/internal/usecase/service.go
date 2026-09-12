package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"time"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/requestscope"
)

type service struct {
	repositories requestscope.RepositoryFactory[ports.TenantRepositories]
	capabilities accessapp.TenantLifecycleCapabilities
}

// New constructs only the declared Application; the concrete type stays private.
func New(repositories requestscope.RepositoryFactory[ports.TenantRepositories], capabilities accessapp.TenantLifecycleCapabilities) (accessapp.TenantLifecycleApplication, error) {
	if repositories == nil {
		return nil, errors.New("access: tenant repository factory is required")
	}
	if capabilities == nil {
		return nil, errors.New("access: tenant lifecycle capabilities are required")
	}
	return &service{repositories: repositories, capabilities: capabilities}, nil
}

func (service *service) CreateTenant(ctx context.Context, request *accessv1.CreateTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetName()) == "" || strings.TrimSpace(request.GetOwnerUserId()) == "" || strings.TrimSpace(request.GetOwnerEmail()) == "" || strings.TrimSpace(request.GetRequestId()) == "" || strings.TrimSpace(request.GetSalesScope()) == "" {
		return nil, accessapp.ErrInvalidTenantRequest
	}
	members := service.capabilities.AccessTenantMemberLifecycle()
	roles := service.capabilities.AccessTenantRolePermission()
	subscriptions := service.capabilities.CommercialSubscriptionManagement()
	if members == nil || roles == nil || subscriptions == nil {
		return nil, errors.New("access: tenant bootstrap child capabilities are required")
	}
	return requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRepositories]) (*accessv1.TenantDTO, error) {
		call := scope.Context()
		principal, ok := identity.FromContext(call)
		if !ok || !principal.Authenticated || principal.Subject == "" || principal.TenantID != "" {
			return nil, status.Error(codes.PermissionDenied, "TENANT_CREATION_PLATFORM_CONTEXT_REQUIRED")
		}
		transportKey := execution.IdempotencyKeyFrom(call)
		if transportKey == "" {
			return nil, execution.ErrIdempotencyKeyRequired
		}
		name, ownerID, email := strings.TrimSpace(request.GetName()), strings.TrimSpace(request.GetOwnerUserId()), strings.ToLower(strings.TrimSpace(request.GetOwnerEmail()))
		requestID, salesScope := strings.TrimSpace(request.GetRequestId()), strings.TrimSpace(request.GetSalesScope())
		if len(name) > 200 || len(ownerID) > 64 || len(email) > 320 || len(requestID) > 128 || len(salesScope) > 96 {
			return nil, accessapp.ErrInvalidTenantRequest
		}
		payload, err := json.Marshal([]string{name, ownerID, email, requestID, salesScope})
		if err != nil {
			return nil, err
		}
		fingerprint := sha256.Sum256(payload)
		key := func(kind, id string) string {
			h := sha256.Sum256([]byte(kind + "\x00" + principal.Subject + "\x00" + id))
			return hex.EncodeToString(h[:])
		}
		keys := []string{key("business", requestID), key("transport", transportKey)}
		repo := scope.Repositories().Tenant
		replay, err := repo.ClaimCreation(call, keys, hex.EncodeToString(fingerprint[:]))
		if err != nil {
			return nil, err
		}
		if replay != nil {
			if err := repo.CompleteCreation(call, keys, hex.EncodeToString(fingerprint[:]), *replay); err != nil {
				return nil, err
			}
			return tenantDTO(*replay), nil
		}
		tenant := domain.NewTenant(newTenantID(), name, time.Now().UTC())
		if err := repo.Create(call, &tenant); err != nil {
			return nil, err
		}
		if _, err := members.BootstrapTenantOwnerMember(call, &accessv1.BootstrapTenantOwnerMemberRequest{TenantId: tenant.ID, UserId: ownerID, Email: email}); err != nil {
			return nil, err
		}
		if _, err := roles.BootstrapTenantOwnerRole(call, &accessv1.BootstrapTenantOwnerRoleRequest{TenantId: tenant.ID, UserId: ownerID}); err != nil {
			return nil, err
		}
		if _, err := subscriptions.BootstrapBaseSubscription(call, &commercialv1.BootstrapTenantSubscriptionRequest{RequestId: requestID, TenantId: tenant.ID, SalesScope: salesScope}); err != nil {
			return nil, err
		}
		if err := repo.CompleteCreation(call, keys, hex.EncodeToString(fingerprint[:]), tenant); err != nil {
			return nil, err
		}
		return tenantDTO(tenant), nil
	})
}

func (service *service) GetTenant(ctx context.Context, request *accessv1.GetTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" {
		return nil, accessapp.ErrInvalidTenantRequest
	}
	tenant, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRepositories]) (domain.Tenant, error) {
		return scope.Repositories().Tenant.Get(scope.Context(), strings.TrimSpace(request.GetId()))
	})
	if err != nil {
		return nil, err
	}
	return tenantDTO(tenant), nil
}

func (service *service) ListTenants(ctx context.Context, _ *accessv1.ListTenantsRequest) (*accessv1.ListTenantsResponse, error) {
	tenants, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRepositories]) ([]domain.Tenant, error) {
		return scope.Repositories().Tenant.List(scope.Context())
	})
	if err != nil {
		return nil, err
	}
	response := &accessv1.ListTenantsResponse{Tenants: make([]*accessv1.TenantDTO, 0, len(tenants))}
	for _, tenant := range tenants {
		response.Tenants = append(response.Tenants, tenantDTO(tenant))
	}
	return response, nil
}

func (service *service) UpdateTenant(ctx context.Context, request *accessv1.UpdateTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || strings.TrimSpace(request.GetName()) == "" || request.GetVersion() == 0 {
		return nil, accessapp.ErrInvalidTenantRequest
	}
	return service.mutate(ctx, strings.TrimSpace(request.GetId()), request.GetVersion(), func(tenant *domain.Tenant) error {
		return tenant.Rename(strings.TrimSpace(request.GetName()), time.Now().UTC())
	})
}

func (service *service) ActivateTenant(ctx context.Context, request *accessv1.ActivateTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, accessapp.ErrInvalidTenantRequest
	}
	return service.mutate(ctx, strings.TrimSpace(request.GetId()), request.GetVersion(), func(tenant *domain.Tenant) error {
		return tenant.Activate(time.Now().UTC())
	})
}

func (service *service) SuspendTenant(ctx context.Context, request *accessv1.SuspendTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, accessapp.ErrInvalidTenantRequest
	}
	return service.mutate(ctx, strings.TrimSpace(request.GetId()), request.GetVersion(), func(tenant *domain.Tenant) error {
		return tenant.Suspend(time.Now().UTC())
	})
}

func (service *service) CloseTenant(ctx context.Context, request *accessv1.CloseTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, accessapp.ErrInvalidTenantRequest
	}
	return service.mutate(ctx, strings.TrimSpace(request.GetId()), request.GetVersion(), func(tenant *domain.Tenant) error {
		return tenant.Close(time.Now().UTC())
	})
}

func (service *service) mutate(ctx context.Context, id string, expectedVersion uint64, apply func(*domain.Tenant) error) (*accessv1.TenantDTO, error) {
	tenant, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRepositories]) (domain.Tenant, error) {
		current, err := scope.Repositories().Tenant.Get(scope.Context(), id)
		if err != nil {
			return domain.Tenant{}, err
		}
		if current.Version != expectedVersion {
			return domain.Tenant{}, ports.ErrTenantConflict
		}
		if err := apply(&current); err != nil {
			return domain.Tenant{}, err
		}
		if err := scope.Repositories().Tenant.Update(scope.Context(), &current, expectedVersion); err != nil {
			return domain.Tenant{}, err
		}
		return current, nil
	})
	if err != nil {
		return nil, err
	}
	return tenantDTO(tenant), nil
}

func tenantDTO(tenant domain.Tenant) *accessv1.TenantDTO {
	return &accessv1.TenantDTO{Id: tenant.ID, Name: tenant.Name, Status: tenantStatusDTO(tenant.Status), Version: tenant.Version}
}

func tenantStatusDTO(status string) accessv1.TenantStatus {
	switch status {
	case domain.TenantStatusPending:
		return accessv1.TenantStatus_TENANT_STATUS_PENDING
	case domain.TenantStatusActive:
		return accessv1.TenantStatus_TENANT_STATUS_ACTIVE
	case domain.TenantStatusSuspended:
		return accessv1.TenantStatus_TENANT_STATUS_SUSPENDED
	case domain.TenantStatusClosed:
		return accessv1.TenantStatus_TENANT_STATUS_CLOSED
	default:
		return accessv1.TenantStatus_TENANT_STATUS_UNSPECIFIED
	}
}

func newTenantID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
