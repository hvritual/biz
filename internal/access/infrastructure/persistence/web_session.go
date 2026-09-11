package persistence

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sort"
	"strings"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/core/identity"
)

const (
	WebActorUser     = "user"
	WebActorPlatform = "platform"
	// Current Biz operation contracts classify server-issued credentials as
	// api-key authentication. A browser never receives an API key: after OIDC
	// verification the BFF replaces provider tokens with a server-side session,
	// and only that trusted server session is projected into this existing
	// contract authentication class. Session origin remains explicit in the Web
	// session context and cannot be supplied by an arbitrary header.
	AuthMethodWeb = identity.AuthMethodAPIKey
)

var (
	ErrWebIdentityUnbound = errors.New("access: web identity is not bound")
	ErrWebSessionInvalid  = errors.New("access: web session invalid")
	ErrWebTenantDenied    = errors.New("access: web tenant selection denied")
	ErrWebLoginFlow       = errors.New("access: web login flow invalid")
)

type webIdentityRecord struct {
	Issuer    string    `gorm:"column:issuer;primaryKey;size:512"`
	Subject   string    `gorm:"column:subject;primaryKey;size:255"`
	ActorKind string    `gorm:"column:actor_kind;size:32;not null;index"`
	ActorID   string    `gorm:"column:actor_id;size:200;not null;index"`
	Email     string    `gorm:"column:email;size:320"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (webIdentityRecord) TableName() string { return "biz_web_identities" }

type webSessionRecord struct {
	TokenHash      string     `gorm:"column:token_hash;primaryKey;size:64"`
	Issuer         string     `gorm:"column:issuer;size:512;not null;index:idx_web_session_identity,priority:1"`
	Subject        string     `gorm:"column:subject;size:255;not null;index:idx_web_session_identity,priority:2"`
	ActiveTenantID string     `gorm:"column:active_tenant_id;size:64;index"`
	CSRFToken      string     `gorm:"column:csrf_token;size:128;not null"`
	ExpiresAt      time.Time  `gorm:"column:expires_at;not null;index"`
	RevokedAt      *time.Time `gorm:"column:revoked_at;index"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null"`
}

func (webSessionRecord) TableName() string { return "biz_web_sessions" }

