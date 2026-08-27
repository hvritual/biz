package ports

import (
	"context"

	"github.com/hvritual/biz/internal/deviceops/domain"
)

type DeviceScope struct {
	All     bool
	Self    bool
	Sites   bool
	UserID  string
	SiteIDs []string
}

func (scope DeviceScope) AllowsSite(siteID string) bool {
	if scope.All {
		return true
	}
	if !scope.Sites {
		return false
	}
	for _, allowed := range scope.SiteIDs {
		if allowed == siteID {
			return true
		}
	}
	return false
}

type ScopedDeviceRepository interface {
	DeviceRepository
	ListScoped(context.Context, DeviceScope) ([]domain.Device, error)
	GetScoped(context.Context, DeviceScope, string) (domain.Device, error)
}

type ScopedRepositories struct {
	Device ScopedDeviceRepository
	Site   SiteRepository
}
