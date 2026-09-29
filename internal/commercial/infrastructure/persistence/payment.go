package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/hvritual/biz/internal/commercial/domain/payment"
	"github.com/hvritual/biz/internal/commercial/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type paymentOrderRow struct {
	ID                    string `gorm:"column:order_id;primaryKey"`
	TenantID              string
	ChangeID              string
	PlanCode              string
	PlanVersion           uint64
	PriceRef              string
	Currency              string
	AmountMinor           uint64
	Provider              string
	State                 string
	Revision              uint64
	ProviderTransactionID *string
	Payload               string
	CreatedAt             time.Time
	ExpiresAt             time.Time
	PaidAt                *time.Time
}

func (paymentOrderRow) TableName() string { return "biz_commercial_payment_orders" }

type paymentOrderRepository struct{ tx *gorm.DB }

var _ ports.PaymentOrderRepository = (*paymentOrderRepository)(nil)

func decodePaymentOrder(row paymentOrderRow) (*payment.Order, error) {
	var value payment.Order
	if json.Unmarshal([]byte(row.Payload), &value) != nil || value.Validate() != nil || value.ID != row.ID || value.TenantID != row.TenantID || value.ChangeID != row.ChangeID || value.PlanCode != row.PlanCode || value.PlanVersion != row.PlanVersion || value.PriceRef != row.PriceRef || value.Currency != row.Currency || value.AmountMinor != row.AmountMinor || value.Provider != row.Provider || value.State != row.State || value.Revision != row.Revision || value.ProviderTransactionID != stringValue(row.ProviderTransactionID) || !value.CreatedAt.Equal(row.CreatedAt) || !value.ExpiresAt.Equal(row.ExpiresAt) || !samePaymentTime(value.PaidAt, row.PaidAt) {
		return nil, payment.ErrInvalid
	}
	return &value, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func samePaymentTime(left, right *time.Time) bool {
	return (left == nil && right == nil) || (left != nil && right != nil && left.Equal(*right))
}

func (r *paymentOrderRepository) Get(ctx context.Context, id string, lock bool) (*payment.Order, error) {
	db := r.tx.WithContext(ctx).Where("order_id=?", id)
	if lock {
		db = db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row paymentOrderRow
	err := db.First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, payment.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodePaymentOrder(row)
}

func (r *paymentOrderRepository) ForChange(ctx context.Context, tenantID, changeID string, lock bool) (*payment.Order, error) {
	db := r.tx.WithContext(ctx).Where("tenant_id=? AND change_id=?", tenantID, changeID)
	if lock {
		db = db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row paymentOrderRow
	err := db.First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, payment.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodePaymentOrder(row)
}

func (r *paymentOrderRepository) Create(ctx context.Context, value payment.Order) error {
	if err := value.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var transactionID *string
	if value.ProviderTransactionID != "" {
		transactionID = &value.ProviderTransactionID
	}
	return r.tx.WithContext(ctx).Create(&paymentOrderRow{ID: value.ID, TenantID: value.TenantID, ChangeID: value.ChangeID, PlanCode: value.PlanCode, PlanVersion: value.PlanVersion, PriceRef: value.PriceRef, Currency: value.Currency, AmountMinor: value.AmountMinor, Provider: value.Provider, State: value.State, Revision: value.Revision, ProviderTransactionID: transactionID, Payload: string(payload), CreatedAt: value.CreatedAt, ExpiresAt: value.ExpiresAt, PaidAt: value.PaidAt}).Error
}

func (r *paymentOrderRepository) Save(ctx context.Context, value payment.Order, expected uint64) error {
	if err := value.Validate(); err != nil || expected == 0 || value.Revision != expected+1 {
		return payment.ErrConflict
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var transactionID *string
	if value.ProviderTransactionID != "" {
		transactionID = &value.ProviderTransactionID
	}
	result := r.tx.WithContext(ctx).Model(&paymentOrderRow{}).Where("order_id=? AND revision=?", value.ID, expected).Updates(map[string]any{"state": value.State, "revision": value.Revision, "provider_transaction_id": transactionID, "payload": string(payload), "paid_at": value.PaidAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return payment.ErrConflict
	}
	return nil
}
