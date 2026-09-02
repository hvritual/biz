package application

import (
	"context"
	"strings"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/requestscope"
)

// AssertTenantMemberDeactivationAllowed is the Role/Permission-owned invariant
// used by TenantMemberLifecycle before suspending or removing a member. The
// repository executes the owner-role check inside the caller's root UoW.
func (service *TenantRolePermissionService) AssertTenantMemberDeactivationAllowed(ctx context.Context, request *accessv1.AssertTenantMemberDeactivationAllowedRequest) (*accessv1.AssertTenantMemberDeactivationAllowedResponse, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" {
		return nil, ErrInvalidTenantRoleRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(request.GetUserId())
	if err := requestscope.JoinDo(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRoleRepositories]) error {
		return scope.Repositories().Role.AssertMemberCanDeactivate(scope.Context(), tenantID, userID)
	}); err != nil {
		return nil, err
	}
	return &accessv1.AssertTenantMemberDeactivationAllowedResponse{}, nil
}
