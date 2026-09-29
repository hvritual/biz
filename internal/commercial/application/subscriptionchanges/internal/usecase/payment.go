package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/domain/payment"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/requestscope"
)

func paymentOrderDTO(order payment.Order) *v1.PaymentOrderDTO {
	paidAt := ""
	if order.PaidAt != nil {
		paidAt = order.PaidAt.UTC().Format(time.RFC3339Nano)
	}
	return &v1.PaymentOrderDTO{OrderId: order.ID, ChangeId: order.ChangeID, PlanCode: order.PlanCode, PlanVersion: order.PlanVersion, PriceRef: order.PriceRef, Currency: order.Currency, AmountMinor: order.AmountMinor, Provider: order.Provider, State: order.State, Revision: order.Revision, ProviderTransactionId: order.ProviderTransactionID, CreatedAt: order.CreatedAt.UTC().Format(time.RFC3339Nano), ExpiresAt: order.ExpiresAt.UTC().Format(time.RFC3339Nano), PaidAt: paidAt}
}

func orderID(tenantID, changeID string) string {
	digest := sha256.Sum256([]byte(tenantID + "\x00" + changeID))
	return "ord-" + hex.EncodeToString(digest[:24])
}

func (s *service) CreateMyPaymentOrder(ctx context.Context, request *v1.CreateMyPaymentOrderRequest) (*v1.PaymentOrderDTO, error) {
	principal, err := tenantChangeActor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if request == nil || !change.Key(request.ChangeId) || len(request.PreviewHash) != 64 || !validKey(ctx, request.RequestId) {
		return nil, expose(change.ErrInvalid)
	}
	result, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.SubscriptionChangeRepositories]) (payment.Order, error) {
		repositories, call := scope.Repositories(), scope.Context()
		if repositories.Payments == nil {
			return payment.Order{}, payment.ErrNotFound
		}
		if _, err := repositories.Changes.LockTenant(call, principal.TenantID); err != nil {
			return payment.Order{}, err
		}
		preview, err := repositories.Changes.Preview(call, principal.TenantID, request.ChangeId, true)
		if err != nil {
			return payment.Order{}, err
		}
		if preview == nil || preview.ActorID != principal.Subject || preview.Hash != request.PreviewHash || !preview.ExpiresAt.After(preview.CreatedAt) {
			return payment.Order{}, change.ErrConflict
		}
		now, err := repositories.Changes.Now(call)
		if err != nil {
			return payment.Order{}, err
		}
		if !now.Before(preview.ExpiresAt) {
			return payment.Order{}, change.ErrExpired
		}
		terms := preview.Target.Terms
		if terms.PriceRef == "" || terms.Currency == "" || terms.AmountMinor == 0 {
			return payment.Order{}, change.ErrInvalid
		}
		existing, err := repositories.Payments.ForChange(call, principal.TenantID, preview.ChangeID, true)
		if err == nil {
			if existing.Provider != request.Provider || existing.PlanCode != preview.Target.PlanCode || existing.PlanVersion != preview.Target.Number || existing.PriceRef != terms.PriceRef || existing.Currency != terms.Currency || existing.AmountMinor != terms.AmountMinor {
				return payment.Order{}, payment.ErrConflict
			}
			return *existing, nil
		}
		if !errors.Is(err, payment.ErrNotFound) {
			return payment.Order{}, err
		}
		expiresAt := now.Add(15 * time.Minute)
		if preview.ExpiresAt.Before(expiresAt) {
			expiresAt = preview.ExpiresAt
		}
		order, err := payment.NewOrder(orderID(principal.TenantID, preview.ChangeID), principal.TenantID, preview.ChangeID, preview.Target.PlanCode, preview.Target.Number, terms.PriceRef, terms.Currency, terms.AmountMinor, request.Provider, now, expiresAt)
		if err != nil {
			return payment.Order{}, err
		}
		if err = repositories.Payments.Create(call, order); err != nil {
			return payment.Order{}, err
		}
		return order, nil
	})
	if err != nil {
		return nil, exposePayment(err)
	}
	return paymentOrderDTO(result), nil
}

func (s *service) GetMyPaymentOrder(ctx context.Context, request *v1.GetMyPaymentOrderRequest) (*v1.PaymentOrderDTO, error) {
	principal, err := tenantChangeActor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if request == nil || !change.Key(request.OrderId) {
		return nil, expose(change.ErrInvalid)
	}
	order, err := requestscope.JoinValue(ctx, s.repositories, func(scope *requestscope.View[ports.SubscriptionChangeRepositories]) (payment.Order, error) {
		if scope.Repositories().Payments == nil {
			return payment.Order{}, payment.ErrNotFound
		}
		value, err := scope.Repositories().Payments.Get(scope.Context(), request.OrderId, false)
		if err != nil {
			return payment.Order{}, err
		}
		if value.TenantID != principal.TenantID {
			return payment.Order{}, change.ErrScope
		}
		return *value, nil
	})
	if err != nil {
		return nil, exposePayment(err)
	}
	return paymentOrderDTO(order), nil
}

func exposePayment(err error) error {
	switch {
	case errors.Is(err, payment.ErrNotFound):
		return expose(change.ErrNotFound)
	case errors.Is(err, payment.ErrConflict):
		return expose(change.ErrConflict)
	case errors.Is(err, payment.ErrInvalid):
		return expose(change.ErrInvalid)
	default:
		return expose(err)
	}
}
