package persistence

import (
	"context"
	"strings"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
)

var _ ports.OptionalNotificationDeliveryAdmitter = (*Store)(nil)

func (store *Store) PrepareOptionalNotificationDelivery(
	ctx context.Context,
	owner domain.NotificationPreferenceOwner,
	channel domain.NotificationPreferenceChannel,
) (domain.OptionalNotificationDeliveryAdmission, error) {
	if !channel.Valid() {
		return domain.OptionalNotificationDeliveryAdmission{}, domain.ErrNotificationPreferenceInvalid
	}
	result := domain.OptionalNotificationDeliveryAdmission{
		NotificationPreferenceOwner: owner,
		Channel:                     channel,
	}
	err := store.preferenceTransaction(ctx, owner, nil, func(tx *gorm.DB) error {
		preference, err := readNotificationPreference(tx, owner, channel)
		if err != nil {
			return err
		}
		allowed, err := preference.OptionalAllowed()
		if err != nil {
			return err
		}
		result.Allowed = allowed
		result.PreferenceVersion = preference.Version
		if !allowed {
			return nil
		}

		var member membershipRecord
		if err := tx.Where("tenant_id = ? AND user_id = ?", owner.TenantID, owner.UserID).Take(&member).Error; err != nil {
			return notificationPreferenceOwnerError(err)
		}
		destination, err := store.notificationDeliveryDestination(ctx, tx, member, channel)
		if err != nil {
			return err
		}
		if strings.TrimSpace(destination) == "" {
			return domain.ErrNotificationDeliveryContactUnavailable
		}
		result.Destination = destination
		return nil
	})
	if err != nil {
		return domain.OptionalNotificationDeliveryAdmission{}, err
	}
	return result, nil
}

func (store *Store) notificationDeliveryDestination(
	ctx context.Context,
	tx *gorm.DB,
	member membershipRecord,
	channel domain.NotificationPreferenceChannel,
) (string, error) {
	switch channel {
	case domain.NotificationPreferenceEmail:
		if strings.TrimSpace(member.EmailCiphertext) != "" || strings.TrimSpace(member.EmailKeyVersion) != "" || member.EmailLookupHash != nil {
			if store == nil || store.contactProtection == nil {
				return "", ErrSensitiveDataKeyUnavailable
			}
			return store.contactProtection.DecryptEmail(member.EmailCiphertext, member.EmailKeyVersion)
		}
		if strings.TrimSpace(member.Email) != "" {
			return NormalizeEmail(member.Email)
		}
		var account userRecord
		if err := tx.WithContext(ctx).Where("id = ?", member.UserID).Take(&account).Error; err != nil {
			return "", notificationPreferenceOwnerError(err)
		}
		return store.userEmail(account)
	case domain.NotificationPreferenceSMS:
		if strings.TrimSpace(member.PhoneCiphertext) != "" || strings.TrimSpace(member.PhoneKeyVersion) != "" || member.PhoneLookupHash != nil {
			if store == nil || store.contactProtection == nil {
				return "", ErrSensitiveDataKeyUnavailable
			}
			return store.contactProtection.DecryptPhone(member.PhoneCiphertext, member.PhoneKeyVersion)
		}
		if strings.TrimSpace(member.Phone) == "" {
			return "", domain.ErrNotificationDeliveryContactUnavailable
		}
		return NormalizePhone(member.Phone)
	default:
		return "", domain.ErrNotificationPreferenceInvalid
	}
}
