package persistence

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ContactBackfillResult struct {
	Users       uint64
	Memberships uint64
	Phones      uint64
}

func (store *Store) BackfillLegacyContacts(ctx context.Context, limit int) (ContactBackfillResult, error) {
	if store == nil || store.database == nil || store.contactProtection == nil {
		return ContactBackfillResult{}, ErrSensitiveDataKeyUnavailable
	}
	if limit <= 0 || limit > 1000 {
		return ContactBackfillResult{}, errors.New("access: legacy contact backfill limit must be between 1 and 1000")
	}
	var result ContactBackfillResult
	err := store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var users []userRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("(email_ciphertext IS NULL OR email_ciphertext = '') AND email <> '' AND email NOT LIKE ?", protectedEmailPrefix+"%").
			Order("id ASC").Limit(limit).Find(&users).Error; err != nil {
			return err
		}
		for index := range users {
			email, err := NormalizeEmail(users[index].Email)
			if err != nil {
				return err
			}
			if err := store.protectLegacyUserEmail(ctx, tx, &users[index], email); err != nil {
				return err
			}
			result.Users++
		}

		var memberships []membershipRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("(email_ciphertext IS NULL OR email_ciphertext = '') OR ((phone_ciphertext IS NULL OR phone_ciphertext = '') AND phone <> '')").
			Order("tenant_id ASC, user_id ASC").Limit(limit).Find(&memberships).Error; err != nil {
			return err
		}
		for _, member := range memberships {
			updates := map[string]any{}
			if strings.TrimSpace(member.EmailCiphertext) == "" {
				contactEmail := strings.TrimSpace(member.Email)
				if contactEmail == "" {
					var account userRecord
					if err := tx.Where("id = ?", member.UserID).First(&account).Error; err != nil {
						return err
					}
					var err error
					contactEmail, err = store.userEmail(account)
					if err != nil {
						return err
					}
				}
				ciphertext, lookup, version, err := store.contactProtection.ProtectEmail(contactEmail)
				if err != nil {
					return err
				}
				updates["email"] = ""
				updates["email_ciphertext"] = ciphertext
				updates["email_lookup_hash"] = lookup
				updates["email_key_version"] = version
				result.Memberships++
			}
			if strings.TrimSpace(member.PhoneCiphertext) == "" && strings.TrimSpace(member.Phone) != "" {
				ciphertext, lookup, version, err := store.contactProtection.ProtectPhone(member.Phone)
				if err != nil {
					return err
				}
				updates["phone"] = ""
				updates["phone_ciphertext"] = ciphertext
				updates["phone_lookup_hash"] = lookup
				updates["phone_key_version"] = version
				result.Phones++
			}
			if len(updates) == 0 {
				continue
			}
			if err := tx.Model(&membershipRecord{}).Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}
