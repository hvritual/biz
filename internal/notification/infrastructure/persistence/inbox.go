package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	inboxMarkAllPrepared = "prepared"
	inboxMarkAllApplied  = "applied"
	markAllBatchSize     = 500
)

type inboxMarkAllRecord struct {
	CommandID   string    `gorm:"column:command_id;primaryKey;size:64"`
	TenantID    string    `gorm:"column:tenant_id;type:varbinary(64);not null;index:idx_notification_inbox_command_owner,priority:1"`
	UserID      string    `gorm:"column:user_id;type:varbinary(64);not null;index:idx_notification_inbox_command_owner,priority:2"`
	State       string    `gorm:"column:state;size:16;not null;index:idx_notification_inbox_command_state"`
	MarkedCount uint64    `gorm:"column:marked_count;not null;default:0"`
	ReadAt      time.Time `gorm:"column:read_at;type:datetime(6);not null"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime(6);not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime(6);not null"`
}

func (inboxMarkAllRecord) TableName() string { return "biz_notification_inbox_mark_all" }

type inboxMarkAllItemRecord struct {
	CommandID string `gorm:"column:command_id;primaryKey;size:64"`
	MessageID string `gorm:"column:message_id;primaryKey;size:64"`
}

func (inboxMarkAllItemRecord) TableName() string { return "biz_notification_inbox_mark_all_items" }

var _ ports.InboxRepository = (*RoutingRepository)(nil)

func (r *RoutingRepository) ReadUnreadInbox(ctx context.Context, owner domain.InboxOwner, limit int) (domain.InboxSnapshot, error) {
	if r == nil || r.db == nil || owner.Validate() != nil || limit < 1 || limit > 100 {
		return domain.InboxSnapshot{}, domain.ErrInboxInvalid
	}
	result := domain.InboxSnapshot{TenantID: owner.TenantID, UserID: owner.UserID, Messages: []domain.InAppMessage{}}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := routingNow(ctx, tx)
		if err != nil {
			return err
		}
		var count int64
		if err := tx.WithContext(ctx).Model(&inAppRecord{}).
			Where("tenant_id=? AND user_id=? AND read_at IS NULL", owner.TenantID, owner.UserID).
			Count(&count).Error; err != nil {
			return err
		}
		var rows []inAppRecord
		if err := tx.WithContext(ctx).
			Where("tenant_id=? AND user_id=? AND read_at IS NULL", owner.TenantID, owner.UserID).
			Order("created_at DESC,message_id DESC").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		result.UnreadCount = uint64(count)
		result.AsOf = now
		result.Messages = make([]domain.InAppMessage, 0, len(rows))
		for _, row := range rows {
			result.Messages = append(result.Messages, domain.InAppMessage{
				MessageID: row.MessageID, TypeCode: row.TypeCode, Level: domain.MessageLevel(row.Level),
				ReferenceKind: row.ReferenceKind, ReferenceID: row.ReferenceID, CreatedAt: row.CreatedAt.UTC(),
			})
		}
		return nil
	})
	if err != nil {
		return domain.InboxSnapshot{}, err
	}
	return result, nil
}

func (r *RoutingRepository) MarkAllRead(ctx context.Context, owner domain.InboxOwner, commandID string) (domain.MarkAllReadReceipt, error) {
	if r == nil || r.db == nil || owner.Validate() != nil || !domain.ValidConfigurationID(commandID, 64) {
		return domain.MarkAllReadReceipt{}, domain.ErrInboxInvalid
	}
	if _, err := r.prepareMarkAllRead(ctx, owner, commandID); err != nil {
		return domain.MarkAllReadReceipt{}, err
	}
	return r.applyPreparedMarkAllRead(ctx, owner, commandID)
}

