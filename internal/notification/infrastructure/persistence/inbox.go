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

type inboxMarkAllRecord struct {
	CommandID   string    `gorm:"column:command_id;primaryKey;size:64"`
	TenantID    string    `gorm:"column:tenant_id;type:varbinary(64);not null;index:idx_notification_inbox_command_owner,priority:1"`
	UserID      string    `gorm:"column:user_id;type:varbinary(64);not null;index:idx_notification_inbox_command_owner,priority:2"`
	MarkedCount uint64    `gorm:"column:marked_count;not null;default:0"`
	ReadAt      time.Time `gorm:"column:read_at;type:datetime(6);not null"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime(6);not null"`
}

func (inboxMarkAllRecord) TableName() string { return "biz_notification_inbox_mark_all" }

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
	var receipt domain.MarkAllReadReceipt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing inboxMarkAllRecord
		err := tx.WithContext(ctx).Where("command_id=?", commandID).Take(&existing).Error
		if err == nil {
			if existing.TenantID != owner.TenantID || existing.UserID != owner.UserID {
				return domain.ErrInboxInvalid
			}
			receipt = markAllReceipt(existing)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Capture the exact unread set before creating the durable command fact.
		// Only these message IDs may be changed by this logical request. A new
		// notification committed after this read is therefore outside the old
		// request even if the client later retries with the same idempotency key.
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
		command := inboxMarkAllRecord{CommandID: commandID, TenantID: owner.TenantID, UserID: owner.UserID, ReadAt: now, CreatedAt: now}
		created := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&command)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			// A concurrent retry won the same idempotency key. The unique insert
			// waits for that transaction; this locking read then observes its
			// committed receipt without applying this transaction's captured set.
			if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("command_id=?", commandID).Take(&existing).Error; err != nil {
				return err
			}
			if existing.TenantID != owner.TenantID || existing.UserID != owner.UserID {
				return domain.ErrInboxInvalid
			}
			receipt = markAllReceipt(existing)
			return nil
		}

		const markAllBatchSize = 500
		for start := 0; start < len(messageIDs); start += markAllBatchSize {
			end := min(start+markAllBatchSize, len(messageIDs))
			updated := tx.WithContext(ctx).Model(&inAppRecord{}).
				Where("tenant_id=? AND user_id=? AND read_at IS NULL AND message_id IN ?", owner.TenantID, owner.UserID, messageIDs[start:end]).
				Update("read_at", now)
			if updated.Error != nil {
				return updated.Error
			}
			command.MarkedCount += uint64(updated.RowsAffected)
		}
		if len(messageIDs) > 0 {
			if err := tx.WithContext(ctx).Model(&inboxMarkAllRecord{}).Where("command_id=?", commandID).
				Update("marked_count", command.MarkedCount).Error; err != nil {
				return err
			}
		}
		receipt = markAllReceipt(command)
		return nil
	})
	if err != nil {
		return domain.MarkAllReadReceipt{}, err
	}
	return receipt, nil
}

func markAllReceipt(row inboxMarkAllRecord) domain.MarkAllReadReceipt {
	return domain.MarkAllReadReceipt{
		ReceiptID: row.CommandID, TenantID: row.TenantID, UserID: row.UserID,
		MarkedCount: row.MarkedCount, ReadAt: row.ReadAt.UTC(),
	}
}
