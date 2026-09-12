package persistence

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const defaultPBKDF2Iterations = 600_000

var (
	ErrInvalidUserCredentials = errors.New("access: invalid user credentials")
	ErrFirstPartyIDPFlow      = errors.New("access: first-party idp flow invalid")
)

type userPasswordCredentialRecord struct {
	UserID            string    `gorm:"column:user_id;primaryKey;size:64"`
	Salt              string    `gorm:"column:salt;size:128;not null"`
	PasswordHash      string    `gorm:"column:password_hash;size:128;not null"`
	Iterations        int       `gorm:"column:iterations;not null"`
	Disabled          bool      `gorm:"column:disabled;not null;default:false"`
	PasswordChangedAt time.Time `gorm:"column:password_changed_at;not null"`
	UpdatedAt         time.Time `gorm:"column:updated_at;not null"`
}

func (userPasswordCredentialRecord) TableName() string { return "biz_user_password_credentials" }

type firstPartyAuthorizationRequestRecord struct {
	RequestHash   string    `gorm:"column:request_hash;primaryKey;size:64"`
	BrowserHash   string    `gorm:"column:browser_hash;size:64;not null;index"`
	CSRFHash      string    `gorm:"column:csrf_hash;size:64;not null"`
	ClientID      string    `gorm:"column:client_id;size:200;not null"`
	RedirectURI   string    `gorm:"column:redirect_uri;size:1024;not null"`
	State         string    `gorm:"column:state;size:1024;not null"`
	Nonce         string    `gorm:"column:nonce;size:512;not null"`
	CodeChallenge string    `gorm:"column:code_challenge;size:128;not null"`
	Scope         string    `gorm:"column:scope;size:1024;not null"`
	ExpiresAt     time.Time `gorm:"column:expires_at;not null;index"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
}

func (firstPartyAuthorizationRequestRecord) TableName() string {
	return "biz_idp_authorization_requests"
}

type firstPartyAuthorizationCodeRecord struct {
	CodeHash      string    `gorm:"column:code_hash;primaryKey;size:64"`
	ClientID      string    `gorm:"column:client_id;size:200;not null"`
	RedirectURI   string    `gorm:"column:redirect_uri;size:1024;not null"`
	UserID        string    `gorm:"column:user_id;size:64;not null;index"`
	Nonce         string    `gorm:"column:nonce;size:512;not null"`
	CodeChallenge string    `gorm:"column:code_challenge;size:128;not null"`
	Scope         string    `gorm:"column:scope;size:1024;not null"`
	ExpiresAt     time.Time `gorm:"column:expires_at;not null;index"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
}

func (firstPartyAuthorizationCodeRecord) TableName() string { return "biz_idp_authorization_codes" }

type LocalUserIdentity struct {
	UserID string
	Email  string
}

type FirstPartyAuthorizationRequestInput struct {
	ClientID      string
	RedirectURI   string
	State         string
	Nonce         string
	CodeChallenge string
	Scope         string
}

type FirstPartyAuthorizationRequest struct {
	ClientID      string
	RedirectURI   string
	State         string
	Nonce         string
	CodeChallenge string
	Scope         string
}

type FirstPartyAuthorizationGrant struct {
	UserID string
	Email  string
	Nonce  string
	Scope  string
}

func (store *Store) EnsureFirstPartyIDPSchema(ctx context.Context) error {
	if store == nil || store.database == nil {
		return errors.New("access: first-party idp schema store unavailable")
	}
	return store.database.WithContext(ctx).AutoMigrate(
		&userPasswordCredentialRecord{},
		&firstPartyAuthorizationRequestRecord{},
		&firstPartyAuthorizationCodeRecord{},
	)
}

// SetUserPassword attaches a local login credential to an existing active Biz
// user. It does not create users, memberships or roles; the member lifecycle
// remains the account authority.
func (store *Store) SetUserPassword(ctx context.Context, userID, password string) error {
	if store == nil || store.database == nil {
		return ErrInvalidUserCredentials
	}
	return setUserPassword(ctx, store.database, userID, password)
}

func setUserPassword(ctx context.Context, database *gorm.DB, userID, password string) error {
	userID = strings.TrimSpace(userID)
	if database == nil || userID == "" {
		return ErrInvalidUserCredentials
	}
	if len(password) < 12 || len(password) > 1024 {
		return errors.New("access: password must contain between 12 and 1024 bytes")
	}
	var user userRecord
	if err := database.WithContext(ctx).Where("id = ? AND status = ?", userID, "active").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidUserCredentials
		}
		return err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	hash := pbkdf2SHA256([]byte(password), salt, defaultPBKDF2Iterations, 32)
	now := time.Now().UTC()
	record := userPasswordCredentialRecord{
		UserID:            userID,
		Salt:              base64.RawStdEncoding.EncodeToString(salt),
		PasswordHash:      base64.RawStdEncoding.EncodeToString(hash),
		Iterations:        defaultPBKDF2Iterations,
		Disabled:          false,
		PasswordChangedAt: now,
		UpdatedAt:         now,
	}
	return database.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"salt", "password_hash", "iterations", "disabled", "password_changed_at", "updated_at"}),
	}).Create(&record).Error
}

