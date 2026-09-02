package application

import (
	"context"
	"errors"
	"strings"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/requestscope"
)

var ErrTenantNotActive = errors.New("access: tenant is not active")

func (service *TenantLifecycleService) AssertTenantActive(ctx context.Context, request *accessv1.AssertTenantActiveRequest) (*accessv1.AssertTenantActiveResponse, error) {
	if request == nil || strings.TrimSpace(request.GetTenantId()) == "" {
		return nil, ErrInvalidTenantRequest
	}
	tenant, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantRepositories]) (domain.Tenant, error) {
		return scope.Repositories().Tenant.Get(scope.Context(), strings.TrimSpace(request.GetTenantId()))
	})
	if err != nil {
		return nil, err
	}
	if tenant.Status != domain.TenantStatusActive {
		return nil, ErrTenantNotActive
	}
	return &accessv1.AssertTenantActiveResponse{}, nil
}
