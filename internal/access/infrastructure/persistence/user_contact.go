package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	protectedEmailPrefix = "protected:"
	absentEmailPrefix    = "absent:"
)

func (store *Store) newUserRecord(id, email string, now time.Time) (userRecord, error) {
	if strings.TrimSpace(email) == "" {
		return userRecord{}, ErrInvalidContact
	}
	return store.newUserRecordWithOptionalEmail(id, email, now)
}

func (store *Store) newUserRecordWithOptionalEmail(id, email string, now time.Time) (userRecord, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return userRecord{}, ErrInvalidContact
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return userRecord{ID: id, Email: absentEmailPrefix + id, Status: "active", CreatedAt: now}, nil
	}
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return userRecord{}, err
	}
	row := userRecord{ID: id, Email: normalized, Status: "active", CreatedAt: now}
	if store == nil || store.contactProtection == nil {
		return row, nil
	}
	ciphertext, lookup, version, err := store.contactProtection.ProtectEmail(normalized)
	if err != nil {
		return userRecord{}, err
	}
	row.Email = protectedEmailPrefix + lookup
	row.EmailCiphertext = ciphertext
	row.EmailLookupHash = &lookup
	row.EmailKeyVersion = version
	return row, nil
}

func (store *Store) userEmail(row userRecord) (string, error) {
	if strings.HasPrefix(strings.TrimSpace(row.Email), absentEmailPrefix) {
		return "", nil
	}
	if strings.TrimSpace(row.EmailCiphertext) != "" || strings.TrimSpace(row.EmailKeyVersion) != "" || row.EmailLookupHash != nil {
		if store == nil || store.contactProtection == nil {
			return "", ErrSensitiveDataKeyUnavailable
		}
		return store.contactProtection.DecryptEmail(row.EmailCiphertext, row.EmailKeyVersion)
	}
	return NormalizeEmail(row.Email)
}

func (store *Store) displayUserEmail(row userRecord) (string, error) {
	email, err := store.userEmail(row)
	if err != nil {
		return "", err
	}
	if email == "" {
		return "", nil
	}
	if store != nil && store.contactProtection != nil {
		return MaskEmail(email), nil
	}
	return email, nil
}

func (store *Store) findUserByEmail(ctx context.Context, db *gorm.DB, email string, activeOnly bool) (userRecord, string, error) {
	normalized, err := NormalizeEmail(email)
	if err != nil {
		return userRecord{}, "", err
	}
	if db == nil {
		return userRecord{}, "", errors.New("access: user lookup database unavailable")
	}
	query := db.WithContext(ctx)
	if store != nil && store.contactProtection != nil {
		lookup, err := store.contactProtection.LookupEmail(normalized)
		if err != nil {
			return userRecord{}, "", err
		}
		var protected userRecord
		protectedQuery := query.Where("email_lookup_hash = ?", lookup)
		if activeOnly {
			protectedQuery = protectedQuery.Where("status = ?", "active")
		}
		if err := protectedQuery.First(&protected).Error; err == nil {
			plain, err := store.userEmail(protected)
			return protected, plain, err
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return userRecord{}, "", err
		}
	}
	var legacy userRecord
	legacyQuery := query.Where("LOWER(email) = LOWER(?)", normalized)
	if activeOnly {
		legacyQuery = legacyQuery.Where("status = ?", "active")
	}
	if err := legacyQuery.First(&legacy).Error; err != nil {
		return userRecord{}, "", err
	}
	plain, err := store.userEmail(legacy)
	return legacy, plain, err
}

func (store *Store) protectLegacyUserEmail(ctx context.Context, db *gorm.DB, row *userRecord, email string) error {
	if store == nil || store.contactProtection == nil || row == nil || db == nil {
		return ErrSensitiveDataKeyUnavailable
	}
	if strings.TrimSpace(row.EmailCiphertext) != "" {
		return nil
	}
	ciphertext, lookup, version, err := store.contactProtection.ProtectEmail(email)
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).Model(&userRecord{}).Where("id = ?", row.ID).Updates(map[string]any{
		"email":             protectedEmailPrefix + lookup,
		"email_ciphertext":  ciphertext,
		"email_lookup_hash": lookup,
		"email_key_version": version,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("access: protected user email backfill lost target")
	}
	row.Email = protectedEmailPrefix + lookup
	row.EmailCiphertext = ciphertext
	row.EmailLookupHash = &lookup
	row.EmailKeyVersion = version
	return nil
}
