package usecase

import (
	"context"
	accessapp "github.com/hvritual/biz/internal/access/application"
	"strings"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/requestscope"
)

func (service *service) AssertTenantActive(ctx context.Context, request *accessv1.AssertTenantActiveRequest) (*accessv1.AssertTenantActiveResponse, error) {
	if request == nil || strings.TrimSpace(request.GetTenantId()) == "" {
		return nil, accessapp.ErrInvalidTenantRequest
	}
	tenant, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRepositories]) (domain.Tenant, error) {
		return scope.Repositories().Tenant.Get(scope.Context(), strings.TrimSpace(request.GetTenantId()))
	})
	if err != nil {
		return nil, err
	}
	if tenant.Status != domain.TenantStatusActive {
		return nil, accessapp.ErrTenantNotActive
	}
	return &accessv1.AssertTenantActiveResponse{}, nil
}
