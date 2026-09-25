package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type notificationPreferenceRecord struct {
	TenantID  string    `gorm:"column:tenant_id;type:varbinary(64);primaryKey"`
	UserID    string    `gorm:"column:user_id;type:varbinary(64);primaryKey"`
	Channel   string    `gorm:"column:channel;type:varbinary(16);primaryKey"`
	State     string    `gorm:"column:state;size:16;not null"`
	Version   uint64    `gorm:"column:version;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime(6);not null"`
}

func (notificationPreferenceRecord) TableName() string { return "biz_notification_preferences" }

type notificationPreferenceReceiptRecord struct {
	TenantID    string    `gorm:"column:tenant_id;type:varbinary(64);primaryKey"`
	UserID      string    `gorm:"column:user_id;type:varbinary(64);primaryKey"`
	KeyHash     string    `gorm:"column:key_hash;type:varbinary(64);primaryKey"`
	PayloadHash string    `gorm:"column:payload_hash;size:64;not null"`
	Policy      string    `gorm:"column:policy;size:64;not null"`
	Channel     string    `gorm:"column:channel;size:16;not null"`
	State       string    `gorm:"column:state;size:16;not null"`
	Version     uint64    `gorm:"column:version;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime(6);not null"`
}

func (notificationPreferenceReceiptRecord) TableName() string {
	return "biz_notification_preference_receipts"
}

var _ ports.SelfNotificationPreferences = (*Store)(nil)
var _ ports.OptionalNotificationPreferenceReader = (*Store)(nil)

func (row notificationPreferenceRecord) preference() (domain.NotificationPreference, error) {
	updated := row.UpdatedAt.UTC()
	preference := domain.NotificationPreference{
		Channel: domain.NotificationPreferenceChannel(row.Channel),
		State:   domain.NotificationPreferenceState(row.State), Version: row.Version, UpdatedAt: &updated,
	}
	return preference, preference.Validate()
}

func (row notificationPreferenceReceiptRecord) receipt() (domain.NotificationPreferenceReceipt, error) {
	if row.Policy != domain.NotificationPreferencePolicy {
		return domain.NotificationPreferenceReceipt{}, domain.ErrNotificationPreferenceUnavailable
	}
	preference, err := (notificationPreferenceRecord{Channel: row.Channel, State: row.State, Version: row.Version, UpdatedAt: row.UpdatedAt}).preference()
	if err != nil {
		return domain.NotificationPreferenceReceipt{}, err
	}
	return domain.NotificationPreferenceReceipt{
		NotificationPreferenceOwner: domain.NotificationPreferenceOwner{TenantID: row.TenantID, UserID: row.UserID},
		Policy:                      row.Policy, ReceiptID: row.KeyHash, Preference: preference,
	}, nil
}

// An existing Membership row serializes first-write absence, two channels, CAS,
// and receipt insertion. Do not replace it with SELECT FOR UPDATE on a missing
// preference row: concurrent first writes must have an existing lock anchor.
// Lock order for this boundary is Tenant -> Account -> Membership. Tenant and
// Account remain shared; only the current tenant/user Membership is exclusive.
func lockNotificationPreferenceOwner(tx *gorm.DB, owner domain.NotificationPreferenceOwner) error {
	var tenant tenantRecord
	if err := tx.Select("id", "status").Clauses(clause.Locking{Strength: "SHARE"}).
		Where("id = ? AND status = ?", owner.TenantID, domain.TenantStatusActive).First(&tenant).Error; err != nil {
		return notificationPreferenceOwnerError(err)
	}
	var user userRecord
	if err := tx.Select("id", "status").Clauses(clause.Locking{Strength: "SHARE"}).
		Where("id = ? AND status = ?", owner.UserID, "active").First(&user).Error; err != nil {
		return notificationPreferenceOwnerError(err)
	}
	var member membershipRecord
	if err := tx.Select("tenant_id", "user_id", "status", "self_deleted_at").Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND user_id = ? AND status = ? AND self_deleted_at IS NULL", owner.TenantID, owner.UserID, domain.TenantMemberStatusActive).
		First(&member).Error; err != nil {
		return notificationPreferenceOwnerError(err)
	}
	// Older authority tables can use a case-insensitive collation. Preserve the
	// canonical identity returned by those tables, not an alias supplied upstream.
	if tenant.ID != owner.TenantID || user.ID != owner.UserID || member.TenantID != owner.TenantID || member.UserID != owner.UserID {
		return domain.ErrNotificationPreferenceForbidden
	}
	return nil
}

