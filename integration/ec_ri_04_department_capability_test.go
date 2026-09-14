//go:build integration

package integration

import (
	"context"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
)

type b12DepartmentAssignmentCapability struct{}

func (b12DepartmentAssignmentCapability) AssertTenantMemberDepartmentAssignmentAllowed(context.Context, *accessv1.AssertTenantMemberDepartmentAssignmentAllowedRequest) (*accessv1.AssertTenantMemberDepartmentAssignmentAllowedResponse, error) {
	return &accessv1.AssertTenantMemberDepartmentAssignmentAllowedResponse{}, nil
}

func (b12MemberCapabilities) AccessTenantDepartmentManagement() accessapp.TenantMemberLifecycleToAccessTenantDepartmentManagementChildCapability {
	return b12DepartmentAssignmentCapability{}
}