func (store *Store) AuthenticateUserPassword(ctx context.Context, email, password string) (LocalUserIdentity, error) {
	email = strings.TrimSpace(email)
	if store == nil || store.database == nil || email == "" || password == "" {
		consumeDummyPasswordWork(password)
		return LocalUserIdentity{}, ErrInvalidUserCredentials
	}
	var user userRecord
	if err := store.database.WithContext(ctx).Where("LOWER(email) = LOWER(?) AND status = ?", email, "active").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			consumeDummyPasswordWork(password)
			return LocalUserIdentity{}, ErrInvalidUserCredentials
		}
		return LocalUserIdentity{}, err
	}
	var credential userPasswordCredentialRecord
	if err := store.database.WithContext(ctx).Where("user_id = ? AND disabled = ?", user.ID, false).First(&credential).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			consumeDummyPasswordWork(password)
			return LocalUserIdentity{}, ErrInvalidUserCredentials
		}
		return LocalUserIdentity{}, err
	}
	if credential.Iterations < 100_000 || credential.Iterations > 2_000_000 {
		return LocalUserIdentity{}, ErrInvalidUserCredentials
	}
	salt, err := base64.RawStdEncoding.DecodeString(credential.Salt)
	if err != nil || len(salt) < 16 {
		return LocalUserIdentity{}, ErrInvalidUserCredentials
	}
	expected, err := base64.RawStdEncoding.DecodeString(credential.PasswordHash)
	if err != nil || len(expected) != 32 {
		return LocalUserIdentity{}, ErrInvalidUserCredentials
	}
	actual := pbkdf2SHA256([]byte(password), salt, credential.Iterations, len(expected))
	if subtle.ConstantTimeCompare(actual, expected) != 1 {
		return LocalUserIdentity{}, ErrInvalidUserCredentials
	}
	return LocalUserIdentity{UserID: user.ID, Email: user.Email}, nil
}

func consumeDummyPasswordWork(password string) {
	salt := sha256.Sum256([]byte("biz-first-party-idp-dummy-salt-v1"))
	_ = pbkdf2SHA256([]byte(password), salt[:16], defaultPBKDF2Iterations, 32)
}

func (store *Store) CreateFirstPartyAuthorizationRequest(ctx context.Context, input FirstPartyAuthorizationRequestInput, ttl time.Duration) (requestID, browserSecret, csrf string, err error) {
	if store == nil || store.database == nil || ttl <= 0 || strings.TrimSpace(input.ClientID) == "" || strings.TrimSpace(input.RedirectURI) == "" || strings.TrimSpace(input.State) == "" || strings.TrimSpace(input.Nonce) == "" || strings.TrimSpace(input.CodeChallenge) == "" {
		return "", "", "", ErrFirstPartyIDPFlow
	}
	requestID, err = randomWebSecret(32)
	if err != nil {
		return "", "", "", err
	}
	browserSecret, err = randomWebSecret(32)
	if err != nil {
		return "", "", "", err
	}
	csrf, err = randomWebSecret(32)
	if err != nil {
		return "", "", "", err
	}
	now := time.Now().UTC()
	_ = store.database.WithContext(ctx).Where("expires_at <= ?", now).Delete(&firstPartyAuthorizationRequestRecord{}).Error
	record := firstPartyAuthorizationRequestRecord{
		RequestHash: TokenHash(requestID), BrowserHash: TokenHash(browserSecret), CSRFHash: TokenHash(csrf),
		ClientID: input.ClientID, RedirectURI: input.RedirectURI, State: input.State, Nonce: input.Nonce,
		CodeChallenge: input.CodeChallenge, Scope: input.Scope, ExpiresAt: now.Add(ttl), CreatedAt: now,
	}
	if err := store.database.WithContext(ctx).Create(&record).Error; err != nil {
		return "", "", "", err
	}
	return requestID, browserSecret, csrf, nil
}

func (store *Store) LoadFirstPartyAuthorizationRequest(ctx context.Context, requestID, browserSecret, csrf string) (FirstPartyAuthorizationRequest, error) {
	if store == nil || store.database == nil || requestID == "" || browserSecret == "" || csrf == "" {
		return FirstPartyAuthorizationRequest{}, ErrFirstPartyIDPFlow
	}
	var row firstPartyAuthorizationRequestRecord
	if err := store.database.WithContext(ctx).Where("request_hash = ? AND browser_hash = ?", TokenHash(requestID), TokenHash(browserSecret)).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return FirstPartyAuthorizationRequest{}, ErrFirstPartyIDPFlow
		}
		return FirstPartyAuthorizationRequest{}, err
	}
	if !row.ExpiresAt.After(time.Now().UTC()) || !constantTimeTokenHashEqual(row.CSRFHash, TokenHash(csrf)) {
		return FirstPartyAuthorizationRequest{}, ErrFirstPartyIDPFlow
	}
	return firstPartyRequestFromRecord(row), nil
}

