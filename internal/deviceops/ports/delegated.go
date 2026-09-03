package ports

import (
	"context"

	"github.com/hvritual/biz/internal/deviceops/domain"
)

// DelegatedDeviceRepository is the narrow persistence port for an already
// authorized delegated Device operation. Implementations must derive the owner
// tenant and resource identity from trusted server-side context, never from a
// caller-supplied tenant field.
type DelegatedDeviceRepository interface {
	GetAuthorized(context.Context, string) (domain.Device, error)
	UpdateAuthorized(context.Context, *domain.Device, uint64) error
}

type DelegatedRepositories struct {
	Device DelegatedDeviceRepository
}
