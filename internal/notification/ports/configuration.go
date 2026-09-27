package ports

import (
	"context"

	"github.com/hvritual/biz/internal/notification/domain"
)

// ConfigurationRepository borrows an already-open root transaction. It neither
// starts another UnitOfWork nor commits a caller's transaction. The application
// must resolve current authorization and reference validity before invoking it.
type ConfigurationRepository interface {
	List(context.Context, string, domain.ConfigurationFilter) ([]domain.Configuration, uint64, error)
	Get(context.Context, string, string, bool) (domain.Configuration, error)
	CreateMany(context.Context, []domain.Configuration) error
	Replace(context.Context, domain.Configuration, uint64) error
	Delete(context.Context, string, string, uint64) error
	ClaimReceipt(context.Context, string, string, string, string, string) (*domain.ConfigurationReceipt, error)
	CompleteReceipt(context.Context, string, string, string, string, domain.ConfigurationReceipt) error
}
