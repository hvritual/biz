package application

import (
	"context"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
)

type tenantMemberDepartmentCapabilityStub struct{}

func (tenantMemberDepartmentCapabilityStub) AssertTenantMemberDepartmentAssignmentAllowed(context.Context, *accessv1.AssertTenantMemberDepartmentAssignmentAllowedRequest) (*accessv1.AssertTenantMemberDepartmentAssignmentAllowedResponse, error) {
	return &accessv1.AssertTenantMemberDepartmentAssignmentAllowedResponse{}, nil
}

func (tenantMemberCapabilitiesStub) AccessTenantDepartmentManagement() TenantMemberLifecycleToAccessTenantDepartmentManagementChildCapability {
	return tenantMemberDepartmentCapabilityStub{}
}