func (store *Store) IssueFirstPartyAuthorizationCode(ctx context.Context, requestID, browserSecret, csrf, userID string, ttl time.Duration) (string, FirstPartyAuthorizationRequest, error) {
	if store == nil || store.database == nil || ttl <= 0 || requestID == "" || browserSecret == "" || csrf == "" || strings.TrimSpace(userID) == "" {
		return "", FirstPartyAuthorizationRequest{}, ErrFirstPartyIDPFlow
	}
	code, err := randomWebSecret(32)
	if err != nil {
		return "", FirstPartyAuthorizationRequest{}, err
	}
	var request FirstPartyAuthorizationRequest
	err = store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row firstPartyAuthorizationRequestRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("request_hash = ? AND browser_hash = ?", TokenHash(requestID), TokenHash(browserSecret)).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFirstPartyIDPFlow
			}
			return err
		}
		if !row.ExpiresAt.After(time.Now().UTC()) || !constantTimeTokenHashEqual(row.CSRFHash, TokenHash(csrf)) {
			return ErrFirstPartyIDPFlow
		}
		var user userRecord
		if err := tx.Where("id = ? AND status = ?", userID, "active").First(&user).Error; err != nil {
			return ErrFirstPartyIDPFlow
		}
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		codeRow := firstPartyAuthorizationCodeRecord{
			CodeHash: TokenHash(code), ClientID: row.ClientID, RedirectURI: row.RedirectURI, UserID: userID,
			Nonce: row.Nonce, CodeChallenge: row.CodeChallenge, Scope: row.Scope, ExpiresAt: now.Add(ttl), CreatedAt: now,
		}
		if err := tx.Create(&codeRow).Error; err != nil {
			return err
		}
		request = firstPartyRequestFromRecord(row)
		return nil
	})
	if err != nil {
		return "", FirstPartyAuthorizationRequest{}, err
	}
	return code, request, nil
}

func (store *Store) ConsumeFirstPartyAuthorizationCode(ctx context.Context, code, clientID, redirectURI, verifier string) (FirstPartyAuthorizationGrant, error) {
	if store == nil || store.database == nil || code == "" || clientID == "" || redirectURI == "" || verifier == "" {
		return FirstPartyAuthorizationGrant{}, ErrFirstPartyIDPFlow
	}
	var grant FirstPartyAuthorizationGrant
	err := store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row firstPartyAuthorizationCodeRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code_hash = ?", TokenHash(code)).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFirstPartyIDPFlow
			}
			return err
		}
		if !row.ExpiresAt.After(time.Now().UTC()) || row.ClientID != clientID || row.RedirectURI != redirectURI || !constantTimeTokenHashEqual(TokenHash(row.CodeChallenge), TokenHash(firstPartyPKCEChallenge(verifier))) {
			return ErrFirstPartyIDPFlow
		}
		var user userRecord
		if err := tx.Where("id = ? AND status = ?", row.UserID, "active").First(&user).Error; err != nil {
			return ErrFirstPartyIDPFlow
		}
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		grant = FirstPartyAuthorizationGrant{UserID: user.ID, Email: user.Email, Nonce: row.Nonce, Scope: row.Scope}
		return nil
	})
	return grant, err
}

func firstPartyRequestFromRecord(row firstPartyAuthorizationRequestRecord) FirstPartyAuthorizationRequest {
	return FirstPartyAuthorizationRequest{
		ClientID:      row.ClientID,
		RedirectURI:   row.RedirectURI,
		State:         row.State,
		Nonce:         row.Nonce,
		CodeChallenge: row.CodeChallenge,
		Scope:         row.Scope,
	}
}

func firstPartyPKCEChallenge(verifier string) string {
	digest := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func constantTimeTokenHashEqual(left, right string) bool {
	if len(left) == 0 || len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLength int) []byte {
	if iterations <= 0 || keyLength <= 0 {
		return nil
	}
	const digestLength = 32
	blocks := (keyLength + digestLength - 1) / digestLength
	result := make([]byte, 0, blocks*digestLength)
	counter := make([]byte, 4)
	for block := 1; block <= blocks; block++ {
		binary.BigEndian.PutUint32(counter, uint32(block))
		mac := hmac.New(sha256.New, password)
		_, _ = mac.Write(salt)
		_, _ = mac.Write(counter)
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			_, _ = mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		result = append(result, t...)
	}
	return result[:keyLength]
}
