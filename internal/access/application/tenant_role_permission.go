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
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/requestscope"
)

var ErrInvalidTenantRoleRequest = errors.New("access: invalid tenant role request")

type TenantRolePermissionService struct {
	repositories requestscope.RepositoryFactory[ports.TenantRoleRepositories]
}

func NewTenantRolePermissionService(repositories requestscope.RepositoryFactory[ports.TenantRoleRepositories]) (*TenantRolePermissionService, error) {
	if repositories == nil {
		return nil, errors.New("access: tenant role repository factory is required")
	}
	return &TenantRolePermissionService{repositories: repositories}, nil
}

func (service *TenantRolePermissionService) BootstrapTenantOwnerRole(ctx context.Context, request *accessv1.BootstrapTenantOwnerRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if request == nil || strings.TrimSpace(request.GetTenantId()) == "" || strings.TrimSpace(request.GetUserId()) == "" {
		return nil, ErrInvalidTenantRoleRequest
	}
	role, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRoleRepositories]) (domain.Role, error) {
		return scope.Repositories().Role.BootstrapOwner(scope.Context(), strings.TrimSpace(request.GetTenantId()), strings.TrimSpace(request.GetUserId()), time.Now().UTC())
	})
	if err != nil {
		return nil, err
	}
	return tenantRoleDTO(role), nil
}

func (service *TenantRolePermissionService) CreateTenantRole(ctx context.Context, request *accessv1.CreateTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if request == nil {
		return nil, ErrInvalidTenantRoleRequest
	}
	name := strings.TrimSpace(request.GetName())
	if name == "" || name == domain.TenantOwnerRoleName {
		return nil, ErrInvalidTenantRoleRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	role := domain.NewRole(newTenantRoleID(), tenantID, name, time.Now().UTC())
	err = requestscope.JoinDo(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRoleRepositories]) error {
		return scope.Repositories().Role.Create(scope.Context(), &role)
	})
	if err != nil {
		return nil, err
	}
	return tenantRoleDTO(role), nil
}

func (service *TenantRolePermissionService) GetTenantRole(ctx context.Context, request *accessv1.GetTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if request == nil || strings.TrimSpace(request.GetRoleId()) == "" {
		return nil, ErrInvalidTenantRoleRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	role, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRoleRepositories]) (domain.Role, error) {
		return scope.Repositories().Role.Get(scope.Context(), tenantID, strings.TrimSpace(request.GetRoleId()))
	})
	if err != nil {
		return nil, err
	}
	return tenantRoleDTO(role), nil
}

func (service *TenantRolePermissionService) ListTenantRoles(ctx context.Context, _ *accessv1.ListTenantRolesRequest) (*accessv1.ListTenantRolesResponse, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRoleRepositories]) ([]domain.Role, error) {
		return scope.Repositories().Role.List(scope.Context(), tenantID)
	})
	if err != nil {
		return nil, err
	}
	response := &accessv1.ListTenantRolesResponse{Roles: make([]*accessv1.TenantRoleDTO, 0, len(roles))}
	for _, role := range roles {
		response.Roles = append(response.Roles, tenantRoleDTO(role))
	}
	return response, nil
}

func (service *TenantRolePermissionService) UpdateTenantRole(ctx context.Context, request *accessv1.UpdateTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if request == nil || strings.TrimSpace(request.GetRoleId()) == "" || strings.TrimSpace(request.GetName()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantRoleRequest
	}
	return service.mutateRole(ctx, strings.TrimSpace(request.GetRoleId()), request.GetVersion(), func(role *domain.Role) error {
		return role.Rename(strings.TrimSpace(request.GetName()), time.Now().UTC())
	})
}

func (service *TenantRolePermissionService) DisableTenantRole(ctx context.Context, request *accessv1.DisableTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if request == nil || strings.TrimSpace(request.GetRoleId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantRoleRequest
	}
	return service.mutateRole(ctx, strings.TrimSpace(request.GetRoleId()), request.GetVersion(), func(role *domain.Role) error {
		return role.Disable(time.Now().UTC())
	})
}

func (service *TenantRolePermissionService) EnableTenantRole(ctx context.Context, request *accessv1.EnableTenantRoleRequest) (*accessv1.TenantRoleDTO, error) {
	if request == nil || strings.TrimSpace(request.GetRoleId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantRoleRequest
	}
	return service.mutateRole(ctx, strings.TrimSpace(request.GetRoleId()), request.GetVersion(), func(role *domain.Role) error {
		return role.Enable(time.Now().UTC())
	})
}

func (service *TenantRolePermissionService) SetTenantRolePermissions(ctx context.Context, request *accessv1.SetTenantRolePermissionsRequest) (*accessv1.TenantRoleDTO, error) {
	if request == nil || strings.TrimSpace(request.GetRoleId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantRoleRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	grants, err := permissionGrantInputs(tenantID, strings.TrimSpace(request.GetRoleId()), request.GetPermissions())
	if err != nil {
		return nil, err
	}
	role, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRoleRepositories]) (domain.Role, error) {
		current, err := scope.Repositories().Role.Get(scope.Context(), tenantID, strings.TrimSpace(request.GetRoleId()))
		if err != nil {
			return domain.Role{}, err
		}
		if current.Version != request.GetVersion() {
			return domain.Role{}, ports.ErrTenantRoleConflict
		}
		if err := current.ReplacePermissions(grants, time.Now().UTC()); err != nil {
			return domain.Role{}, err
		}
		if err := scope.Repositories().Role.ReplacePermissions(scope.Context(), &current, request.GetVersion()); err != nil {
			return domain.Role{}, err
		}
		return current, nil
	})
	if err != nil {
		return nil, err
	}
	return tenantRoleDTO(role), nil
}

