package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrServiceAPICredentialUnavailable = errors.New("access: service api credential unavailable")
	ErrServiceAPIReplay                = errors.New("access: service api nonce replay")
)

type serviceAPICredentialRecord struct {
	KeyID        string     `gorm:"column:key_id;primaryKey;size:128"`
	Subject      string     `gorm:"column:subject;size:200;not null;index"`
	TenantID     string     `gorm:"column:tenant_id;size:64;not null;default:'';index"`
	SecretDigest string     `gorm:"column:secret_digest;size:64;not null"`
	Disabled     bool       `gorm:"column:disabled;not null;default:false;index"`
	NotBefore    *time.Time `gorm:"column:not_before;type:datetime(6)"`
	ExpiresAt    *time.Time `gorm:"column:expires_at;type:datetime(6);index"`
	CreatedAt    time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;not null"`
}

func (serviceAPICredentialRecord) TableName() string { return "biz_service_api_credentials" }

type serviceAPIOperationRecord struct {
	KeyID     string `gorm:"column:key_id;primaryKey;size:128"`
	Operation string `gorm:"column:operation;primaryKey;size:160"`
}

func (serviceAPIOperationRecord) TableName() string { return "biz_service_api_operations" }

type serviceAPINonceRecord struct {
	KeyID     string    `gorm:"column:key_id;primaryKey;size:128"`
	Nonce     string    `gorm:"column:nonce;primaryKey;size:128"`
	ExpiresAt time.Time `gorm:"column:expires_at;type:datetime(6);not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (serviceAPINonceRecord) TableName() string { return "biz_service_api_nonces" }

type ServiceAPICredentialBootstrap struct {
	KeyID        string
	Subject      string
	TenantID     string
	SecretDigest string
	Operations   []string
	Disabled     bool
	NotBefore    *time.Time
	ExpiresAt    *time.Time
}

type ServiceAPICredential struct {
	KeyID      string
	Subject    string
	TenantID   string
	Operations []string
	Disabled   bool
	NotBefore  *time.Time
	ExpiresAt  *time.Time
}

func ServiceAPISecretDigest(secret []byte) string {
	sum := sha256.Sum256(secret)
	return hex.EncodeToString(sum[:])
}

func (store *Store) BootstrapServiceAPICredentials(ctx context.Context, values []ServiceAPICredentialBootstrap) error {
	if store == nil || store.database == nil {
		return errors.New("access: service api credential store unavailable")
	}
	if len(values) == 0 {
		return nil
	}
	now := time.Now().UTC()
	return store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, value := range values {
			keyID := strings.TrimSpace(value.KeyID)
			subject := strings.TrimSpace(value.Subject)
			digest := strings.TrimSpace(value.SecretDigest)
			if keyID == "" || subject == "" || len(digest) != 64 || len(value.Operations) == 0 {
				return errors.New("access: invalid service api credential bootstrap")
			}
			record := serviceAPICredentialRecord{
				KeyID: keyID, Subject: subject, TenantID: strings.TrimSpace(value.TenantID), SecretDigest: digest,
				Disabled: value.Disabled, NotBefore: copyTimePointer(value.NotBefore), ExpiresAt: copyTimePointer(value.ExpiresAt),
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "key_id"}},
				// A persisted revoke is authority. Reloading trusted process configuration
				// may rotate metadata/secret material, but must never resurrect a disabled key.
				DoUpdates: clause.AssignmentColumns([]string{
					"subject", "tenant_id", "secret_digest", "not_before", "expires_at", "updated_at",
				}),
			}).Create(&record).Error; err != nil {
				return err
			}
			if err := tx.Where("key_id = ?", keyID).Delete(&serviceAPIOperationRecord{}).Error; err != nil {
				return err
			}
			operations := canonicalServiceAPIOperations(value.Operations)
			if len(operations) == 0 {
				return errors.New("access: service api credential operations are required")
			}
			for _, operation := range operations {
				if err := tx.Create(&serviceAPIOperationRecord{KeyID: keyID, Operation: operation}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (store *Store) ResolveServiceAPICredential(ctx context.Context, keyID, secretDigest, operation, tenantID string, now time.Time) (ServiceAPICredential, error) {
	if store == nil || store.database == nil {
		return ServiceAPICredential{}, ErrServiceAPICredentialUnavailable
	}
	keyID = strings.TrimSpace(keyID)
	secretDigest = strings.TrimSpace(secretDigest)
	operation = strings.TrimSpace(operation)
	tenantID = strings.TrimSpace(tenantID)
	if keyID == "" || len(secretDigest) != 64 || operation == "" {
		return ServiceAPICredential{}, ErrUnauthorized
	}
	var row serviceAPICredentialRecord
	if err := store.database.WithContext(ctx).Where("key_id = ?", keyID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ServiceAPICredential{}, ErrUnauthorized
		}
		return ServiceAPICredential{}, ErrServiceAPICredentialUnavailable
	}
	if row.Disabled || row.SecretDigest != secretDigest {
		return ServiceAPICredential{}, ErrUnauthorized
	}
	now = now.UTC()
	if row.NotBefore != nil && now.Before(row.NotBefore.UTC()) {
		return ServiceAPICredential{}, ErrUnauthorized
	}
	if row.ExpiresAt != nil && !now.Before(row.ExpiresAt.UTC()) {
		return ServiceAPICredential{}, ErrUnauthorized
	}
	if row.TenantID != "" && row.TenantID != tenantID {
		return ServiceAPICredential{}, ErrUnauthorized
	}
	var count int64
	if err := store.database.WithContext(ctx).Model(&serviceAPIOperationRecord{}).
		Where("key_id = ? AND operation = ?", keyID, operation).Count(&count).Error; err != nil {
		return ServiceAPICredential{}, ErrServiceAPICredentialUnavailable
	}
	if count != 1 {
		return ServiceAPICredential{}, ErrUnauthorized
	}
	var operationRows []serviceAPIOperationRecord
	if err := store.database.WithContext(ctx).Where("key_id = ?", keyID).Order("operation ASC").Find(&operationRows).Error; err != nil {
		return ServiceAPICredential{}, ErrServiceAPICredentialUnavailable
	}
	operations := make([]string, 0, len(operationRows))
	for _, item := range operationRows {
		operations = append(operations, item.Operation)
	}
	return ServiceAPICredential{
		KeyID: row.KeyID, Subject: row.Subject, TenantID: row.TenantID, Operations: operations,
		Disabled: row.Disabled, NotBefore: copyTimePointer(row.NotBefore), ExpiresAt: copyTimePointer(row.ExpiresAt),
	}, nil
}

func (store *Store) ConsumeServiceAPINonce(ctx context.Context, keyID, nonce string, expiresAt time.Time) error {
	if store == nil || store.database == nil {
		return ErrServiceAPICredentialUnavailable
	}
	keyID = strings.TrimSpace(keyID)
	nonce = strings.TrimSpace(nonce)
	if keyID == "" || nonce == "" {
		return ErrUnauthorized
	}
	now := time.Now().UTC()
	result := store.database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&serviceAPINonceRecord{
		KeyID: keyID, Nonce: nonce, ExpiresAt: expiresAt.UTC(), CreatedAt: now,
	})
	if result.Error != nil {
		return ErrServiceAPICredentialUnavailable
	}
	if result.RowsAffected != 1 {
		return ErrServiceAPIReplay
	}
	_ = store.database.WithContext(ctx).Where("expires_at < ?", now.Add(-time.Hour)).Limit(1000).Delete(&serviceAPINonceRecord{}).Error
	return nil
}

func (store *Store) DisableServiceAPICredential(ctx context.Context, keyID string) error {
	if store == nil || store.database == nil {
		return ErrServiceAPICredentialUnavailable
	}
	result := store.database.WithContext(ctx).Model(&serviceAPICredentialRecord{}).
		Where("key_id = ?", strings.TrimSpace(keyID)).
		Updates(map[string]any{"disabled": true, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrUnauthorized
	}
	return nil
}

func (store *Store) serviceAPIPrincipalAvailable(ctx context.Context, subject string, now time.Time) (bool, error) {
	if store == nil || store.database == nil || strings.TrimSpace(subject) == "" {
		return false, nil
	}
	var count int64
	err := store.database.WithContext(ctx).Model(&serviceAPICredentialRecord{}).
		Where("subject = ? AND disabled = ?", strings.TrimSpace(subject), false).
		Where("(not_before IS NULL OR not_before <= ?) AND (expires_at IS NULL OR expires_at > ?)", now.UTC(), now.UTC()).
		Count(&count).Error
	return count > 0, err
}

func canonicalServiceAPIOperations(values []string) []string {
	unique := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			unique[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(unique))
	for value := range unique {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func copyTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