func notificationPreferenceOwnerError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrNotificationPreferenceForbidden
	}
	return err
}

func readNotificationPreference(tx *gorm.DB, owner domain.NotificationPreferenceOwner, channel domain.NotificationPreferenceChannel) (domain.NotificationPreference, error) {
	var row notificationPreferenceRecord
	err := tx.Where("tenant_id = ? AND user_id = ? AND channel = ?", owner.TenantID, owner.UserID, string(channel)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.NotificationPreference{Channel: channel, State: domain.NotificationPreferenceDefault}, nil
	}
	if err != nil {
		return domain.NotificationPreference{}, err
	}
	return row.preference()
}

func (store *Store) preferenceTransaction(ctx context.Context, owner domain.NotificationPreferenceOwner, session *notificationPreferenceSession, fn func(*gorm.DB) error) error {
	if store == nil || store.database == nil {
		return domain.ErrNotificationPreferenceUnavailable
	}
	if err := owner.Validate(); err != nil {
		return err
	}
	return store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockNotificationPreferenceOwner(tx, owner); err != nil {
			return err
		}
		if session != nil {
			if err := session.validate(tx, owner); err != nil {
				return err
			}
		}
		return fn(tx)
	})
}

func (store *Store) ReadNotificationPreferences(ctx context.Context, owner domain.NotificationPreferenceOwner) (domain.NotificationPreferenceSet, error) {
	return store.readNotificationPreferences(ctx, owner, nil)
}

func (store *Store) readNotificationPreferences(ctx context.Context, owner domain.NotificationPreferenceOwner, session *notificationPreferenceSession) (domain.NotificationPreferenceSet, error) {
	result, err := domain.DefaultNotificationPreferences(owner)
	if err != nil {
		return domain.NotificationPreferenceSet{}, err
	}
	err = store.preferenceTransaction(ctx, owner, session, func(tx *gorm.DB) error {
		var rows []notificationPreferenceRecord
		if err := tx.Where("tenant_id = ? AND user_id = ?", owner.TenantID, owner.UserID).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			preference, err := row.preference()
			if err != nil {
				return err
			}
			switch preference.Channel {
			case domain.NotificationPreferenceSMS:
				result.SMS = preference
			case domain.NotificationPreferenceEmail:
				result.Email = preference
			default:
				return domain.ErrNotificationPreferenceInvalid
			}
		}
		return nil
	})
	if err != nil {
		return domain.NotificationPreferenceSet{}, err
	}
	return result, nil
}

func (store *Store) ReadNotificationPreference(ctx context.Context, owner domain.NotificationPreferenceOwner, channel domain.NotificationPreferenceChannel) (domain.NotificationPreference, error) {
	if !channel.Valid() {
		return domain.NotificationPreference{}, domain.ErrNotificationPreferenceInvalid
	}
	var result domain.NotificationPreference
	err := store.preferenceTransaction(ctx, owner, nil, func(tx *gorm.DB) error {
		var err error
		result, err = readNotificationPreference(tx, owner, channel)
		return err
	})
	if err != nil {
		return domain.NotificationPreference{}, err
	}
	return result, nil
}

func (store *Store) ChangeNotificationPreference(ctx context.Context, owner domain.NotificationPreferenceOwner, change domain.NotificationPreferenceChange) (domain.NotificationPreferenceReceipt, error) {
	return store.changeNotificationPreference(ctx, owner, change, nil)
}

