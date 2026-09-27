package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/hvritual/biz/internal/notification/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/core/identity"
)

type configurationReceiptRecord struct {
	TenantID    string    `gorm:"column:tenant_id;type:varbinary(64);primaryKey"`
	ActorID     string    `gorm:"column:actor_id;type:varbinary(64);primaryKey"`
	Operation   string    `gorm:"column:operation;size:64;primaryKey"`
	KeyHash     string    `gorm:"column:key_hash;size:64;primaryKey"`
	RequestHash string    `gorm:"column:request_hash;size:64;not null"`
	Response    string    `gorm:"column:response;type:longtext;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;not null"`
}

func (configurationReceiptRecord) TableName() string {
	return "biz_notification_configuration_receipts"
}

func receiptKey(ctx context.Context, tenant, actor, operation, key string) (configurationReceiptRecord, error) {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.TenantID != tenant || p.UserID != actor || !domain.ValidConfigurationID(actor, 64) || !domain.ValidConfigurationID(key, 256) {
		return configurationReceiptRecord{}, domain.ErrConfigurationInvalid
	}
	switch operation {
	case "create", "update", "delete":
	default:
		return configurationReceiptRecord{}, domain.ErrConfigurationInvalid
	}
	sum := sha256.Sum256([]byte(key))
	return configurationReceiptRecord{TenantID: tenant, ActorID: actor, Operation: operation, KeyHash: hex.EncodeToString(sum[:]), CreatedAt: time.Now().UTC()}, nil
}

// ClaimReceipt serializes concurrent attempts on the same scoped key inside the
// root transaction. A returned receipt is the original response, never a current
// read of a configuration that another command may since have changed/deleted.
func (r *ConfigurationRepository) ClaimReceipt(ctx context.Context, tenant, actor, operation, key, digest string) (*domain.ConfigurationReceipt, error) {
	tx, err := r.scoped(ctx, tenant)
	if err != nil {
		return nil, err
	}
	row, err := receiptKey(ctx, tenant, actor, operation, key)
	if err != nil {
		return nil, err
	}
	if !exactDigest(digest) {
		return nil, domain.ErrConfigurationInvalid
	}
	row.RequestHash = digest
	if err := tx.Clauses(clause.OnConflict{DoUpdates: clause.Assignments(map[string]any{"key_hash": gorm.Expr("key_hash")})}).Create(&row).Error; err != nil {
		return nil, err
	}
	var stored configurationReceiptRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND actor_id = ? AND operation = ? AND key_hash = ?", row.TenantID, row.ActorID, row.Operation, row.KeyHash).Take(&stored).Error; err != nil {
		return nil, err
	}
	if stored.RequestHash != digest {
		return nil, domain.ErrConfigurationReplayConflict
	}
	if stored.Response == "" {
		return nil, nil
	}
	var receipt domain.ConfigurationReceipt
	if err := json.Unmarshal([]byte(stored.Response), &receipt); err != nil {
		return nil, domain.ErrConfigurationUnavailable
	}
	if receipt.TenantID != tenant || receipt.ReceiptID == "" || len(receipt.Configurations) == 0 {
		return nil, domain.ErrConfigurationUnavailable
	}
	for _, c := range receipt.Configurations {
		if c.TenantID != tenant || c.Version == 0 || !c.Level.Valid() {
			return nil, domain.ErrConfigurationUnavailable
		}
	}
	return &receipt, nil
}
func (r *ConfigurationRepository) CompleteReceipt(ctx context.Context, tenant, actor, operation, key string, receipt domain.ConfigurationReceipt) error {
	tx, err := r.scoped(ctx, tenant)
	if err != nil {
		return err
	}
	row, err := receiptKey(ctx, tenant, actor, operation, key)
	if err != nil {
		return err
	}
	if receipt.TenantID != tenant || receipt.ReceiptID == "" || len(receipt.Configurations) == 0 || len(receipt.Configurations) > 3 {
		return domain.ErrConfigurationInvalid
	}
	for _, c := range receipt.Configurations {
		if c.TenantID != tenant || c.Version == 0 || !c.Level.Valid() {
			return domain.ErrConfigurationInvalid
		}
	}
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	update := tx.Model(&configurationReceiptRecord{}).Where("tenant_id = ? AND actor_id = ? AND operation = ? AND key_hash = ? AND response = ''", row.TenantID, row.ActorID, row.Operation, row.KeyHash).Update("response", string(encoded))
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return errors.New("notification: receipt not claimed or already completed")
	}
	return nil
}
