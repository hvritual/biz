package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/hvritual/biz/internal/access/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm/clause"
	"sort"
)

// Claims and responses share the root transaction with all bootstrap children.
// No pending row may survive a failed root.
type tenantCreationRecord struct {
	Key         string  `gorm:"column:receipt_key;primaryKey;size:64"`
	Fingerprint string  `gorm:"column:fingerprint;size:64;not null"`
	Payload     *string `gorm:"column:payload;type:mediumtext"`
}

func (tenantCreationRecord) TableName() string { return "biz_tenant_creation_receipts" }
func (r *TenantRepository) ClaimCreation(ctx context.Context, keys []string, fingerprint string) (*domain.Tenant, error) {
	ordered := append([]string(nil), keys...)
	sort.Strings(ordered)
	var replay *domain.Tenant
	for _, key := range ordered {
		inserted := r.database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&tenantCreationRecord{Key: key, Fingerprint: fingerprint})
		if inserted.Error != nil {
			return nil, inserted.Error
		}
		var row tenantCreationRecord
		if err := r.database.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("receipt_key=?", key).First(&row).Error; err != nil {
			return nil, err
		}
		if row.Fingerprint != fingerprint {
			return nil, status.Error(codes.Aborted, "TENANT_CREATION_REQUEST_CONFLICT")
		}
		if row.Payload == nil {
			if inserted.RowsAffected != 1 {
				return nil, errors.New("access: incomplete tenant creation authority")
			}
			continue
		}
		var tenant domain.Tenant
		if err := json.Unmarshal([]byte(*row.Payload), &tenant); err != nil || tenant.ID == "" || tenant.Version == 0 {
			return nil, errors.New("access: corrupt tenant creation receipt")
		}
		if replay != nil && *replay != tenant {
			return nil, errors.New("access: divergent tenant creation receipts")
		}
		replay = &tenant
	}
	return replay, nil
}
func (r *TenantRepository) CompleteCreation(ctx context.Context, keys []string, fingerprint string, tenant domain.Tenant) error {
	data, err := json.Marshal(tenant)
	if err != nil {
		return err
	}
	for _, key := range keys {
		result := r.database.WithContext(ctx).Model(&tenantCreationRecord{}).Where("receipt_key=? AND fingerprint=?", key, fingerprint).Update("payload", string(data))
		if result.Error != nil {
			return result.Error
		}
		// MySQL reports zero for unchanged replay; verify the row still exists.
		var count int64
		if err := r.database.WithContext(ctx).Model(&tenantCreationRecord{}).Where("receipt_key=? AND fingerprint=?", key, fingerprint).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return errors.New("access: tenant creation receipt lease lost")
		}
	}
	return nil
}
