package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const PrivacyConsentLoginSource = "first_party_idp_login"

var (
	ErrPrivacyConsentNotFound        = errors.New("access: privacy consent not found")
	ErrPrivacyConsentVersionMismatch = errors.New("access: privacy consent version mismatch")
)

type privacyConsentRecord struct {
	UserID            string     `gorm:"column:user_id;primaryKey;size:64"`
	AgreementVersion  string     `gorm:"column:agreement_version;primaryKey;size:128"`
	AcceptedAt        time.Time  `gorm:"column:accepted_at;not null;index"`
	Source            string     `gorm:"column:source;size:64;not null"`
	LoginAuditID      uint64     `gorm:"column:login_audit_id;not null;index"`
	AuthorizationHash string     `gorm:"column:authorization_hash;size:64;not null"`
	WithdrawnAt       *time.Time `gorm:"column:withdrawn_at;index"`
	WithdrawnSource   string     `gorm:"column:withdrawn_source;size:64;not null;default:''"`
	CreatedAt         time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;not null"`
}

func (privacyConsentRecord) TableName() string { return "biz_privacy_consents" }

type PrivacyConsent struct {
	UserID            string
	AgreementVersion  string
	AcceptedAt        time.Time
	Source            string
	LoginAuditID      uint64
	AuthorizationHash string
	WithdrawnAt       *time.Time
	WithdrawnSource   string
}

func (store *Store) PrivacyConsentSatisfies(ctx context.Context, userID, agreementVersion string, requireCurrentVersion bool) (bool, error) {
	userID = strings.TrimSpace(userID)
	agreementVersion = strings.TrimSpace(agreementVersion)
	if store == nil || store.database == nil || userID == "" || agreementVersion == "" {
		return false, ErrPrivacyConsentNotFound
	}
	query := store.database.WithContext(ctx).Model(&privacyConsentRecord{}).Where("user_id = ? AND withdrawn_at IS NULL", userID)
	if requireCurrentVersion {
		query = query.Where("agreement_version = ?", agreementVersion)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (store *Store) GetPrivacyConsent(ctx context.Context, userID, agreementVersion string) (PrivacyConsent, error) {
	userID = strings.TrimSpace(userID)
	agreementVersion = strings.TrimSpace(agreementVersion)
	if store == nil || store.database == nil || userID == "" || agreementVersion == "" {
		return PrivacyConsent{}, ErrPrivacyConsentNotFound
	}
	var row privacyConsentRecord
	if err := store.database.WithContext(ctx).Where("user_id = ? AND agreement_version = ?", userID, agreementVersion).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PrivacyConsent{}, ErrPrivacyConsentNotFound
		}
		return PrivacyConsent{}, err
	}
	return privacyConsentFromRecord(row), nil
}

func (store *Store) WithdrawPrivacyConsent(ctx context.Context, userID, agreementVersion, source string) error {
	userID = strings.TrimSpace(userID)
	agreementVersion = strings.TrimSpace(agreementVersion)
	source = strings.TrimSpace(source)
	if store == nil || store.database == nil || userID == "" || agreementVersion == "" || source == "" {
		return ErrPrivacyConsentNotFound
	}
	now := time.Now().UTC()
	result := store.database.WithContext(ctx).Model(&privacyConsentRecord{}).
		Where("user_id = ? AND agreement_version = ? AND withdrawn_at IS NULL", userID, agreementVersion).
		Updates(map[string]any{"withdrawn_at": now, "withdrawn_source": source, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrPrivacyConsentNotFound
	}
	return nil
}

func (store *Store) BindFirstPartyAuthorizationIdentity(ctx context.Context, requestID, browserSecret, csrf, userID string, loginAuditID uint64) error {
	if store == nil || store.database == nil || strings.TrimSpace(userID) == "" || loginAuditID == 0 {
		return ErrFirstPartyIDPFlow
	}
	return store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := loadFirstPartyAuthorizationRequestForUpdate(ctx, tx, requestID, browserSecret, csrf)
		if err != nil {
			return err
		}
		var user userRecord
		if err := tx.Where("id = ? AND status = ?", strings.TrimSpace(userID), "active").First(&user).Error; err != nil {
			return ErrFirstPartyIDPFlow
		}
		var audit firstPartyLoginAuditRecord
		if err := tx.Where("id = ? AND user_id = ? AND outcome = ?", loginAuditID, user.ID, "success").First(&audit).Error; err != nil {
			return ErrFirstPartyIDPFlow
		}
		now := time.Now().UTC()
		return tx.Model(&firstPartyAuthorizationRequestRecord{}).Where("request_hash = ?", row.RequestHash).
			Updates(map[string]any{"authenticated_user_id": user.ID, "login_audit_id": loginAuditID, "authenticated_at": now}).Error
	})
}

func (store *Store) ClearFirstPartyAuthorizationIdentity(ctx context.Context, requestID, browserSecret, csrf string) error {
	if store == nil || store.database == nil {
		return ErrFirstPartyIDPFlow
	}
	return store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := loadFirstPartyAuthorizationRequestForUpdate(ctx, tx, requestID, browserSecret, csrf)
		if err != nil {
			return err
		}
		return tx.Model(&firstPartyAuthorizationRequestRecord{}).Where("request_hash = ?", row.RequestHash).
			Updates(map[string]any{"authenticated_user_id": "", "login_audit_id": 0, "authenticated_at": nil}).Error
	})
}

type AcceptPrivacyConsentInput struct {
	RequestID        string
	BrowserSecret    string
	CSRF             string
	SubmittedVersion string
	ExpectedVersion  string
	Source           string
	CodeTTL          time.Duration
}

