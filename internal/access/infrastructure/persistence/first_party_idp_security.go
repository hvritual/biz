package persistence

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
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

func (store *Store) AuthenticateFirstPartyLogin(ctx context.Context, identifier, password, remoteAddr string, policy FirstPartyLoginPolicy) (LocalUserIdentity, error) {
	identity, _, err := store.AuthenticateFirstPartyLoginWithAudit(ctx, identifier, password, remoteAddr, policy)
	return identity, err
}

func (store *Store) AuthenticateFirstPartyLoginWithAudit(ctx context.Context, identifier, password, remoteAddr string, policy FirstPartyLoginPolicy) (LocalUserIdentity, uint64, error) {
	return store.authenticateFirstPartyLoginWithAudit(ctx, identifier, password, remoteAddr, policy, nil, "")
}

func (store *Store) AuthenticateFirstPartyLoginWithAuditAndLockNotification(
	ctx context.Context,
	identifier, password, remoteAddr string,
	policy FirstPartyLoginPolicy,
	protection *VerificationProtection,
	flowID string,
) (LocalUserIdentity, uint64, error) {
	return store.authenticateFirstPartyLoginWithAudit(ctx, identifier, password, remoteAddr, policy, protection, flowID)
}

func (store *Store) authenticateFirstPartyLoginWithAudit(
	ctx context.Context,
	identifier, password, remoteAddr string,
	policy FirstPartyLoginPolicy,
	protection *VerificationProtection,
	flowID string,
) (LocalUserIdentity, uint64, error) {
	if store == nil || store.database == nil {
		return LocalUserIdentity{}, 0, ErrInvalidUserCredentials
	}
	if err := policy.Validate(); err != nil {
		return LocalUserIdentity{}, 0, err
	}
	identifier = strings.TrimSpace(identifier)
	flowID = strings.TrimSpace(flowID)
	identityHash := LoginIdentifierThrottleHash(identifier)
	sourceHash := TokenHash(normalizeRemoteHost(remoteAddr))
	now := time.Now().UTC()

	var notificationTarget *LoginIdentifierResolution
	if protection != nil && flowID != "" {
		resolved, resolveErr := store.ResolveLoginIdentifier(ctx, identifier)
		switch {
		case resolveErr == nil && resolved.Identity.UserID != "" && resolved.OTPDestination != "":
			notificationTarget = &resolved
		case resolveErr != nil && !errors.Is(resolveErr, ErrInvalidUserCredentials):
			return LocalUserIdentity{}, 0, resolveErr
		}
	}

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
			return LocalUserIdentity{}, 0, err
		}
		consumeDummyPasswordWork(password)
		_ = store.recordFirstPartyLoginAudit(ctx, now, "throttled", "", identityHash, sourceHash)
		return LocalUserIdentity{}, 0, ErrInvalidUserCredentials
	}

	identity, authErr := store.AuthenticateUserPassword(ctx, identifier, password)
	if authErr != nil {
		if err := store.recordFirstPartyLoginFailure(ctx, identityHash, sourceHash, now, policy, protection, flowID, notificationTarget); err != nil {
			return LocalUserIdentity{}, 0, err
		}
		return LocalUserIdentity{}, 0, ErrInvalidUserCredentials
	}
	auditID, err := store.recordFirstPartyLoginSuccess(ctx, identityHash, sourceHash, identity.UserID, now)
	if err != nil {
		return LocalUserIdentity{}, 0, err
	}
	return identity, auditID, nil
}

func (store *Store) recordFirstPartyLoginFailure(
	ctx context.Context,
	identityHash, sourceHash string,
	now time.Time,
	policy FirstPartyLoginPolicy,
	protection *VerificationProtection,
	flowID string,
	notificationTarget *LoginIdentifierResolution,
) error {
	return store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row firstPartyLoginThrottleRecord
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("identity_hash = ?", identityHash).First(&row).Error
		wasBlocked := false
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			row = firstPartyLoginThrottleRecord{IdentityHash: identityHash, FailureCount: 1, WindowStartedAt: now, UpdatedAt: now}
		case err != nil:
			return err
		default:
			wasBlocked = row.BlockedUntil != nil && row.BlockedUntil.After(now)
			if now.Sub(row.WindowStartedAt) >= policy.Window {
				row.FailureCount = 1
				row.WindowStartedAt = now
				row.BlockedUntil = nil
				wasBlocked = false
			} else {
				row.FailureCount++
			}
			row.UpdatedAt = now
		}
		if row.FailureCount >= policy.MaxFailures && !wasBlocked {
			blockedUntil := now.Add(policy.Lockout)
			row.BlockedUntil = &blockedUntil
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "identity_hash"}}, DoUpdates: clause.AssignmentColumns([]string{"failure_count", "window_started_at", "blocked_until", "updated_at"})}).Create(&row).Error; err != nil {
			return err
		}
		if err := tx.Create(&firstPartyLoginAuditRecord{OccurredAt: now, Outcome: "invalid_credentials", EmailHash: identityHash, SourceHash: sourceHash}).Error; err != nil {
			return err
		}
		if notificationTarget != nil && notificationTarget.Identity.UserID != "" {
			if _, err := AppendTrustedUserAuditPairsTx(ctx, tx, notificationTarget.Identity.UserID, TrustedAudit{
				EventKey: "login-failure/" + identityHash + "/" + fmt.Sprint(now.UnixNano()),
				OperationID: "identity.login.password", Module: "access",
				ActorSubject: "account:" + notificationTarget.Identity.UserID, ActorUserID: notificationTarget.Identity.UserID,
				AuthMethod: "password", AuthChannel: "first-party-idp",
				Target: "account:" + notificationTarget.Identity.UserID,
				DecisionReason: "INVALID_CREDENTIALS", RequestDigest: identityHash,
				Risk: domain.AuditRiskHigh, Outcome: domain.AuditResultFailure, OccurredAt: now,
			}); err != nil {
				return err
			}
		}
		if row.BlockedUntil == nil || wasBlocked || notificationTarget == nil || protection == nil {
			return nil
		}
		verification := &VerificationRepository{database: tx, protection: protection}
		delivery, err := verification.EnqueueSecurityNotification(ctx, domain.SecurityNotificationRequest{
			BusinessEventID: fmt.Sprintf("idp-login-lock/%s/%d", notificationTarget.Identity.UserID, row.BlockedUntil.UTC().Unix()),
			Kind:            domain.SecurityNotificationLoginLock,
			Purpose:         domain.VerificationPurposeLogin,
			UserID:          notificationTarget.Identity.UserID,
			FlowID:          flowID,
			Channel:         notificationTarget.OTPChannel,
			Destination:     notificationTarget.OTPDestination,
			ExpiresAt:       row.BlockedUntil.UTC().Add(24 * time.Hour),
		})
		if err != nil {
			return err
		}
		_, err = AppendTrustedUserAuditPairsTx(ctx, tx, notificationTarget.Identity.UserID, TrustedAudit{
			EventKey: "login-lock/" + notificationTarget.Identity.UserID + "/" + fmt.Sprint(row.BlockedUntil.UTC().Unix()),
			OperationID: "identity.login.lock", Module: "access",
			ActorSubject: "system:login-throttle", AuthMethod: "system", AuthChannel: "first-party-idp",
			Target: "account:" + notificationTarget.Identity.UserID,
			DecisionReason: "FAILURE_THRESHOLD_REACHED", ReceiptRef: delivery.EventID,
			Reason: "changed_fields=login_lock", Risk: domain.AuditRiskHigh,
			Outcome: domain.AuditResultSuccess, OccurredAt: now,
		})
		return err
	})
}

