package ports

import (
	"context"

	"github.com/hvritual/biz/internal/deviceops/domain"
)

type ScopedDeviceRepository interface {
	DeviceRepository
	ListVisible(context.Context) ([]domain.Device, error)
	GetVisible(context.Context, string) (domain.Device, error)
}

type ScopedRepositories struct {
	Device ScopedDeviceRepository
	Site   SiteRepository
}
