package persistence

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type roleCreationRecord struct {
	Key         string  `gorm:"column:receipt_key;primaryKey;size:64"`
	TenantID    string  `gorm:"column:tenant_id;size:64;not null"`
	Fingerprint string  `gorm:"column:fingerprint;size:64;not null"`
	Payload     *string `gorm:"column:payload;type:mediumtext"`
}

func (roleCreationRecord) TableName() string { return "biz_role_creation_receipts" }

// The repository factory lends the root transaction; this method never starts
// a separate transaction and never mutates a role while replaying its receipt.
func (repository *TenantRoleRepository) CreateOnce(ctx context.Context, role *domain.Role, key, fingerprint string) (domain.Role, error) {
	if repository == nil || repository.database == nil || role == nil || role.TenantID == "" || len(key) != 64 || len(fingerprint) != 64 {
		return domain.Role{}, errors.New("access: invalid role creation receipt")
	}
	db := repository.database.WithContext(ctx)
	if _, ok := db.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return domain.Role{}, errors.New("access: role creation requires root transaction")
	}
	inserted := int64(0)
	if !ports.IsRoleCreationReplay(ctx) {
		result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&roleCreationRecord{Key: key, TenantID: role.TenantID, Fingerprint: fingerprint})
		if result.Error != nil {
			return domain.Role{}, result.Error
		}
		inserted = result.RowsAffected
	}
	var row roleCreationRecord
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("receipt_key=?", key).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Role{}, ports.ErrTenantRoleConflict
		}
		return domain.Role{}, err
	}
	if row.TenantID != role.TenantID || row.Fingerprint != fingerprint {
		return domain.Role{}, ports.ErrTenantRoleConflict
	}
	if row.Payload != nil {
		var original domain.Role
		if err := json.Unmarshal([]byte(*row.Payload), &original); err != nil || original.ID == "" || original.TenantID != role.TenantID || original.Version != 1 || original.System || len(original.Permissions) != 0 {
			return domain.Role{}, errors.New("access: corrupt role creation receipt")
		}
		return original, nil
	}
	if inserted != 1 || ports.IsRoleCreationReplay(ctx) {
		return domain.Role{}, errors.New("access: incomplete role creation receipt")
	}
	if err := repository.Create(ctx, role); err != nil {
		return domain.Role{}, err
	}
	payload, err := json.Marshal(role)
	if err != nil {
		return domain.Role{}, err
	}
	result := db.Model(&roleCreationRecord{}).Where("receipt_key=? AND tenant_id=? AND fingerprint=? AND payload IS NULL", key, role.TenantID, fingerprint).Update("payload", string(payload))
	if result.Error != nil {
		return domain.Role{}, result.Error
	}
	if result.RowsAffected != 1 {
		return domain.Role{}, errors.New("access: role creation receipt lease lost")
	}
	return *role, nil
}
