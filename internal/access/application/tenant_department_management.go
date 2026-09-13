package application

import (
	"context"
	"errors"
	"strings"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/requestscope"
)

var ErrInvalidTenantDepartmentRequest = errors.New("access: invalid tenant department request")

type TenantDepartmentManagementService struct {
	repositories requestscope.RepositoryFactory[ports.TenantDepartmentRepositories]
}

func NewTenantDepartmentManagementService(repositories requestscope.RepositoryFactory[ports.TenantDepartmentRepositories]) (*TenantDepartmentManagementService, error) {
	if repositories == nil {
		return nil, errors.New("access: tenant department repository factory is required")
	}
	return &TenantDepartmentManagementService{repositories: repositories}, nil
}

func (service *TenantDepartmentManagementService) CreateTenantDepartment(ctx context.Context, request *accessv1.CreateTenantDepartmentRequest) (*accessv1.TenantDepartmentDTO, error) {
	if request == nil || strings.TrimSpace(request.GetName()) == "" {
		return nil, ErrInvalidTenantDepartmentRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	department, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantDepartmentRepositories]) (domain.Department, error) {
		repository := scope.Repositories().Department
		id := newMemberUserID()
		if err := repository.ValidateParent(scope.Context(), tenantID, id, request.GetParentId()); err != nil {
			return domain.Department{}, err
		}
		if err := repository.ValidateLeader(scope.Context(), tenantID, request.GetLeaderUserId()); err != nil {
			return domain.Department{}, err
		}
		value, err := domain.NewDepartment(id, tenantID, request.GetName(), request.GetParentId(), request.GetLeaderUserId(), request.GetEmail(), request.GetPhone(), request.GetSort(), time.Now().UTC())
		if err != nil {
			return domain.Department{}, err
		}
		if err := repository.Create(scope.Context(), &value); err != nil {
			return domain.Department{}, err
		}
		return value, nil
	})
	if err != nil {
		return nil, err
	}
	return tenantDepartmentDTO(department), nil
}

func (service *TenantDepartmentManagementService) GetTenantDepartment(ctx context.Context, request *accessv1.GetTenantDepartmentRequest) (*accessv1.TenantDepartmentDTO, error) {
	if request == nil || strings.TrimSpace(request.GetDepartmentId()) == "" {
		return nil, ErrInvalidTenantDepartmentRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	department, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantDepartmentRepositories]) (domain.Department, error) {
		return scope.Repositories().Department.Get(scope.Context(), tenantID, strings.TrimSpace(request.GetDepartmentId()))
	})
	if err != nil {
		return nil, err
	}
	return tenantDepartmentDTO(department), nil
}

func (service *TenantDepartmentManagementService) ListTenantDepartments(ctx context.Context, _ *accessv1.ListTenantDepartmentsRequest) (*accessv1.ListTenantDepartmentsResponse, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	departments, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantDepartmentRepositories]) ([]domain.Department, error) {
		return scope.Repositories().Department.List(scope.Context(), tenantID)
	})
	if err != nil {
		return nil, err
	}
	response := &accessv1.ListTenantDepartmentsResponse{Departments: make([]*accessv1.TenantDepartmentDTO, 0, len(departments))}
	for _, department := range departments {
		response.Departments = append(response.Departments, tenantDepartmentDTO(department))
	}
	return response, nil
}

func (service *TenantDepartmentManagementService) UpdateTenantDepartment(ctx context.Context, request *accessv1.UpdateTenantDepartmentRequest) (*accessv1.TenantDepartmentDTO, error) {
	if request == nil || strings.TrimSpace(request.GetDepartmentId()) == "" || strings.TrimSpace(request.GetName()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantDepartmentRequest
	}
	return service.mutate(ctx, request.GetDepartmentId(), request.GetVersion(), func(scope *requestscope.View[ports.TenantDepartmentRepositories], department *domain.Department) error {
		repository := scope.Repositories().Department
		if err := repository.ValidateParent(scope.Context(), department.TenantID, department.ID, request.GetParentId()); err != nil {
			return err
		}
		if err := repository.ValidateLeader(scope.Context(), department.TenantID, request.GetLeaderUserId()); err != nil {
			return err
		}
		return department.Update(request.GetName(), request.GetParentId(), request.GetLeaderUserId(), request.GetEmail(), request.GetPhone(), request.GetSort(), time.Now().UTC())
	})
}

