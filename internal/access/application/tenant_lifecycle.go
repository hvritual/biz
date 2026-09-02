package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"github.com/hvritual/yunka.io/framework/requestscope"
)

var ErrInvalidTenantRequest = errors.New("access: invalid tenant request")

type TenantLifecycleService struct {
	repositories requestscope.RepositoryFactory[ports.TenantRepositories]
	capabilities TenantLifecycleCapabilities
}

func NewTenantLifecycleService(repositories requestscope.RepositoryFactory[ports.TenantRepositories], capabilities TenantLifecycleCapabilities) (*TenantLifecycleService, error) {
	if repositories == nil {
		return nil, errors.New("access: tenant repository factory is required")
	}
	if capabilities == nil {
		return nil, errors.New("access: tenant lifecycle capabilities are required")
	}
	return &TenantLifecycleService{repositories: repositories, capabilities: capabilities}, nil
}

func (service *TenantLifecycleService) CreateTenant(ctx context.Context, request *accessv1.CreateTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetName()) == "" || strings.TrimSpace(request.GetOwnerUserId()) == "" || strings.TrimSpace(request.GetOwnerEmail()) == "" {
		return nil, ErrInvalidTenantRequest
	}
	members := service.capabilities.AccessTenantMemberLifecycle()
	roles := service.capabilities.AccessTenantRolePermission()
	if members == nil || roles == nil {
		return nil, errors.New("access: tenant bootstrap child capabilities are required")
	}
	now := time.Now().UTC()
	tenant := domain.NewTenant(newTenantID(), strings.TrimSpace(request.GetName()), now)
	if err := requestscope.JoinDo(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRepositories]) error {
		return scope.Repositories().Tenant.Create(scope.Context(), &tenant)
	}); err != nil {
		return nil, err
	}
	ownerUserID := strings.TrimSpace(request.GetOwnerUserId())
	ownerEmail := strings.TrimSpace(strings.ToLower(request.GetOwnerEmail()))
	if _, err := members.BootstrapTenantOwnerMember(ctx, &accessv1.BootstrapTenantOwnerMemberRequest{
		TenantId: tenant.ID,
		UserId:   ownerUserID,
		Email:    ownerEmail,
	}); err != nil {
		return nil, err
	}
	if _, err := roles.BootstrapTenantOwnerRole(ctx, &accessv1.BootstrapTenantOwnerRoleRequest{
		TenantId: tenant.ID,
		UserId:   ownerUserID,
	}); err != nil {
		return nil, err
	}
	return tenantDTO(tenant), nil
}

func (service *TenantLifecycleService) GetTenant(ctx context.Context, request *accessv1.GetTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" {
		return nil, ErrInvalidTenantRequest
	}
	tenant, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRepositories]) (domain.Tenant, error) {
		return scope.Repositories().Tenant.Get(scope.Context(), strings.TrimSpace(request.GetId()))
	})
	if err != nil {
		return nil, err
	}
	return tenantDTO(tenant), nil
}

func (service *TenantLifecycleService) ListTenants(ctx context.Context, _ *accessv1.ListTenantsRequest) (*accessv1.ListTenantsResponse, error) {
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

func (service *TenantLifecycleService) UpdateTenant(ctx context.Context, request *accessv1.UpdateTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || strings.TrimSpace(request.GetName()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantRequest
	}
	return service.mutate(ctx, strings.TrimSpace(request.GetId()), request.GetVersion(), func(tenant *domain.Tenant) error {
		return tenant.Rename(strings.TrimSpace(request.GetName()), time.Now().UTC())
	})
}

func (service *TenantLifecycleService) ActivateTenant(ctx context.Context, request *accessv1.ActivateTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantRequest
	}
	return service.mutate(ctx, strings.TrimSpace(request.GetId()), request.GetVersion(), func(tenant *domain.Tenant) error {
		return tenant.Activate(time.Now().UTC())
	})
}

func (service *TenantLifecycleService) SuspendTenant(ctx context.Context, request *accessv1.SuspendTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantRequest
	}
	return service.mutate(ctx, strings.TrimSpace(request.GetId()), request.GetVersion(), func(tenant *domain.Tenant) error {
		return tenant.Suspend(time.Now().UTC())
	})
}

func (service *TenantLifecycleService) CloseTenant(ctx context.Context, request *accessv1.CloseTenantRequest) (*accessv1.TenantDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantRequest
	}
	return service.mutate(ctx, strings.TrimSpace(request.GetId()), request.GetVersion(), func(tenant *domain.Tenant) error {
		return tenant.Close(time.Now().UTC())
	})
}

func (service *TenantLifecycleService) mutate(ctx context.Context, id string, expectedVersion uint64, apply func(*domain.Tenant) error) (*accessv1.TenantDTO, error) {
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