func (r *RoutingRepository) prepareMarkAllRead(ctx context.Context, owner domain.InboxOwner, commandID string) (inboxMarkAllRecord, error) {
	var command inboxMarkAllRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.WithContext(ctx).Where("command_id=?", commandID).Take(&command).Error
		if err == nil {
			return validateMarkAllOwner(command, owner)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Persist the exact unread membership before applying any read side effect.
		// A later retry with this command ID must resume this snapshot rather than
		// re-evaluating unread rows that may have arrived after the user's action.
		var messageIDs []string
		if err := tx.WithContext(ctx).Model(&inAppRecord{}).Select("message_id").
			Where("tenant_id=? AND user_id=? AND read_at IS NULL", owner.TenantID, owner.UserID).
			Order("created_at ASC,message_id ASC").Pluck("message_id", &messageIDs).Error; err != nil {
			return err
		}
		now, err := routingNow(ctx, tx)
		if err != nil {
			return err
		}
		candidate := inboxMarkAllRecord{
			CommandID: commandID, TenantID: owner.TenantID, UserID: owner.UserID,
			State: inboxMarkAllPrepared, ReadAt: now, CreatedAt: now, UpdatedAt: now,
		}
		created := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			// The competing insert waits for the winner. Use a current locking
			// read here so MySQL REPEATABLE READ cannot hide the committed winner
			// behind this transaction's earlier consistent-read snapshot.
			if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("command_id=?", commandID).Take(&command).Error; err != nil {
				return err
			}
			return validateMarkAllOwner(command, owner)
		}
		if len(messageIDs) > 0 {
			items := make([]inboxMarkAllItemRecord, 0, len(messageIDs))
			for _, messageID := range messageIDs {
				items = append(items, inboxMarkAllItemRecord{CommandID: commandID, MessageID: messageID})
			}
			if err := tx.WithContext(ctx).CreateInBatches(items, markAllBatchSize).Error; err != nil {
				return err
			}
		}
		command = candidate
		return nil
	})
	if err != nil {
		return inboxMarkAllRecord{}, err
	}
	return command, nil
}

func (r *RoutingRepository) applyPreparedMarkAllRead(ctx context.Context, owner domain.InboxOwner, commandID string) (domain.MarkAllReadReceipt, error) {
	var receipt domain.MarkAllReadReceipt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var command inboxMarkAllRecord
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("command_id=?", commandID).Take(&command).Error; err != nil {
			return err
		}
		if err := validateMarkAllOwner(command, owner); err != nil {
			return err
		}
		if command.State == inboxMarkAllApplied {
			receipt = markAllReceipt(command)
			return nil
		}
		if command.State != inboxMarkAllPrepared {
			return domain.ErrInboxInvalid
		}

		var messageIDs []string
		if err := tx.WithContext(ctx).Model(&inboxMarkAllItemRecord{}).Select("message_id").
			Where("command_id=?", commandID).Order("message_id ASC").Pluck("message_id", &messageIDs).Error; err != nil {
			return err
		}
		var marked uint64
		for start := 0; start < len(messageIDs); start += markAllBatchSize {
			end := min(start+markAllBatchSize, len(messageIDs))
			updated := tx.WithContext(ctx).Model(&inAppRecord{}).
				Where("tenant_id=? AND user_id=? AND read_at IS NULL AND message_id IN ?", owner.TenantID, owner.UserID, messageIDs[start:end]).
				Update("read_at", command.ReadAt)
			if updated.Error != nil {
				return updated.Error
			}
			marked += uint64(updated.RowsAffected)
		}
		now, err := routingNow(ctx, tx)
		if err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Model(&inboxMarkAllRecord{}).Where("command_id=?", commandID).
			Updates(map[string]any{"state": inboxMarkAllApplied, "marked_count": marked, "updated_at": now}).Error; err != nil {
			return err
		}
		if err := tx.WithContext(ctx).Where("command_id=?", commandID).Delete(&inboxMarkAllItemRecord{}).Error; err != nil {
			return err
		}
		command.State = inboxMarkAllApplied
		command.MarkedCount = marked
		command.UpdatedAt = now
		receipt = markAllReceipt(command)
		return nil
	})
	if err != nil {
		return domain.MarkAllReadReceipt{}, err
	}
	return receipt, nil
}

func validateMarkAllOwner(command inboxMarkAllRecord, owner domain.InboxOwner) error {
	if command.TenantID != owner.TenantID || command.UserID != owner.UserID {
		return domain.ErrInboxInvalid
	}
	return nil
}

func markAllReceipt(row inboxMarkAllRecord) domain.MarkAllReadReceipt {
	return domain.MarkAllReadReceipt{
		ReceiptID: row.CommandID, TenantID: row.TenantID, UserID: row.UserID,
		MarkedCount: row.MarkedCount, ReadAt: row.ReadAt.UTC(),
	}
}
