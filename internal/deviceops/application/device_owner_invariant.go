package application

import (
	"context"
	"strings"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/deviceops/domain"
	"github.com/hvritual/biz/internal/deviceops/ports"
	"yunka.io/framework/requestscope"
)

func (service *Service) AssertDeviceOwnedByActorTenant(ctx context.Context, request *deviceopsv1.AssertDeviceOwnedByActorTenantRequest) (*deviceopsv1.AssertDeviceOwnedByActorTenantResponse, error) {
	if request == nil || strings.TrimSpace(request.GetDeviceId()) == "" {
		return nil, ErrInvalid
	}
	if _, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.ScopedRepositories]) (domain.Device, error) {
		return scope.Repositories().Device.Get(scope.Context(), strings.TrimSpace(request.GetDeviceId()))
	}); err != nil {
		return nil, err
	}
	return &deviceopsv1.AssertDeviceOwnedByActorTenantResponse{}, nil
}
