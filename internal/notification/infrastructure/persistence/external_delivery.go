package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ ports.ExternalTaskRepository = (*RoutingRepository)(nil)

func (r *RoutingRepository) ClaimNextExternalTask(
	ctx context.Context,
	worker string,
	policy domain.ExternalDeliveryPolicy,
) (domain.ExternalTaskClaim, domain.ExternalTaskReceipt, error) {
	if r == nil || r.db == nil || !domain.ValidConfigurationID(worker, 96) || policy.Validate() != nil {
		return domain.ExternalTaskClaim{}, domain.ExternalTaskReceipt{}, domain.ErrExternalDeliveryInvalid
	}
	var claim domain.ExternalTaskClaim
	var receipt domain.ExternalTaskReceipt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := routingNow(ctx, tx)
		if err != nil {
			return err
		}
		var row externalTaskRecord
		err = tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("state = ? OR (state = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (state = ? AND lease_until <= ?)",
				domain.ExternalTaskStatePending, domain.ExternalTaskStateRetryWait, now, domain.ExternalTaskStateLeased, now).
			Order("COALESCE(next_attempt_at, created_at) ASC, task_id ASC").
			First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if row.Attempts >= policy.MaxAttempts {
			receipt, err = terminalExternalTask(ctx, tx, row, domain.ExternalTaskStateManualReview, "DELIVERY_RETRIES_EXHAUSTED", now)
			return err
		}
		if row.LeaseToken == ^uint64(0) {
			return domain.ErrExternalDeliveryUnavailable
		}
		row.Attempts++
		row.LeaseToken++
		until := time.UnixMicro(now.Add(policy.LeaseDuration).UTC().UnixMicro()).UTC()
		if err := tx.Model(&externalTaskRecord{}).Where("task_id = ?", row.TaskID).Updates(map[string]any{
			"state": domain.ExternalTaskStateLeased, "attempts": row.Attempts,
			"lease_owner": worker, "lease_token": row.LeaseToken, "lease_until": until,
			"next_attempt_at": nil, "failure_code": "", "updated_at": now,
		}).Error; err != nil {
			return err
		}
		row.State = domain.ExternalTaskStateLeased
		row.LeaseOwner = worker
		row.LeaseUntil = &until
		claim = externalTaskClaim(row)
		receipt = externalTaskReceipt(row)
		return nil
	})
	return claim, receipt, err
}

func externalTaskClaim(row externalTaskRecord) domain.ExternalTaskClaim {
	claim := domain.ExternalTaskClaim{
		TaskID: row.TaskID, TenantID: row.TenantID, UserID: row.UserID, EventID: row.EventID,
		Channel: row.Channel, ConfigurationID: row.ConfigurationID, ConfigurationVersion: row.ConfigurationVersion,
		GroupID: row.GroupID, TypeCode: row.TypeCode, Level: domain.MessageLevel(row.Level),
		TraceID: row.TraceID, ReferenceKind: row.ReferenceKind, ReferenceID: row.ReferenceID,
		Attempt: row.Attempts, WorkerID: row.LeaseOwner, LeaseToken: row.LeaseToken,
	}
	if row.LeaseUntil != nil {
		claim.LeaseUntil = row.LeaseUntil.UTC()
	}
	return claim
}

func externalTaskReceipt(row externalTaskRecord) domain.ExternalTaskReceipt {
	result := domain.ExternalTaskReceipt{
		TaskID: row.TaskID, State: row.State, Attempt: row.Attempts,
		FailureCode: row.FailureCode, ProviderReceipt: row.ProviderReceipt,
	}
	if row.NextAttemptAt != nil {
		next := row.NextAttemptAt.UTC()
		result.NextAttemptAt = &next
	}
	return result
}

func (r *RoutingRepository) lockedExternalTask(
	ctx context.Context,
	tx *gorm.DB,
	claim domain.ExternalTaskClaim,
) (externalTaskRecord, time.Time, error) {
	if err := claim.Validate(); err != nil {
		return externalTaskRecord{}, time.Time{}, err
	}
	var row externalTaskRecord
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("task_id = ?", claim.TaskID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return row, time.Time{}, domain.ErrExternalDeliveryUnavailable
		}
		return row, time.Time{}, err
	}
	now, err := routingNow(ctx, tx)
	if err != nil {
		return row, time.Time{}, err
	}
	if row.State == domain.ExternalTaskStateProviderAccepted || row.State == domain.ExternalTaskStateDelivered ||
		row.State == domain.ExternalTaskStateCancelled || row.State == domain.ExternalTaskStateManualReview {
		return row, now, nil
	}
	if row.State != domain.ExternalTaskStateLeased || row.LeaseOwner != claim.WorkerID ||
		row.LeaseToken != claim.LeaseToken || row.LeaseUntil == nil || !now.Before(*row.LeaseUntil) {
		return row, now, domain.ErrExternalDeliveryLease
	}
	return row, now, nil
}