type webLoginFlowRecord struct {
	StateHash    string    `gorm:"column:state_hash;primaryKey;size:64"`
	BrowserHash  string    `gorm:"column:browser_hash;size:64;not null;index"`
	CodeVerifier string    `gorm:"column:code_verifier;size:160;not null"`
	NonceHash    string    `gorm:"column:nonce_hash;size:64;not null"`
	ReturnTo     string    `gorm:"column:return_to;size:1024;not null"`
	ExpiresAt    time.Time `gorm:"column:expires_at;not null;index"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
}

func (webLoginFlowRecord) TableName() string { return "biz_web_login_flows" }

type WebIdentity struct {
	Issuer    string
	Subject   string
	ActorKind string
	ActorID   string
	Email     string
}

type WebTenant struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type WebSessionContext struct {
	ActorKind       string
	UserID          string
	PlatformSubject string
	ActiveTenantID  string
	Tenants         []WebTenant
	ExpiresAt       time.Time
	CSRFToken       string
}

type WebSessionAuthentication struct {
	Principal identity.Principal
	Session   WebSessionContext
}

type WebLoginFlow struct {
	CodeVerifier string
	NonceHash    string
	ReturnTo     string
}

func (store *Store) EnsureWebSessionSchema(ctx context.Context) error {
	if store == nil || store.database == nil {
		return errors.New("access: web session schema store unavailable")
	}
	return store.database.WithContext(ctx).AutoMigrate(&webIdentityRecord{}, &webSessionRecord{}, &webLoginFlowRecord{})
}

func (store *Store) BindOIDCPlatformIdentity(ctx context.Context, issuer, subject, platformSubject, email string) error {
	issuer = strings.TrimSpace(issuer)
	subject = strings.TrimSpace(subject)
	platformSubject = strings.TrimSpace(platformSubject)
	if store == nil || store.database == nil || issuer == "" || subject == "" || platformSubject == "" {
		return ErrWebIdentityUnbound
	}
	if _, err := store.AuthenticatePlatformSubject(ctx, platformSubject, AuthMethodWeb); err != nil {
		return ErrWebIdentityUnbound
	}
	var existing webIdentityRecord
	err := store.database.WithContext(ctx).Where("issuer = ? AND subject = ?", issuer, subject).First(&existing).Error
	if err == nil {
		if existing.ActorKind != WebActorPlatform || existing.ActorID != platformSubject {
			return errors.New("access: OIDC identity already bound to a different actor")
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	now := time.Now().UTC()
	return store.database.WithContext(ctx).Create(&webIdentityRecord{
		Issuer: issuer, Subject: subject, ActorKind: WebActorPlatform, ActorID: platformSubject,
		Email: strings.TrimSpace(email), CreatedAt: now, UpdatedAt: now,
	}).Error
}

func (store *Store) ResolveOrBindOIDCIdentity(ctx context.Context, issuer, subject, email string, emailVerified bool) (WebIdentity, error) {
	issuer = strings.TrimSpace(issuer)
	subject = strings.TrimSpace(subject)
	email = strings.TrimSpace(email)
	if store == nil || store.database == nil || issuer == "" || subject == "" {
		return WebIdentity{}, ErrWebIdentityUnbound
	}
	var link webIdentityRecord
	err := store.database.WithContext(ctx).Where("issuer = ? AND subject = ?", issuer, subject).First(&link).Error
	if err == nil {
		if err := store.validateWebIdentityAuthority(ctx, link); err != nil {
			return WebIdentity{}, err
		}
		return webIdentityFromRecord(link), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return WebIdentity{}, err
	}
	if !emailVerified || email == "" {
		return WebIdentity{}, ErrWebIdentityUnbound
	}
	var users []userRecord
	if err := store.database.WithContext(ctx).
		Where("LOWER(email) = LOWER(?) AND status = ?", email, "active").
		Limit(2).Find(&users).Error; err != nil {
		return WebIdentity{}, err
	}
	if len(users) != 1 {
		return WebIdentity{}, ErrWebIdentityUnbound
	}
	now := time.Now().UTC()
	candidate := webIdentityRecord{
		Issuer: issuer, Subject: subject, ActorKind: WebActorUser, ActorID: users[0].ID,
		Email: email, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
		return WebIdentity{}, err
	}
	if err := store.database.WithContext(ctx).Where("issuer = ? AND subject = ?", issuer, subject).First(&link).Error; err != nil {
		return WebIdentity{}, err
	}
	if err := store.validateWebIdentityAuthority(ctx, link); err != nil {
		return WebIdentity{}, err
	}
	return webIdentityFromRecord(link), nil
}

func (store *Store) validateWebIdentityAuthority(ctx context.Context, link webIdentityRecord) error {
	switch link.ActorKind {
	case WebActorUser:
		var count int64
		if err := store.database.WithContext(ctx).Model(&userRecord{}).
			Where("id = ? AND status = ?", link.ActorID, "active").Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrUnauthorized
		}
		return nil
	case WebActorPlatform:
		_, err := store.AuthenticatePlatformSubject(ctx, link.ActorID, AuthMethodWeb)
		return err
	default:
		return ErrWebIdentityUnbound
	}
}

func webIdentityFromRecord(record webIdentityRecord) WebIdentity {
	return WebIdentity{Issuer: record.Issuer, Subject: record.Subject, ActorKind: record.ActorKind, ActorID: record.ActorID, Email: record.Email}
}

func (store *Store) CreateWebSession(ctx context.Context, webIdentity WebIdentity, ttl time.Duration) (string, WebSessionAuthentication, error) {
	if store == nil || store.database == nil || ttl <= 0 || strings.TrimSpace(webIdentity.Issuer) == "" || strings.TrimSpace(webIdentity.Subject) == "" {
		return "", WebSessionAuthentication{}, ErrWebSessionInvalid
	}
	authoritative, err := store.ResolveOrBindOIDCIdentity(ctx, webIdentity.Issuer, webIdentity.Subject, "", false)
	if err != nil {
		return "", WebSessionAuthentication{}, err
	}
	token, err := randomWebSecret(32)
	if err != nil {
		return "", WebSessionAuthentication{}, err
	}
	csrf, err := randomWebSecret(32)
	if err != nil {
		return "", WebSessionAuthentication{}, err
	}
	now := time.Now().UTC()
	activeTenant := ""
	if authoritative.ActorKind == WebActorUser {
		tenants, err := store.listWebTenants(ctx, authoritative.ActorID)
		if err != nil {
			return "", WebSessionAuthentication{}, err
		}
		if len(tenants) == 1 {
			activeTenant = tenants[0].ID
		}
	}
	record := webSessionRecord{
		TokenHash: TokenHash(token), Issuer: authoritative.Issuer, Subject: authoritative.Subject,
		ActiveTenantID: activeTenant, CSRFToken: csrf, ExpiresAt: now.Add(ttl), CreatedAt: now, UpdatedAt: now,
	}
	if err := store.database.WithContext(ctx).Create(&record).Error; err != nil {
		return "", WebSessionAuthentication{}, err
	}
	authentication, err := store.authenticateWebSessionRecord(ctx, record)
	if err != nil {
		return "", WebSessionAuthentication{}, err
	}
	return token, authentication, nil
}

func (store *Store) AuthenticateWebSession(ctx context.Context, rawToken string) (WebSessionAuthentication, error) {
	if store == nil || store.database == nil || strings.TrimSpace(rawToken) == "" {
		return WebSessionAuthentication{}, ErrWebSessionInvalid
	}
	var record webSessionRecord
	if err := store.database.WithContext(ctx).Where("token_hash = ?", TokenHash(rawToken)).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return WebSessionAuthentication{}, ErrWebSessionInvalid
		}
		return WebSessionAuthentication{}, err
	}
	if record.RevokedAt != nil || !record.ExpiresAt.After(time.Now().UTC()) {
		return WebSessionAuthentication{}, ErrWebSessionInvalid
	}
	return store.authenticateWebSessionRecord(ctx, record)
}

func (store *Store) authenticateWebSessionRecord(ctx context.Context, record webSessionRecord) (WebSessionAuthentication, error) {
	var link webIdentityRecord
	if err := store.database.WithContext(ctx).Where("issuer = ? AND subject = ?", record.Issuer, record.Subject).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return WebSessionAuthentication{}, ErrWebSessionInvalid
		}
		return WebSessionAuthentication{}, err
	}
	if err := store.validateWebIdentityAuthority(ctx, link); err != nil {
		return WebSessionAuthentication{}, err
	}
	result := WebSessionAuthentication{Session: WebSessionContext{ActorKind: link.ActorKind, ExpiresAt: record.ExpiresAt, CSRFToken: record.CSRFToken}}
	switch link.ActorKind {
	case WebActorPlatform:
		principal, err := store.AuthenticatePlatformSubject(ctx, link.ActorID, AuthMethodWeb)
		if err != nil {
			return WebSessionAuthentication{}, err
		}
		result.Session.PlatformSubject = link.ActorID
		result.Principal = principal
	case WebActorUser:
		result.Session.UserID = link.ActorID
		tenants, err := store.listWebTenants(ctx, link.ActorID)
		if err != nil {
			return WebSessionAuthentication{}, err
		}
		result.Session.Tenants = tenants
		if record.ActiveTenantID == "" {
			return result, nil
		}
		principal, err := store.resolveWebUserPrincipal(ctx, link.ActorID, record.ActiveTenantID)
		if err != nil {
			if errors.Is(err, ErrUnauthorized) {
				return result, nil
			}
			return WebSessionAuthentication{}, err
		}
		result.Session.ActiveTenantID = record.ActiveTenantID
		result.Principal = principal
	default:
		return WebSessionAuthentication{}, ErrWebSessionInvalid
	}
	return result, nil
}

func (store *Store) resolveWebUserPrincipal(ctx context.Context, userID, tenantID string) (identity.Principal, error) {
	var user userRecord
	if err := store.database.WithContext(ctx).Where("id = ? AND status = ?", userID, "active").First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.Principal{}, ErrUnauthorized
		}
		return identity.Principal{}, err
	}
	var tenant tenantRecord
	if err := store.database.WithContext(ctx).Where("id = ? AND status = ?", tenantID, accessdomain.TenantStatusActive).First(&tenant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.Principal{}, ErrUnauthorized
		}
		return identity.Principal{}, err
	}
	var membership membershipRecord
	if err := store.database.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, accessdomain.TenantMemberStatusActive).First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return identity.Principal{}, ErrUnauthorized
		}
		return identity.Principal{}, err
	}
	var roles []string
	if err := store.database.WithContext(ctx).Table("biz_member_roles mr").
		Select("r.name").
		Joins("JOIN biz_roles r ON r.id = mr.role_id AND r.tenant_id = mr.tenant_id AND r.status = ?", accessdomain.TenantRoleStatusActive).
		Where("mr.tenant_id = ? AND mr.user_id = ?", tenantID, userID).
		Scan(&roles).Error; err != nil {
		return identity.Principal{}, err
	}
	sort.Strings(roles)
	return identity.Principal{
		Subject: "user:" + userID, TenantID: tenantID, UserID: userID,
		Roles: roles, AuthMethod: AuthMethodWeb, Authenticated: true,
	}, nil
}

func (store *Store) listWebTenants(ctx context.Context, userID string) ([]WebTenant, error) {
	var rows []WebTenant
	if err := store.database.WithContext(ctx).Table("biz_memberships m").
		Select("t.id AS id, t.name AS name").
		Joins("JOIN biz_tenants t ON t.id = m.tenant_id AND t.status = ?", accessdomain.TenantStatusActive).
		Where("m.user_id = ? AND m.status = ?", userID, accessdomain.TenantMemberStatusActive).
		Order("t.name ASC, t.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (store *Store) SwitchWebSessionTenant(ctx context.Context, rawToken, tenantID string) (WebSessionAuthentication, error) {
	tenantID = strings.TrimSpace(tenantID)
	if store == nil || store.database == nil || strings.TrimSpace(rawToken) == "" || tenantID == "" {
		return WebSessionAuthentication{}, ErrWebTenantDenied
	}
	authentication, err := store.AuthenticateWebSession(ctx, rawToken)
	if err != nil {
		return WebSessionAuthentication{}, err
	}
	if authentication.Session.ActorKind != WebActorUser || authentication.Session.UserID == "" {
		return WebSessionAuthentication{}, ErrWebTenantDenied
	}
	if _, err := store.resolveWebUserPrincipal(ctx, authentication.Session.UserID, tenantID); err != nil {
		return WebSessionAuthentication{}, ErrWebTenantDenied
	}
	now := time.Now().UTC()
	result := store.database.WithContext(ctx).Model(&webSessionRecord{}).
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", TokenHash(rawToken), now).
		Updates(map[string]any{"active_tenant_id": tenantID, "updated_at": now})
	if result.Error != nil {
		return WebSessionAuthentication{}, result.Error
	}
	if result.RowsAffected != 1 {
		return WebSessionAuthentication{}, ErrWebSessionInvalid
	}
	return store.AuthenticateWebSession(ctx, rawToken)
}

func (store *Store) RevokeWebSession(ctx context.Context, rawToken string) error {
	if store == nil || store.database == nil || strings.TrimSpace(rawToken) == "" {
		return nil
	}
	now := time.Now().UTC()
	return store.database.WithContext(ctx).Model(&webSessionRecord{}).
		Where("token_hash = ? AND revoked_at IS NULL", TokenHash(rawToken)).
		Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error
}

func (store *Store) ValidateWebSessionCSRF(authentication WebSessionAuthentication, provided string) bool {
	expected := authentication.Session.CSRFToken
	if expected == "" || provided == "" || len(expected) != len(provided) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

func (store *Store) CreateWebLoginFlow(ctx context.Context, state, browser, codeVerifier, nonce, returnTo string, ttl time.Duration) error {
	if store == nil || store.database == nil || ttl <= 0 || state == "" || browser == "" || codeVerifier == "" || nonce == "" {
		return ErrWebLoginFlow
	}
	now := time.Now().UTC()
	_ = store.database.WithContext(ctx).Where("expires_at <= ?", now).Delete(&webLoginFlowRecord{}).Error
	return store.database.WithContext(ctx).Create(&webLoginFlowRecord{
		StateHash: TokenHash(state), BrowserHash: TokenHash(browser), CodeVerifier: codeVerifier,
		NonceHash: TokenHash(nonce), ReturnTo: returnTo, ExpiresAt: now.Add(ttl), CreatedAt: now,
	}).Error
}

func (store *Store) ConsumeWebLoginFlow(ctx context.Context, state, browser string) (WebLoginFlow, error) {
	if store == nil || store.database == nil || state == "" || browser == "" {
		return WebLoginFlow{}, ErrWebLoginFlow
	}
	var result WebLoginFlow
	err := store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row webLoginFlowRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("state_hash = ? AND browser_hash = ?", TokenHash(state), TokenHash(browser)).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWebLoginFlow
			}
			return err
		}
		if !row.ExpiresAt.After(time.Now().UTC()) {
			_ = tx.Delete(&row).Error
			return ErrWebLoginFlow
		}
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		result = WebLoginFlow{CodeVerifier: row.CodeVerifier, NonceHash: row.NonceHash, ReturnTo: row.ReturnTo}
		return nil
	})
	return result, err
}

func randomWebSecret(bytes int) (string, error) {
	if bytes < 16 {
		return "", errors.New("access: web secret length too small")
	}
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