func (store *Store) AcceptPrivacyConsentAndIssueAuthorizationCode(ctx context.Context, input AcceptPrivacyConsentInput) (string, FirstPartyAuthorizationRequest, PrivacyConsent, error) {
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.BrowserSecret = strings.TrimSpace(input.BrowserSecret)
	input.CSRF = strings.TrimSpace(input.CSRF)
	input.SubmittedVersion = strings.TrimSpace(input.SubmittedVersion)
	input.ExpectedVersion = strings.TrimSpace(input.ExpectedVersion)
	input.Source = strings.TrimSpace(input.Source)
	if store == nil || store.database == nil || input.CodeTTL <= 0 || input.RequestID == "" || input.BrowserSecret == "" || input.CSRF == "" || input.ExpectedVersion == "" || input.Source == "" {
		return "", FirstPartyAuthorizationRequest{}, PrivacyConsent{}, ErrFirstPartyIDPFlow
	}
	if input.SubmittedVersion != input.ExpectedVersion {
		return "", FirstPartyAuthorizationRequest{}, PrivacyConsent{}, ErrPrivacyConsentVersionMismatch
	}
	code, err := randomWebSecret(32)
	if err != nil {
		return "", FirstPartyAuthorizationRequest{}, PrivacyConsent{}, err
	}
	var request FirstPartyAuthorizationRequest
	var consent PrivacyConsent
	err = store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, err := loadFirstPartyAuthorizationRequestForUpdate(ctx, tx, input.RequestID, input.BrowserSecret, input.CSRF)
		if err != nil {
			return err
		}
		if strings.TrimSpace(row.AuthenticatedUserID) == "" || row.LoginAuditID == 0 || row.AuthenticatedAt == nil {
			return ErrFirstPartyIDPFlow
		}
		var user userRecord
		if err := tx.Where("id = ? AND status = ?", row.AuthenticatedUserID, "active").First(&user).Error; err != nil {
			return ErrFirstPartyIDPFlow
		}
		var audit firstPartyLoginAuditRecord
		if err := tx.Where("id = ? AND user_id = ? AND outcome = ?", row.LoginAuditID, row.AuthenticatedUserID, "success").First(&audit).Error; err != nil {
			return ErrFirstPartyIDPFlow
		}
		now := time.Now().UTC()
		var consentRow privacyConsentRecord
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND agreement_version = ?", row.AuthenticatedUserID, input.ExpectedVersion).First(&consentRow).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			consentRow = privacyConsentRecord{
				UserID: row.AuthenticatedUserID, AgreementVersion: input.ExpectedVersion,
				AcceptedAt: now, Source: input.Source, LoginAuditID: row.LoginAuditID,
				AuthorizationHash: row.RequestHash, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&consentRow).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		case consentRow.WithdrawnAt != nil:
			if err := tx.Model(&privacyConsentRecord{}).Where("user_id = ? AND agreement_version = ?", row.AuthenticatedUserID, input.ExpectedVersion).
				Updates(map[string]any{
					"accepted_at": now, "source": input.Source, "login_audit_id": row.LoginAuditID,
					"authorization_hash": row.RequestHash, "withdrawn_at": nil, "withdrawn_source": "", "updated_at": now,
				}).Error; err != nil {
				return err
			}
			consentRow.AcceptedAt, consentRow.Source, consentRow.LoginAuditID = now, input.Source, row.LoginAuditID
			consentRow.AuthorizationHash, consentRow.WithdrawnAt, consentRow.WithdrawnSource = row.RequestHash, nil, ""
		}
		codeRow := firstPartyAuthorizationCodeRecord{
			CodeHash: TokenHash(code), ClientID: row.ClientID, RedirectURI: row.RedirectURI, UserID: row.AuthenticatedUserID,
			Nonce: row.Nonce, CodeChallenge: row.CodeChallenge, Scope: row.Scope, ExpiresAt: now.Add(input.CodeTTL), CreatedAt: now,
		}
		if err := tx.Create(&codeRow).Error; err != nil {
			return err
		}
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		request = firstPartyRequestFromRecord(row)
		consent = privacyConsentFromRecord(consentRow)
		return nil
	})
	if err != nil {
		return "", FirstPartyAuthorizationRequest{}, PrivacyConsent{}, err
	}
	return code, request, consent, nil
}

func loadFirstPartyAuthorizationRequestForUpdate(ctx context.Context, tx *gorm.DB, requestID, browserSecret, csrf string) (firstPartyAuthorizationRequestRecord, error) {
	if tx == nil || strings.TrimSpace(requestID) == "" || strings.TrimSpace(browserSecret) == "" || strings.TrimSpace(csrf) == "" {
		return firstPartyAuthorizationRequestRecord{}, ErrFirstPartyIDPFlow
	}
	var row firstPartyAuthorizationRequestRecord
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("request_hash = ? AND browser_hash = ?", TokenHash(requestID), TokenHash(browserSecret)).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return firstPartyAuthorizationRequestRecord{}, ErrFirstPartyIDPFlow
		}
		return firstPartyAuthorizationRequestRecord{}, err
	}
	if !row.ExpiresAt.After(time.Now().UTC()) || !constantTimeTokenHashEqual(row.CSRFHash, TokenHash(csrf)) {
		return firstPartyAuthorizationRequestRecord{}, ErrFirstPartyIDPFlow
	}
	return row, nil
}

func privacyConsentFromRecord(row privacyConsentRecord) PrivacyConsent {
	return PrivacyConsent{
		UserID: row.UserID, AgreementVersion: row.AgreementVersion, AcceptedAt: row.AcceptedAt,
		Source: row.Source, LoginAuditID: row.LoginAuditID, AuthorizationHash: row.AuthorizationHash,
		WithdrawnAt: row.WithdrawnAt, WithdrawnSource: row.WithdrawnSource,
	}
}
