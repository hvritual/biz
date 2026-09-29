package ports

import (
	"context"

	"github.com/hvritual/biz/internal/commercial/domain/payment"
)

// PaymentOrderRepository owns only durable commercial payment facts. Provider
// network calls and signature verification are deliberately outside this port.
type PaymentOrderRepository interface {
	Get(context.Context, string, bool) (*payment.Order, error)
	ForChange(context.Context, string, string, bool) (*payment.Order, error)
	Create(context.Context, payment.Order) error
	Save(context.Context, payment.Order, uint64) error
}
