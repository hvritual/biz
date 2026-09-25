package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type securityNotificationDeliveryLeaseRecord struct {
	EventID       string     `gorm:"column:event_id;primaryKey;size:64"`
	LeaseOwner    string     `gorm:"column:lease_owner;size:96;not null;default:''"`
	LeaseToken    uint64     `gorm:"column:lease_token;not null;default:0"`
	LeaseUntil    *time.Time `gorm:"column:lease_until;type:datetime(6)"`
	NextAttemptAt *time.Time `gorm:"column:next_attempt_at;type:datetime(6);index"`
	LastAttemptAt *time.Time `gorm:"column:last_attempt_at;type:datetime(6)"`
}

func (securityNotificationDeliveryLeaseRecord) TableName() string {
	return "biz_security_notification_outbox"
}

var _ ports.ReliableSecurityNotificationRepository = (*VerificationRepository)(nil)

func (repository *VerificationRepository) EnsureReliableSecurityNotificationSchema(ctx context.Context) error {
	if repository == nil || repository.database == nil {
		return domain.ErrNotificationUnavailable
	}
	return repository.database.WithContext(ctx).AutoMigrate(&securityNotificationDeliveryLeaseRecord{})
}

func (repository *VerificationRepository) ClaimNextSecurityNotification(
	ctx context.Context,
	workerID string,
	policy domain.NotificationRetryPolicy,
) (domain.ReliableSecurityNotificationClaim, domain.NotificationDeliveryReceipt, error) {
	if repository == nil || repository.database == nil || repository.protection == nil ||
		strings.TrimSpace(workerID) == "" || len(workerID) > 96 || policy.Validate() != nil {
		return domain.ReliableSecurityNotificationClaim{}, domain.NotificationDeliveryReceipt{}, domain.ErrNotificationUnavailable
	}
	var claim domain.ReliableSecurityNotificationClaim
	var receipt domain.NotificationDeliveryReceipt
	err := repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := verificationDatabaseNow(ctx, tx)
		if err != nil {
			return err
		}
		legacyStale := now.Add(-policy.LeaseDuration)
		var record securityNotificationOutboxRecord
		err = tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("(state = ? OR (state IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (state = ? AND ((lease_until IS NOT NULL AND lease_until <= ?) OR (lease_until IS NULL AND updated_at <= ?))))",
				domain.NotificationStatePending,
				[]string{domain.NotificationStateFailed, domain.NotificationStateRetryWait}, now,
				domain.NotificationStateSending, now, legacyStale).
			Order("COALESCE(next_attempt_at, created_at) ASC, event_id ASC").
			First(&record).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		receipt = notificationReceipt(record)
		if !record.ExpiresAt.After(now) {
			return repository.finalizeSecurityNotification(ctx, tx, record, now, "NOTIFICATION_EXPIRED", &receipt)
		}
		if record.Attempts >= policy.MaxAttempts {
			return repository.finalizeSecurityNotification(ctx, tx, record, now, "DELIVERY_RETRIES_EXHAUSTED", &receipt)
		}
		var lease securityNotificationDeliveryLeaseRecord
		if err := tx.WithContext(ctx).Where("event_id = ?", record.EventID).First(&lease).Error; err != nil {
			return err
		}
		if lease.LeaseToken == ^uint64(0) {
			return domain.ErrNotificationUnavailable
		}
		destination, err := repository.protection.DecryptNotification(record.EventID, "destination", record.DestinationCiphertext, record.KeyVersion)
		if err != nil {
			return err
		}
		secret := ""
		if record.SecretCiphertext != "" {
			secret, err = repository.protection.DecryptNotification(record.EventID, "secret", record.SecretCiphertext, record.KeyVersion)
			if err != nil {
				return err
			}
		}
		if !constantVerificationEqual(record.BindingHash, repository.notificationBinding(record, secret)) {
			return ErrVerificationCipherCorrupt
		}
		record.Attempts++
		until := canonicalVerificationTime(now.Add(policy.LeaseDuration))
		token := lease.LeaseToken + 1
		if err := tx.WithContext(ctx).Model(&securityNotificationOutboxRecord{}).Where("event_id = ?", record.EventID).
			Updates(map[string]any{
				"state": domain.NotificationStateSending, "attempts": record.Attempts,
				"failure_code": "", "lease_owner": workerID, "lease_token": token,
				"lease_until": until, "last_attempt_at": now, "next_attempt_at": nil, "updated_at": now,
			}).Error; err != nil {
			return err
		}
		claim = domain.ReliableSecurityNotificationClaim{
			SecurityNotificationClaim: domain.SecurityNotificationClaim{
				EventID: record.EventID, BusinessEventID: record.BusinessEventID,
				Kind: domain.SecurityNotificationKind(record.Kind), Purpose: domain.VerificationPurpose(record.Purpose),
				UserID: record.UserID, TenantID: record.TenantID, FlowID: record.FlowID,
				Channel: domain.SecurityNotificationChannel(record.Channel), Destination: destination,
				Secret: secret, ExpiresAt: record.ExpiresAt, Attempt: record.Attempts,
			},
			WorkerID: workerID, LeaseToken: token, LeaseUntil: until,
		}
		receipt.State = domain.NotificationStateSending
		receipt.Attempt = record.Attempts
		return nil
	})
	return claim, receipt, err
}

