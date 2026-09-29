// Package payment owns the immutable commercial facts that a provider callback
// must match before a paid subscription change may be activated.
package payment

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	WeChatNative = "WECHAT_NATIVE"
	AlipayPage   = "ALIPAY_PAGE"

	Pending   = "PENDING"
	Paid      = "PAID"
	Cancelled = "CANCELLED"
	Expired   = "EXPIRED"
)

var (
	ErrInvalid  = errors.New("PAYMENT_ORDER_INVALID")
	ErrConflict = errors.New("PAYMENT_ORDER_CONFLICT")
	ErrNotFound = errors.New("PAYMENT_ORDER_NOT_FOUND")
)

var (
	key      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	currency = regexp.MustCompile(`^[A-Z]{3}$`)
)

type Order struct {
	ID                    string     `json:"id"`
	TenantID              string     `json:"tenant_id"`
	ChangeID              string     `json:"change_id"`
	PlanCode              string     `json:"plan_code"`
	PlanVersion           uint64     `json:"plan_version"`
	PriceRef              string     `json:"price_ref"`
	Currency              string     `json:"currency"`
	AmountMinor           uint64     `json:"amount_minor"`
	Provider              string     `json:"provider"`
	State                 string     `json:"state"`
	Revision              uint64     `json:"revision"`
	ProviderTransactionID string     `json:"provider_transaction_id,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	ExpiresAt             time.Time  `json:"expires_at"`
	PaidAt                *time.Time `json:"paid_at,omitempty"`
}

type ProviderReceipt struct {
	Provider              string    `json:"provider"`
	ProviderTransactionID string    `json:"provider_transaction_id"`
	Currency              string    `json:"currency"`
	AmountMinor           uint64    `json:"amount_minor"`
	PaidAt                time.Time `json:"paid_at"`
}

func validProvider(v string) bool { return v == WeChatNative || v == AlipayPage }

func NewOrder(id, tenantID, changeID, planCode string, planVersion uint64, priceRef, currencyCode string, amountMinor uint64, provider string, createdAt, expiresAt time.Time) (Order, error) {
	o := Order{ID: id, TenantID: tenantID, ChangeID: changeID, PlanCode: planCode, PlanVersion: planVersion, PriceRef: priceRef, Currency: currencyCode, AmountMinor: amountMinor, Provider: provider, State: Pending, Revision: 1, CreatedAt: createdAt.UTC(), ExpiresAt: expiresAt.UTC()}
	if err := o.Validate(); err != nil {
		return Order{}, err
	}
	return o, nil
}

func (o Order) Validate() error {
	if !key.MatchString(o.ID) || !key.MatchString(o.TenantID) || !key.MatchString(o.ChangeID) || !key.MatchString(o.PlanCode) || o.PlanVersion == 0 || strings.TrimSpace(o.PriceRef) == "" || len(o.PriceRef) > 128 || !currency.MatchString(o.Currency) || o.AmountMinor == 0 || !validProvider(o.Provider) || o.Revision == 0 || o.CreatedAt.IsZero() || o.ExpiresAt.IsZero() || !o.ExpiresAt.After(o.CreatedAt) {
		return ErrInvalid
	}
	switch o.State {
	case Pending, Cancelled, Expired:
		if o.ProviderTransactionID != "" || o.PaidAt != nil {
			return ErrInvalid
		}
	case Paid:
		if !key.MatchString(o.ProviderTransactionID) || o.PaidAt == nil || o.PaidAt.IsZero() {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	return nil
}

// RecordPaid accepts an exact duplicate callback but rejects every callback
// that changes provider, payment identity or immutable settlement facts.
func (o *Order) RecordPaid(receipt ProviderReceipt, observedAt time.Time) (bool, error) {
	if o == nil || o.Validate() != nil || !validProvider(receipt.Provider) || !key.MatchString(receipt.ProviderTransactionID) || !currency.MatchString(receipt.Currency) || receipt.AmountMinor == 0 || receipt.PaidAt.IsZero() || observedAt.IsZero() {
		return false, ErrInvalid
	}
	if receipt.Provider != o.Provider || receipt.Currency != o.Currency || receipt.AmountMinor != o.AmountMinor {
		return false, ErrConflict
	}
	if o.State == Paid {
		if o.ProviderTransactionID == receipt.ProviderTransactionID {
			return false, nil
		}
		return false, ErrConflict
	}
	if o.State != Pending || !observedAt.UTC().Before(o.ExpiresAt) {
		return false, ErrConflict
	}
	paidAt := receipt.PaidAt.UTC()
	o.State = Paid
	o.ProviderTransactionID = receipt.ProviderTransactionID
	o.PaidAt = &paidAt
	o.Revision++
	if err := o.Validate(); err != nil {
		return false, err
	}
	return true, nil
}