func (store *Store) recordFirstPartyLoginSuccess(ctx context.Context, identityHash, sourceHash, userID string, now time.Time) (uint64, error) {
	var audit firstPartyLoginAuditRecord
	err := store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("identity_hash = ?", identityHash).Delete(&firstPartyLoginThrottleRecord{}).Error; err != nil {
			return err
		}
		audit = firstPartyLoginAuditRecord{OccurredAt: now, Outcome: "success", UserID: userID, EmailHash: identityHash, SourceHash: sourceHash}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}
		_, err := AppendTrustedUserAuditPairsTx(ctx, tx, userID, TrustedAudit{
			EventKey: "login-success/" + fmt.Sprint(audit.ID),
			OperationID: "identity.login.password", Module: "access",
			ActorSubject: "user:" + userID, ActorUserID: userID,
			AuthMethod: "password", AuthChannel: "first-party-idp",
			Target: "account:" + userID, RequestDigest: identityHash,
			ReceiptRef: "login-audit:" + fmt.Sprint(audit.ID),
			Risk: domain.AuditRiskHigh, Outcome: domain.AuditResultSuccess, OccurredAt: now,
		})
		return err
	})
	if err != nil {
		return 0, err
	}
	return audit.ID, nil
}
func (store *Store) recordFirstPartyLoginAudit(ctx context.Context, at time.Time, outcome, userID, emailHash, sourceHash string) error {
	return store.database.WithContext(ctx).Create(&firstPartyLoginAuditRecord{OccurredAt: at, Outcome: outcome, UserID: userID, EmailHash: emailHash, SourceHash: sourceHash}).Error
}

func (store *Store) FirstPartyLoginThrottleState(ctx context.Context, identifier string) (bool, *time.Time, error) {
	if store == nil || store.database == nil {
		return false, nil, nil
	}
	var row firstPartyLoginThrottleRecord
	err := store.database.WithContext(ctx).Where("identity_hash = ?", LoginIdentifierThrottleHash(identifier)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	if row.BlockedUntil == nil || !row.BlockedUntil.After(time.Now().UTC()) {
		return false, row.BlockedUntil, nil
	}
	value := row.BlockedUntil.UTC()
	return true, &value, nil
}

func (store *Store) RecordFirstPartyVerifiedLogin(ctx context.Context, identifier, userID, remoteAddr string) (uint64, error) {
	identifier = strings.TrimSpace(identifier)
	userID = strings.TrimSpace(userID)
	if store == nil || store.database == nil || identifier == "" || userID == "" {
		return 0, ErrInvalidUserCredentials
	}
	return store.recordFirstPartyLoginSuccess(
		ctx,
		LoginIdentifierThrottleHash(identifier),
		TokenHash(normalizeRemoteHost(remoteAddr)),
		userID,
		time.Now().UTC(),
	)
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
	if !database.Migrator().HasTable(&webSessionRecord{}) || !database.Migrator().HasTable(&webIdentityRecord{}) {
		return nil
	}
	now := time.Now().UTC()
	return database.WithContext(ctx).Exec(`
UPDATE biz_web_sessions s
JOIN biz_web_identities i ON i.issuer = s.issuer AND i.subject = s.subject
SET s.revoked_at = ?, s.revoked_reason = ?, s.revoked_scope = ?, s.context_version = s.context_version + 1, s.updated_at = ?
WHERE i.actor_kind = ? AND i.actor_id = ? AND s.revoked_at IS NULL`,
		now, "account_security", "account", now, WebActorUser, userID).Error
}

func normalizeRemoteHost(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return strings.TrimSpace(host)
	}
	return remoteAddr
}