func (service *TenantDepartmentManagementService) EnableTenantDepartment(ctx context.Context, request *accessv1.EnableTenantDepartmentRequest) (*accessv1.TenantDepartmentDTO, error) {
	if request == nil || strings.TrimSpace(request.GetDepartmentId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantDepartmentRequest
	}
	return service.mutate(ctx, request.GetDepartmentId(), request.GetVersion(), func(_ *requestscope.View[ports.TenantDepartmentRepositories], department *domain.Department) error {
		return department.Enable(time.Now().UTC())
	})
}

func (service *TenantDepartmentManagementService) DisableTenantDepartment(ctx context.Context, request *accessv1.DisableTenantDepartmentRequest) (*accessv1.TenantDepartmentDTO, error) {
	if request == nil || strings.TrimSpace(request.GetDepartmentId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantDepartmentRequest
	}
	return service.mutate(ctx, request.GetDepartmentId(), request.GetVersion(), func(_ *requestscope.View[ports.TenantDepartmentRepositories], department *domain.Department) error {
		return department.Disable(time.Now().UTC())
	})
}

func (service *TenantDepartmentManagementService) AssertTenantMemberDepartmentAssignmentAllowed(ctx context.Context, request *accessv1.AssertTenantMemberDepartmentAssignmentAllowedRequest) (*accessv1.AssertTenantMemberDepartmentAssignmentAllowedResponse, error) {
	if request == nil {
		return nil, ErrInvalidTenantDepartmentRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	_, err = requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantDepartmentRepositories]) (struct{}, error) {
		return struct{}{}, scope.Repositories().Department.AssertMemberAssignmentAllowed(scope.Context(), tenantID, request.GetDepartmentId())
	})
	if err != nil {
		return nil, err
	}
	return &accessv1.AssertTenantMemberDepartmentAssignmentAllowedResponse{}, nil
}

func (service *TenantDepartmentManagementService) mutate(ctx context.Context, departmentID string, expectedVersion uint64, apply func(*requestscope.View[ports.TenantDepartmentRepositories], *domain.Department) error) (*accessv1.TenantDepartmentDTO, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	department, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantDepartmentRepositories]) (domain.Department, error) {
		repository := scope.Repositories().Department
		current, err := repository.Get(scope.Context(), tenantID, strings.TrimSpace(departmentID))
		if err != nil {
			return domain.Department{}, err
		}
		if current.Version != expectedVersion {
			return domain.Department{}, ports.ErrTenantDepartmentConflict
		}
		if err := apply(scope, &current); err != nil {
			return domain.Department{}, err
		}
		if err := repository.Update(scope.Context(), &current, expectedVersion); err != nil {
			return domain.Department{}, err
		}
		return current, nil
	})
	if err != nil {
		return nil, err
	}
	return tenantDepartmentDTO(department), nil
}

func tenantDepartmentDTO(department domain.Department) *accessv1.TenantDepartmentDTO {
	return &accessv1.TenantDepartmentDTO{DepartmentId: department.ID, Name: department.Name, ParentId: department.ParentID, LeaderUserId: department.LeaderUserID, Email: department.Email, Phone: department.Phone, Status: tenantDepartmentStatusDTO(department.Status), Sort: department.Sort, Version: department.Version}
}

func tenantDepartmentStatusDTO(value string) accessv1.TenantDepartmentStatus {
	switch value {
	case domain.TenantDepartmentStatusActive:
		return accessv1.TenantDepartmentStatus_TENANT_DEPARTMENT_STATUS_ACTIVE
	case domain.TenantDepartmentStatusDisabled:
		return accessv1.TenantDepartmentStatus_TENANT_DEPARTMENT_STATUS_DISABLED
	default:
		return accessv1.TenantDepartmentStatus_TENANT_DEPARTMENT_STATUS_UNSPECIFIED
	}
}
