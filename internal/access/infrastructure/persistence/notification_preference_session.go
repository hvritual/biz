package persistence

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type notificationPreferenceSession struct {
	store     *Store
	tokenHash string
	expected  WebSessionContext
}

// NotificationPreferencesForWebSession binds the self-service port to an
// authenticated session snapshot. The snapshot is revalidated inside the same
// transaction as the preference read/write, including idempotency replays.
// Callers cannot select a different owner. No raw token is retained.
func (store *Store) NotificationPreferencesForWebSession(raw string, expected WebSessionContext) ports.SelfNotificationPreferences {
	hash := ""
	if raw != "" {
		hash = TokenHash(raw)
	}
	return &notificationPreferenceSession{store: store, tokenHash: hash, expected: expected}
}

func (session *notificationPreferenceSession) ReadNotificationPreferences(ctx context.Context, owner domain.NotificationPreferenceOwner) (domain.NotificationPreferenceSet, error) {
	return session.store.readNotificationPreferences(ctx, owner, session)
}

func (session *notificationPreferenceSession) ChangeNotificationPreference(ctx context.Context, owner domain.NotificationPreferenceOwner, change domain.NotificationPreferenceChange) (domain.NotificationPreferenceReceipt, error) {
	return session.store.changeNotificationPreference(ctx, owner, change, session)
}

func (session *notificationPreferenceSession) validate(tx *gorm.DB, owner domain.NotificationPreferenceOwner) error {
	expected := session.expected
	if session.tokenHash == "" || expected.ActorKind != WebActorUser || expected.ContextVersion == 0 || expected.CSRFToken == "" {
		return ErrWebSessionInvalid
	}
	if owner.TenantID != expected.ActiveTenantID || owner.UserID != expected.UserID {
		return ErrWebSessionChanged
	}
	// Owner locks precede this session lock, matching membership deactivation.
	// SHARE blocks a concurrent switch/revoke without introducing a second
	// preference writer. Expiry is sampled after acquiring the lock.
	var current webSessionRecord
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("token_hash = ?", session.tokenHash).First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrWebSessionInvalid
		}
		return err
	}
	if current.RevokedAt != nil || !current.ExpiresAt.After(time.Now()) {
		return ErrWebSessionInvalid
	}
	if current.ActiveTenantID != expected.ActiveTenantID || current.ContextVersion != expected.ContextVersion ||
		subtle.ConstantTimeCompare([]byte(current.CSRFToken), []byte(expected.CSRFToken)) != 1 {
		return ErrWebSessionChanged
	}
	var identity webIdentityRecord
	if err := tx.Select("actor_kind", "actor_id").Clauses(clause.Locking{Strength: "SHARE"}).
		Where("issuer = ? AND subject = ?", current.Issuer, current.Subject).First(&identity).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrWebSessionInvalid
		}
		return err
	}
	if identity.ActorKind != WebActorUser || identity.ActorID != owner.UserID {
		return ErrWebSessionInvalid
	}
	return nil
}