func (service *TenantRolePermissionService) AssignTenantRoleMember(ctx context.Context, request *accessv1.AssignTenantRoleMemberRequest) (*accessv1.TenantRoleDTO, error) {
	if request == nil || strings.TrimSpace(request.GetRoleId()) == "" || strings.TrimSpace(request.GetUserId()) == "" {
		return nil, ErrInvalidTenantRoleRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	role, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRoleRepositories]) (domain.Role, error) {
		return scope.Repositories().Role.AssignMember(scope.Context(), tenantID, strings.TrimSpace(request.GetRoleId()), strings.TrimSpace(request.GetUserId()))
	})
	if err != nil {
		return nil, err
	}
	return tenantRoleDTO(role), nil
}

func (service *TenantRolePermissionService) RevokeTenantRoleMember(ctx context.Context, request *accessv1.RevokeTenantRoleMemberRequest) (*accessv1.TenantRoleDTO, error) {
	if request == nil || strings.TrimSpace(request.GetRoleId()) == "" || strings.TrimSpace(request.GetUserId()) == "" {
		return nil, ErrInvalidTenantRoleRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	role, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRoleRepositories]) (domain.Role, error) {
		return scope.Repositories().Role.RevokeMember(scope.Context(), tenantID, strings.TrimSpace(request.GetRoleId()), strings.TrimSpace(request.GetUserId()))
	})
	if err != nil {
		return nil, err
	}
	return tenantRoleDTO(role), nil
}

func (service *TenantRolePermissionService) mutateRole(ctx context.Context, roleID string, expectedVersion uint64, apply func(*domain.Role) error) (*accessv1.TenantRoleDTO, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	role, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRoleRepositories]) (domain.Role, error) {
		current, err := scope.Repositories().Role.Get(scope.Context(), tenantID, roleID)
		if err != nil {
			return domain.Role{}, err
		}
		if current.Version != expectedVersion {
			return domain.Role{}, ports.ErrTenantRoleConflict
		}
		if err := apply(&current); err != nil {
			return domain.Role{}, err
		}
		if err := scope.Repositories().Role.Update(scope.Context(), &current, expectedVersion); err != nil {
			return domain.Role{}, err
		}
		return current, nil
	})
	if err != nil {
		return nil, err
	}
	return tenantRoleDTO(role), nil
}

func permissionGrantInputs(tenantID, roleID string, inputs []*accessv1.PermissionGrantInput) ([]domain.PermissionGrant, error) {
	seen := map[string]struct{}{}
	grants := make([]domain.PermissionGrant, 0, len(inputs))
	for _, input := range inputs {
		if input == nil {
			return nil, ErrInvalidTenantRoleRequest
		}
		permission := strings.TrimSpace(input.GetPermission())
		scope, ok := dataScopeDomain(input.GetScope())
		if permission == "" || !ok {
			return nil, ErrInvalidTenantRoleRequest
		}
		if _, exists := seen[permission]; exists {
			return nil, ErrInvalidTenantRoleRequest
		}
		seen[permission] = struct{}{}
		grants = append(grants, domain.PermissionGrant{TenantID: tenantID, RoleID: roleID, Permission: permission, Scope: scope})
	}
	sort.Slice(grants, func(i, j int) bool { return grants[i].Permission < grants[j].Permission })
	return grants, nil
}

func tenantRoleDTO(role domain.Role) *accessv1.TenantRoleDTO {
	result := &accessv1.TenantRoleDTO{Id: role.ID, Name: role.Name, Status: tenantRoleStatusDTO(role.Status), Version: role.Version}
	result.Permissions = make([]*accessv1.PermissionGrantDTO, 0, len(role.Permissions))
	for _, grant := range role.Permissions {
		result.Permissions = append(result.Permissions, &accessv1.PermissionGrantDTO{Permission: grant.Permission, Scope: dataScopeDTO(grant.Scope)})
	}
	return result
}

func tenantRoleStatusDTO(status string) accessv1.TenantRoleStatus {
	switch status {
	case domain.TenantRoleStatusActive:
		return accessv1.TenantRoleStatus_TENANT_ROLE_STATUS_ACTIVE
	case domain.TenantRoleStatusDisabled:
		return accessv1.TenantRoleStatus_TENANT_ROLE_STATUS_DISABLED
	default:
		return accessv1.TenantRoleStatus_TENANT_ROLE_STATUS_UNSPECIFIED
	}
}

func dataScopeDomain(scope accessv1.DataScope) (domain.DataScope, bool) {
	switch scope {
	case accessv1.DataScope_DATA_SCOPE_NONE:
		return domain.DataScopeNone, true
	case accessv1.DataScope_DATA_SCOPE_SELF:
		return domain.DataScopeSelf, true
	case accessv1.DataScope_DATA_SCOPE_SITES:
		return domain.DataScopeSites, true
	case accessv1.DataScope_DATA_SCOPE_ALL:
		return domain.DataScopeAll, true
	default:
		return "", false
	}
}

func dataScopeDTO(scope domain.DataScope) accessv1.DataScope {
	switch scope {
	case domain.DataScopeNone:
		return accessv1.DataScope_DATA_SCOPE_NONE
	case domain.DataScopeSelf:
		return accessv1.DataScope_DATA_SCOPE_SELF
	case domain.DataScopeSites:
		return accessv1.DataScope_DATA_SCOPE_SITES
	case domain.DataScopeAll:
		return accessv1.DataScope_DATA_SCOPE_ALL
	default:
		return accessv1.DataScope_DATA_SCOPE_UNSPECIFIED
	}
}

func newTenantRoleID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