func (r *RoutingRepository) CompleteExternalTask(
	ctx context.Context,
	claim domain.ExternalTaskClaim,
	result domain.ExternalProviderResult,
) (domain.ExternalTaskReceipt, error) {
	if r == nil || r.db == nil || result.Validate() != nil {
		return domain.ExternalTaskReceipt{}, domain.ErrExternalDeliveryInvalid
	}
	var receipt domain.ExternalTaskReceipt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, now, err := r.lockedExternalTask(ctx, tx, claim)
		if err != nil {
			return err
		}
		if row.State == domain.ExternalTaskStateProviderAccepted || row.State == domain.ExternalTaskStateDelivered {
			if row.ProviderReceipt != result.ReceiptID {
				return domain.ErrExternalDeliveryLease
			}
			receipt = externalTaskReceipt(row)
			return nil
		}
		if row.State != domain.ExternalTaskStateLeased {
			return domain.ErrExternalDeliveryLease
		}
		state := domain.ExternalTaskStateProviderAccepted
		values := map[string]any{
			"state": state, "provider_receipt": result.ReceiptID, "failure_code": "",
			"accepted_at": now, "lease_owner": "", "lease_until": nil,
			"next_attempt_at": nil, "updated_at": now,
		}
		row.AcceptedAt = &now
		if result.Status == domain.ExternalProviderDelivered {
			state = domain.ExternalTaskStateDelivered
			values["state"] = state
			values["delivered_at"] = now
			row.DeliveredAt = &now
		}
		if err := tx.Model(&externalTaskRecord{}).Where("task_id = ?", row.TaskID).Updates(values).Error; err != nil {
			return err
		}
		row.State = state
		row.ProviderReceipt = result.ReceiptID
		row.FailureCode = ""
		row.LeaseOwner = ""
		row.LeaseUntil = nil
		row.NextAttemptAt = nil
		receipt = externalTaskReceipt(row)
		return nil
	})
	return receipt, err
}

func (r *RoutingRepository) FailExternalTask(
	ctx context.Context,
	claim domain.ExternalTaskClaim,
	failureCode string,
	retryable bool,
	policy domain.ExternalDeliveryPolicy,
) (domain.ExternalTaskReceipt, error) {
	if r == nil || r.db == nil || policy.Validate() != nil {
		return domain.ExternalTaskReceipt{}, domain.ErrExternalDeliveryInvalid
	}
	var receipt domain.ExternalTaskReceipt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, now, err := r.lockedExternalTask(ctx, tx, claim)
		if err != nil {
			return err
		}
		if row.State != domain.ExternalTaskStateLeased {
			receipt = externalTaskReceipt(row)
			return nil
		}
		code := sanitizeExternalFailureCode(failureCode)
		delay, ok := policy.NextDelay(row.Attempts)
		if retryable && ok {
			next := time.UnixMicro(now.Add(delay).UTC().UnixMicro()).UTC()
			if err := tx.Model(&externalTaskRecord{}).Where("task_id = ?", row.TaskID).Updates(map[string]any{
				"state": domain.ExternalTaskStateRetryWait, "failure_code": code,
				"lease_owner": "", "lease_until": nil, "next_attempt_at": next, "updated_at": now,
			}).Error; err != nil {
				return err
			}
			row.State = domain.ExternalTaskStateRetryWait
			row.FailureCode = code
			row.LeaseOwner = ""
			row.LeaseUntil = nil
			row.NextAttemptAt = &next
			receipt = externalTaskReceipt(row)
			return nil
		}
		terminalCode := code
		if retryable && !ok {
			terminalCode = "DELIVERY_RETRIES_EXHAUSTED"
		}
		receipt, err = terminalExternalTask(ctx, tx, row, domain.ExternalTaskStateManualReview, terminalCode, now)
		return err
	})
	return receipt, err
}

func (r *RoutingRepository) TerminalExternalTask(
	ctx context.Context,
	claim domain.ExternalTaskClaim,
	state string,
	failureCode string,
) (domain.ExternalTaskReceipt, error) {
	if r == nil || r.db == nil || (state != domain.ExternalTaskStateCancelled && state != domain.ExternalTaskStateManualReview) {
		return domain.ExternalTaskReceipt{}, domain.ErrExternalDeliveryInvalid
	}
	var receipt domain.ExternalTaskReceipt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, now, err := r.lockedExternalTask(ctx, tx, claim)
		if err != nil {
			return err
		}
		if row.State != domain.ExternalTaskStateLeased {
			receipt = externalTaskReceipt(row)
			return nil
		}
		receipt, err = terminalExternalTask(ctx, tx, row, state, failureCode, now)
		return err
	})
	return receipt, err
}

func terminalExternalTask(
	ctx context.Context,
	tx *gorm.DB,
	row externalTaskRecord,
	state string,
	failureCode string,
	now time.Time,
) (domain.ExternalTaskReceipt, error) {
	code := sanitizeExternalFailureCode(failureCode)
	if err := tx.WithContext(ctx).Model(&externalTaskRecord{}).Where("task_id = ?", row.TaskID).Updates(map[string]any{
		"state": state, "failure_code": code, "lease_owner": "", "lease_until": nil,
		"next_attempt_at": nil, "updated_at": now,
	}).Error; err != nil {
		return domain.ExternalTaskReceipt{}, err
	}
	row.State = state
	row.FailureCode = code
	row.LeaseOwner = ""
	row.LeaseUntil = nil
	row.NextAttemptAt = nil
	return externalTaskReceipt(row), nil
}

func sanitizeExternalFailureCode(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "DELIVERY_FAILED"
	}
	if len(value) > 64 {
		value = value[:64]
	}
	var builder strings.Builder
	for _, char := range value {
		if (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' {
			builder.WriteRune(char)
		}
	}
	if builder.Len() == 0 {
		return "DELIVERY_FAILED"
	}
	return builder.String()
}