func (store *Store) changeNotificationPreference(ctx context.Context, owner domain.NotificationPreferenceOwner, change domain.NotificationPreferenceChange, session *notificationPreferenceSession) (domain.NotificationPreferenceReceipt, error) {
	keyHash, payloadHash, err := change.Fingerprints(owner)
	if err != nil {
		return domain.NotificationPreferenceReceipt{}, err
	}
	var result domain.NotificationPreferenceReceipt
	err = store.preferenceTransaction(ctx, owner, session, func(tx *gorm.DB) error {
		var existing notificationPreferenceReceiptRecord
		err := tx.Where("tenant_id = ? AND user_id = ? AND key_hash = ?", owner.TenantID, owner.UserID, keyHash).First(&existing).Error
		if err == nil {
			if existing.PayloadHash != payloadHash {
				return domain.ErrNotificationPreferenceIdempotencyConflict
			}
			result, err = existing.receipt()
			return err // Replay must not execute the mutation or append another audit.
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		current, err := readNotificationPreference(tx, owner, change.Channel)
		if err != nil {
			return err
		}
		next, err := change.Apply(current, time.Now().UTC())
		if err != nil {
			return err
		}
		if current.Version == 0 {
			row := notificationPreferenceRecord{
				TenantID: owner.TenantID, UserID: owner.UserID, Channel: string(next.Channel),
				State: string(next.State), Version: next.Version, UpdatedAt: *next.UpdatedAt,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		} else {
			update := tx.Model(&notificationPreferenceRecord{}).
				Where("tenant_id = ? AND user_id = ? AND channel = ? AND version = ?", owner.TenantID, owner.UserID, string(change.Channel), change.ExpectedVersion).
				Updates(map[string]any{"state": string(next.State), "version": next.Version, "updated_at": *next.UpdatedAt})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return domain.ErrNotificationPreferenceConflict
			}
		}
		receipt := notificationPreferenceReceiptRecord{
			TenantID: owner.TenantID, UserID: owner.UserID, KeyHash: keyHash, PayloadHash: payloadHash,
			Policy: domain.NotificationPreferencePolicy, Channel: string(next.Channel), State: string(next.State),
			Version: next.Version, UpdatedAt: *next.UpdatedAt,
		}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		if err := appendNotificationPreferenceAudit(ctx, tx, owner, receipt, current.State); err != nil {
			return err
		}
		result, err = receipt.receipt()
		return err
	})
	if err != nil {
		return domain.NotificationPreferenceReceipt{}, err
	}
	return result, nil
}

func appendNotificationPreferenceAudit(ctx context.Context, tx *gorm.DB, owner domain.NotificationPreferenceOwner, receipt notificationPreferenceReceiptRecord, previous domain.NotificationPreferenceState) error {
	repository, err := NewAuditRepository(tx)
	if err != nil {
		return err
	}
	for _, eventType := range []string{domain.AuditEventAttempt, domain.AuditEventOutcome} {
		event := domain.AuditEvent{
			EventID: TokenHash("notification-preference/" + receipt.KeyHash + "/" + eventType),
			AuditID: receipt.KeyHash, EventType: eventType, TenantID: owner.TenantID,
			ActorSubject: "user:" + owner.UserID, ActorUserID: owner.UserID,
			OperationID: "access.notification_preference.update", Module: "access",
			Target: "notification-preference:" + receipt.Channel, ResourceTenantID: owner.TenantID,
			IdempotencyRef: receipt.KeyHash, RequestDigest: receipt.PayloadHash, ReceiptRef: receipt.KeyHash,
			Reason: "optional preference " + string(previous) + " -> " + receipt.State,
			Risk:   domain.AuditRiskLow, Outcome: domain.AuditResultPending, OccurredAt: receipt.UpdatedAt,
		}
		if eventType == domain.AuditEventOutcome {
			event.Outcome = domain.AuditResultSuccess
		}
		if err := repository.AppendAuditEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}