func (repository *VerificationRepository) reliableLocked(ctx context.Context, tx *gorm.DB, claim domain.ReliableSecurityNotificationClaim) (securityNotificationOutboxRecord, time.Time, error) {
	if err := claim.Validate(); err != nil {
		return securityNotificationOutboxRecord{}, time.Time{}, err
	}
	var record securityNotificationOutboxRecord
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id = ?", claim.EventID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return record, time.Time{}, domain.ErrNotificationUnavailable
		}
		return record, time.Time{}, err
	}
	var lease securityNotificationDeliveryLeaseRecord
	if err := tx.WithContext(ctx).Where("event_id = ?", claim.EventID).First(&lease).Error; err != nil {
		return record, time.Time{}, err
	}
	now, err := verificationDatabaseNow(ctx, tx)
	if err != nil {
		return record, time.Time{}, err
	}
	if record.State == domain.NotificationStateDelivered {
		return record, now, nil
	}
	if record.State != domain.NotificationStateSending || lease.LeaseOwner != claim.WorkerID ||
		lease.LeaseToken != claim.LeaseToken || lease.LeaseUntil == nil || !now.Before(*lease.LeaseUntil) {
		return record, now, domain.ErrNotificationLease
	}
	return record, now, nil
}

func (repository *VerificationRepository) CompleteReliableSecurityNotification(ctx context.Context, claim domain.ReliableSecurityNotificationClaim, providerReceipt string) (domain.NotificationDeliveryReceipt, error) {
	if repository == nil || repository.database == nil || strings.TrimSpace(providerReceipt) == "" || len(providerReceipt) > 200 {
		return domain.NotificationDeliveryReceipt{}, domain.ErrNotificationUnavailable
	}
	var receipt domain.NotificationDeliveryReceipt
	err := repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, now, err := repository.reliableLocked(ctx, tx, claim)
		if err != nil {
			return err
		}
		if record.State == domain.NotificationStateDelivered {
			receipt = notificationReceipt(record)
			return nil
		}
		record.State = domain.NotificationStateDelivered
		record.ProviderReceipt = strings.TrimSpace(providerReceipt)
		record.FailureCode = ""
		record.DeliveredAt = &now
		record.DestinationCiphertext = ""
		record.SecretCiphertext = ""
		if err := tx.WithContext(ctx).Model(&securityNotificationOutboxRecord{}).Where("event_id = ?", record.EventID).
			Updates(map[string]any{
				"state": record.State, "provider_receipt": record.ProviderReceipt, "failure_code": "",
				"delivered_at": now, "destination_ciphertext": "", "secret_ciphertext": "",
				"lease_owner": "", "lease_until": nil, "next_attempt_at": nil, "updated_at": now,
			}).Error; err != nil {
			return err
		}
		receipt = notificationReceipt(record)
		return nil
	})
	return receipt, err
}

func (repository *VerificationRepository) FailReliableSecurityNotification(ctx context.Context, claim domain.ReliableSecurityNotificationClaim, failureCode string, retryable bool, policy domain.NotificationRetryPolicy) (domain.NotificationDeliveryReceipt, error) {
	if repository == nil || repository.database == nil || policy.Validate() != nil {
		return domain.NotificationDeliveryReceipt{}, domain.ErrNotificationUnavailable
	}
	var receipt domain.NotificationDeliveryReceipt
	err := repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		record, now, err := repository.reliableLocked(ctx, tx, claim)
		if err != nil {
			return err
		}
		if record.State == domain.NotificationStateDelivered {
			receipt = notificationReceipt(record)
			return nil
		}
		code := sanitizeFailureCode(failureCode)
		delay, hasRetry := policy.NextDelay(record.Attempts)
		if retryable && hasRetry && record.ExpiresAt.After(now.Add(delay)) {
			next := canonicalVerificationTime(now.Add(delay))
			record.State = domain.NotificationStateRetryWait
			record.FailureCode = code
			if err := tx.WithContext(ctx).Model(&securityNotificationOutboxRecord{}).Where("event_id = ?", record.EventID).
				Updates(map[string]any{
					"state": record.State, "failure_code": code, "lease_owner": "", "lease_until": nil,
					"next_attempt_at": next, "updated_at": now,
				}).Error; err != nil {
				return err
			}
			receipt = notificationReceipt(record)
			return nil
		}
		terminalCode := code
		if retryable && !hasRetry {
			terminalCode = "DELIVERY_RETRIES_EXHAUSTED"
		} else if retryable && hasRetry && !record.ExpiresAt.After(now.Add(delay)) {
			terminalCode = "NOTIFICATION_EXPIRES_BEFORE_RETRY"
		}
		return repository.finalizeSecurityNotification(ctx, tx, record, now, terminalCode, &receipt)
	})
	return receipt, err
}

func (repository *VerificationRepository) finalizeSecurityNotification(ctx context.Context, tx *gorm.DB, record securityNotificationOutboxRecord, now time.Time, failureCode string, receipt *domain.NotificationDeliveryReceipt) error {
	record.State = domain.NotificationStateManualReview
	record.FailureCode = sanitizeFailureCode(failureCode)
	record.DestinationCiphertext = ""
	record.SecretCiphertext = ""
	if err := tx.WithContext(ctx).Model(&securityNotificationOutboxRecord{}).Where("event_id = ?", record.EventID).
		Updates(map[string]any{
			"state": record.State, "failure_code": record.FailureCode,
			"destination_ciphertext": "", "secret_ciphertext": "",
			"lease_owner": "", "lease_until": nil, "next_attempt_at": nil, "updated_at": now,
		}).Error; err != nil {
		return err
	}
	*receipt = notificationReceipt(record)
	return nil
}
