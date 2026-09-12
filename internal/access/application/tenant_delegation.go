package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/requestscope"
)

var ErrInvalidTenantDelegationRequest = errors.New("access: invalid tenant delegation request")

var delegatedDevicePermissions = map[string]struct{}{
	"device.read":   {},
	"device.update": {},
}

type TenantDelegationManagementService struct {
	repositories requestscope.RepositoryFactory[ports.TenantDelegationRepositories]
	capabilities TenantDelegationManagementCapabilities
}

func NewTenantDelegationManagementService(repositories requestscope.RepositoryFactory[ports.TenantDelegationRepositories], capabilities TenantDelegationManagementCapabilities) (*TenantDelegationManagementService, error) {
	if repositories == nil {
		return nil, errors.New("access: tenant delegation repository factory is required")
	}
	if capabilities == nil || capabilities.AccessTenantLifecycle() == nil || capabilities.DeviceopsDeviceManagement() == nil {
		return nil, errors.New("access: tenant delegation child capabilities are required")
	}
	return &TenantDelegationManagementService{repositories: repositories, capabilities: capabilities}, nil
}

func (service *TenantDelegationManagementService) GrantTenantDeviceDelegation(ctx context.Context, request *accessv1.GrantTenantDeviceDelegationRequest) (*accessv1.TenantDelegationDTO, error) {
	if request == nil {
		return nil, ErrInvalidTenantDelegationRequest
	}
	ownerTenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	granteeTenantID := strings.TrimSpace(request.GetGranteeTenantId())
	deviceID := strings.TrimSpace(request.GetDeviceId())
	if granteeTenantID == "" || deviceID == "" || granteeTenantID == ownerTenantID {
		return nil, ErrInvalidTenantDelegationRequest
	}
	permissions, err := normalizeDelegatedDevicePermissions(request.GetPermissions())
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var expiresAt *time.Time
	if request.GetExpiresAtUnixMs() != 0 {
		value := time.UnixMilli(request.GetExpiresAtUnixMs()).UTC()
		if !value.After(now) {
			return nil, ErrInvalidTenantDelegationRequest
		}
		expiresAt = &value
	}
	if _, err := service.capabilities.AccessTenantLifecycle().AssertTenantActive(ctx, &accessv1.AssertTenantActiveRequest{TenantId: granteeTenantID}); err != nil {
		return nil, err
	}
	if _, err := service.capabilities.DeviceopsDeviceManagement().AssertDeviceOwnedByActorTenant(ctx, &deviceopsv1.AssertDeviceOwnedByActorTenantRequest{DeviceId: deviceID}); err != nil {
		return nil, err
	}
	delegation := domain.NewTenantDelegation(
		newTenantDelegationID(), ownerTenantID, granteeTenantID, domain.TenantDelegationResourceDevice,
		deviceID, permissions, expiresAt, now,
	)
	created, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantDelegationRepositories]) (domain.TenantDelegation, error) {
		return scope.Repositories().Delegation.CreateOrGetActive(scope.Context(), &delegation)
	})
	if err != nil {
		return nil, err
	}
	return tenantDelegationDTO(created), nil
}

func (service *TenantDelegationManagementService) GetTenantDelegation(ctx context.Context, request *accessv1.GetTenantDelegationRequest) (*accessv1.TenantDelegationDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" {
		return nil, ErrInvalidTenantDelegationRequest
	}
	if _, err := trustedTenantID(ctx); err != nil {
		return nil, err
	}
	delegation, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantDelegationRepositories]) (domain.TenantDelegation, error) {
		return scope.Repositories().Delegation.Get(scope.Context(), strings.TrimSpace(request.GetId()))
	})
	if err != nil {
		return nil, err
	}
	return tenantDelegationDTO(delegation), nil
}

func (service *TenantDelegationManagementService) ListTenantDelegations(ctx context.Context, _ *accessv1.ListTenantDelegationsRequest) (*accessv1.ListTenantDelegationsResponse, error) {
	if _, err := trustedTenantID(ctx); err != nil {
		return nil, err
	}
	delegations, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantDelegationRepositories]) ([]domain.TenantDelegation, error) {
		return scope.Repositories().Delegation.List(scope.Context())
	})
	if err != nil {
		return nil, err
	}
	response := &accessv1.ListTenantDelegationsResponse{Delegations: make([]*accessv1.TenantDelegationDTO, 0, len(delegations))}
	for _, delegation := range delegations {
		response.Delegations = append(response.Delegations, tenantDelegationDTO(delegation))
	}
	return response, nil
}

func (service *TenantDelegationManagementService) RevokeTenantDelegation(ctx context.Context, request *accessv1.RevokeTenantDelegationRequest) (*accessv1.TenantDelegationDTO, error) {
	if request == nil || strings.TrimSpace(request.GetId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantDelegationRequest
	}
	if _, err := trustedTenantID(ctx); err != nil {
		return nil, err
	}
	delegation, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantDelegationRepositories]) (domain.TenantDelegation, error) {
		current, err := scope.Repositories().Delegation.Get(scope.Context(), strings.TrimSpace(request.GetId()))
		if err != nil {
			return domain.TenantDelegation{}, err
		}
		if current.Version != request.GetVersion() {
			return domain.TenantDelegation{}, ports.ErrTenantDelegationConflict
		}
		if err := current.Revoke(time.Now().UTC()); err != nil {
			return domain.TenantDelegation{}, err
		}
		if err := scope.Repositories().Delegation.Update(scope.Context(), &current, request.GetVersion()); err != nil {
			return domain.TenantDelegation{}, err
		}
		return current, nil
	})
	if err != nil {
		return nil, err
	}
	return tenantDelegationDTO(delegation), nil
}

func normalizeDelegatedDevicePermissions(values []string) ([]string, error) {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		permission := strings.TrimSpace(value)
		if _, ok := delegatedDevicePermissions[permission]; !ok {
			return nil, ErrInvalidTenantDelegationRequest
		}
		if _, ok := seen[permission]; ok {
			continue
		}
		seen[permission] = struct{}{}
		result = append(result, permission)
	}
	if len(result) == 0 {
		return nil, ErrInvalidTenantDelegationRequest
	}
	sort.Strings(result)
	return result, nil
}

func tenantDelegationDTO(delegation domain.TenantDelegation) *accessv1.TenantDelegationDTO {
	var expiresAtUnixMS int64
	if delegation.ExpiresAt != nil {
		expiresAtUnixMS = delegation.ExpiresAt.UnixMilli()
	}
	return &accessv1.TenantDelegationDTO{
		Id: delegation.ID, OwnerTenantId: delegation.OwnerTenantID, GranteeTenantId: delegation.GranteeTenantID,
		ResourceKind: delegation.ResourceKind, ResourceId: delegation.ResourceID,
		Permissions: append([]string(nil), delegation.Permissions...), Status: tenantDelegationStatusDTO(delegation.Status),
		Version: delegation.Version, ExpiresAtUnixMs: expiresAtUnixMS,
	}
}

func tenantDelegationStatusDTO(status string) accessv1.TenantDelegationStatus {
	switch status {
	case domain.TenantDelegationStatusActive:
		return accessv1.TenantDelegationStatus_TENANT_DELEGATION_STATUS_ACTIVE
	case domain.TenantDelegationStatusRevoked:
		return accessv1.TenantDelegationStatus_TENANT_DELEGATION_STATUS_REVOKED
	default:
		return accessv1.TenantDelegationStatus_TENANT_DELEGATION_STATUS_UNSPECIFIED
	}
}

func newTenantDelegationID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
