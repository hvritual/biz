package persistence

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FirstPartyLoginPolicy struct {
	MaxFailures int
	Window      time.Duration
	Lockout     time.Duration
}

func DefaultFirstPartyLoginPolicy() FirstPartyLoginPolicy {
	return FirstPartyLoginPolicy{MaxFailures: 5, Window: 15 * time.Minute, Lockout: 15 * time.Minute}
}

func (policy FirstPartyLoginPolicy) Validate() error {
	if policy.MaxFailures < 3 || policy.MaxFailures > 100 || policy.Window <= 0 || policy.Lockout <= 0 {
		return errors.New("access: invalid first-party login policy")
	}
	return nil
}

type firstPartyLoginThrottleRecord struct {
	IdentityHash    string     `gorm:"column:identity_hash;primaryKey;size:64"`
	FailureCount    int        `gorm:"column:failure_count;not null"`
	WindowStartedAt time.Time  `gorm:"column:window_started_at;not null"`
	BlockedUntil    *time.Time `gorm:"column:blocked_until;index"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null"`
}

func (firstPartyLoginThrottleRecord) TableName() string { return "biz_idp_login_throttles" }

type firstPartyLoginAuditRecord struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	OccurredAt time.Time `gorm:"column:occurred_at;not null;index"`
	Outcome    string    `gorm:"column:outcome;size:32;not null;index"`
	UserID     string    `gorm:"column:user_id;size:64;index"`
	EmailHash  string    `gorm:"column:email_hash;size:64;not null;index"`
	SourceHash string    `gorm:"column:source_hash;size:64;not null"`
}

func (firstPartyLoginAuditRecord) TableName() string { return "biz_idp_login_audit" }

func (store *Store) EnsureFirstPartyIDPSecuritySchema(ctx context.Context) error {
	if store == nil || store.database == nil {
		return errors.New("access: first-party idp security store unavailable")
	}
	return store.database.WithContext(ctx).AutoMigrate(&firstPartyLoginThrottleRecord{}, &firstPartyLoginAuditRecord{})
}

func (store *Store) AuthenticateFirstPartyLogin(ctx context.Context, email, password, remoteAddr string, policy FirstPartyLoginPolicy) (LocalUserIdentity, error) {
	if err := policy.Validate(); err != nil {
		return LocalUserIdentity{}, err
	}
	email = strings.TrimSpace(email)
	identityHash := TokenHash(strings.ToLower(email))
	sourceHash := TokenHash(normalizeRemoteHost(remoteAddr))
	now := time.Now().UTC()
	err := store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row firstPartyLoginThrottleRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("identity_hash = ?", identityHash).First(&row).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && row.BlockedUntil != nil && row.BlockedUntil.After(now) {
			return ErrInvalidUserCredentials
		}
		return nil
	})
	if err != nil {
		if !errors.Is(err, ErrInvalidUserCredentials) {
			return LocalUserIdentity{}, err
		}
		consumeDummyPasswordWork(password)
		_ = store.recordFirstPartyLoginAudit(ctx, now, "throttled", "", identityHash, sourceHash)
		return LocalUserIdentity{}, ErrInvalidUserCredentials
	}

	identity, authErr := store.AuthenticateUserPassword(ctx, email, password)
	if authErr != nil {
		if err := store.recordFirstPartyLoginFailure(ctx, identityHash, sourceHash, now, policy); err != nil {
			return LocalUserIdentity{}, err
		}
		return LocalUserIdentity{}, ErrInvalidUserCredentials
	}
	if err := store.recordFirstPartyLoginSuccess(ctx, identityHash, sourceHash, identity.UserID, now); err != nil {
		return LocalUserIdentity{}, err
	}
	return identity, nil
}

func (store *Store) recordFirstPartyLoginFailure(ctx context.Context, identityHash, sourceHash string, now time.Time, policy FirstPartyLoginPolicy) error {
	return store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row firstPartyLoginThrottleRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("identity_hash = ?", identityHash).First(&row).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			row = firstPartyLoginThrottleRecord{IdentityHash: identityHash, FailureCount: 1, WindowStartedAt: now, UpdatedAt: now}
		case err != nil:
			return err
		default:
			if now.Sub(row.WindowStartedAt) >= policy.Window {
				row.FailureCount = 1
				row.WindowStartedAt = now
				row.BlockedUntil = nil
			} else {
				row.FailureCount++
			}
			row.UpdatedAt = now
		}
		if row.FailureCount >= policy.MaxFailures {
			blockedUntil := now.Add(policy.Lockout)
			row.BlockedUntil = &blockedUntil
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "identity_hash"}}, DoUpdates: clause.AssignmentColumns([]string{"failure_count", "window_started_at", "blocked_until", "updated_at"})}).Create(&row).Error; err != nil {
			return err
		}
		return tx.Create(&firstPartyLoginAuditRecord{OccurredAt: now, Outcome: "invalid_credentials", EmailHash: identityHash, SourceHash: sourceHash}).Error
	})
}

func (store *Store) recordFirstPartyLoginSuccess(ctx context.Context, identityHash, sourceHash, userID string, now time.Time) error {
	return store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("identity_hash = ?", identityHash).Delete(&firstPartyLoginThrottleRecord{}).Error; err != nil {
			return err
		}
		return tx.Create(&firstPartyLoginAuditRecord{OccurredAt: now, Outcome: "success", UserID: userID, EmailHash: identityHash, SourceHash: sourceHash}).Error
	})
}

func (store *Store) recordFirstPartyLoginAudit(ctx context.Context, at time.Time, outcome, userID, emailHash, sourceHash string) error {
	return store.database.WithContext(ctx).Create(&firstPartyLoginAuditRecord{OccurredAt: at, Outcome: outcome, UserID: userID, EmailHash: emailHash, SourceHash: sourceHash}).Error
}

func (store *Store) DisableUserPassword(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if store == nil || store.database == nil || userID == "" {
		return ErrInvalidUserCredentials
	}
	return store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&userPasswordCredentialRecord{}).Where("user_id = ?", userID).Update("disabled", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrInvalidUserCredentials
		}
		return revokeWebSessionsForUser(ctx, tx, userID)
	})
}

func (store *Store) RotateUserPassword(ctx context.Context, userID, password string) error {
	userID = strings.TrimSpace(userID)
	if store == nil || store.database == nil || userID == "" {
		return ErrInvalidUserCredentials
	}
	return store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := setUserPassword(ctx, tx, userID, password); err != nil {
			return err
		}
		return revokeWebSessionsForUser(ctx, tx, userID)
	})
}

func (store *Store) RevokeWebSessionsForUser(ctx context.Context, userID string) error {
	userID = strings.TrimSpace(userID)
	if store == nil || store.database == nil || userID == "" {
		return nil
	}
	return revokeWebSessionsForUser(ctx, store.database, userID)
}

func revokeWebSessionsForUser(ctx context.Context, database *gorm.DB, userID string) error {
	if database == nil {
		return errors.New("access: web session revoke store unavailable")
	}
	now := time.Now().UTC()
	return database.WithContext(ctx).Exec(`
UPDATE biz_web_sessions s
JOIN biz_web_identities i ON i.issuer = s.issuer AND i.subject = s.subject
SET s.revoked_at = ?, s.updated_at = ?
WHERE i.actor_kind = ? AND i.actor_id = ? AND s.revoked_at IS NULL`, now, now, WebActorUser, userID).Error
}

func normalizeRemoteHost(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return strings.TrimSpace(host)
	}
	return remoteAddr
}
