package persistence

import (
	"context"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"github.com/hvritual/biz/internal/access/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type LoginIdentifierKind string

const (
	LoginIdentifierUsername LoginIdentifierKind = "username"
	LoginIdentifierEmail    LoginIdentifierKind = "email"
	LoginIdentifierPhone    LoginIdentifierKind = "phone"
)

var (
	ErrInvalidLoginIdentifier  = errors.New("access: invalid login identifier")
	ErrLoginIdentifierConflict = errors.New("access: login identifier already exists")
)

type LoginIdentifierResolution struct {
	Identity       LocalUserIdentity
	Kind           LoginIdentifierKind
	Normalized     string
	OTPChannel     domain.SecurityNotificationChannel
	OTPDestination string
}

func NormalizeLoginIdentifier(value string) (LoginIdentifierKind, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", ErrInvalidLoginIdentifier
	}
	if strings.Contains(value, "@") {
		normalized, err := NormalizeEmail(value)
		if err != nil {
			return "", "", ErrInvalidLoginIdentifier
		}
		return LoginIdentifierEmail, normalized, nil
	}
	if looksLikePhoneIdentifier(value) {
		normalized, err := NormalizePhone(value)
		if err != nil || normalized == "" {
			return "", "", ErrInvalidLoginIdentifier
		}
		return LoginIdentifierPhone, normalized, nil
	}
	username := strings.ToLower(value)
	length := utf8.RuneCountInString(username)
	if length < 5 || length > 20 {
		return "", "", ErrInvalidLoginIdentifier
	}
	allDigits := true
	for _, r := range username {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", "", ErrInvalidLoginIdentifier
		}
		if !unicode.IsDigit(r) {
			allDigits = false
		}
	}
	if allDigits {
		return "", "", ErrInvalidLoginIdentifier
	}
	return LoginIdentifierUsername, username, nil
}

func looksLikePhoneIdentifier(value string) bool {
	for index, r := range value {
		switch {
		case unicode.IsDigit(r):
		case r == '+' && index == 0:
		case r == ' ' || r == '-' || r == '(' || r == ')':
		default:
			return false
		}
	}
	return true
}

func (store *Store) SetUserUsername(ctx context.Context, userID, username string) error {
	if store == nil || store.database == nil || strings.TrimSpace(userID) == "" {
		return ErrInvalidLoginIdentifier
	}
	_, normalized, err := NormalizeLoginIdentifier(username)
	if err != nil {
		return err
	}
	kind, _, _ := NormalizeLoginIdentifier(normalized)
	if kind != LoginIdentifierUsername {
		return ErrInvalidLoginIdentifier
	}
	result := store.database.WithContext(ctx).Model(&userRecord{}).
		Where("id = ?", strings.TrimSpace(userID)).
		Update("username", normalized)
	if result.Error != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(result.Error, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrLoginIdentifierConflict
		}
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInvalidLoginIdentifier
	}
	return nil
}

func (store *Store) ResolveLoginIdentifier(ctx context.Context, identifier string) (LoginIdentifierResolution, error) {
	if store == nil || store.database == nil {
		return LoginIdentifierResolution{}, ErrInvalidUserCredentials
	}
	kind, normalized, err := NormalizeLoginIdentifier(identifier)
	if err != nil {
		return LoginIdentifierResolution{}, ErrInvalidUserCredentials
	}
	switch kind {
	case LoginIdentifierEmail:
		user, authoritativeEmail, err := store.findUserByEmail(ctx, store.database, normalized, true)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, ErrInvalidContact) {
				return LoginIdentifierResolution{}, ErrInvalidUserCredentials
			}
			return LoginIdentifierResolution{}, err
		}
		return LoginIdentifierResolution{
			Identity: LocalUserIdentity{UserID: user.ID, Email: authoritativeEmail},
			Kind:     kind, Normalized: normalized,
			OTPChannel: domain.SecurityNotificationEmail, OTPDestination: authoritativeEmail,
		}, nil
	case LoginIdentifierUsername:
		var users []userRecord
		if err := store.database.WithContext(ctx).
			Where("LOWER(username) = ? AND status = ?", normalized, "active").
			Order("id ASC").Limit(2).Find(&users).Error; err != nil {
			return LoginIdentifierResolution{}, err
		}
		if len(users) != 1 {
			return LoginIdentifierResolution{}, ErrInvalidUserCredentials
		}
		email, err := store.userEmail(users[0])
		if err != nil {
			return LoginIdentifierResolution{}, err
		}
		return LoginIdentifierResolution{
			Identity: LocalUserIdentity{UserID: users[0].ID, Email: email},
			Kind:     kind, Normalized: normalized,
			OTPChannel: domain.SecurityNotificationEmail, OTPDestination: email,
		}, nil
	case LoginIdentifierPhone:
		userID, err := store.resolveUserIDByPhone(ctx, normalized)
		if err != nil {
			return LoginIdentifierResolution{}, err
		}
		var user userRecord
		if err := store.database.WithContext(ctx).Where("id = ? AND status = ?", userID, "active").First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return LoginIdentifierResolution{}, ErrInvalidUserCredentials
			}
			return LoginIdentifierResolution{}, err
		}
		email, err := store.userEmail(user)
		if err != nil {
			return LoginIdentifierResolution{}, err
		}
		return LoginIdentifierResolution{
			Identity: LocalUserIdentity{UserID: user.ID, Email: email},
			Kind:     kind, Normalized: normalized,
			OTPChannel: domain.SecurityNotificationSMS, OTPDestination: normalized,
		}, nil
	default:
		return LoginIdentifierResolution{}, ErrInvalidUserCredentials
	}
}

func (store *Store) resolveUserIDByPhone(ctx context.Context, normalized string) (string, error) {
	query := store.database.WithContext(ctx).Model(&membershipRecord{}).Where("status = ?", domain.TenantMemberStatusActive)
	if store.contactProtection != nil {
		lookup, err := store.contactProtection.LookupPhone(normalized)
		if err != nil {
			return "", err
		}
		query = query.Where("phone_lookup_hash = ?", lookup)
	} else {
		if query.Logger != nil {
			query = query.Session(&gorm.Session{Logger: query.Logger.LogMode(logger.Silent)})
		}
		query = query.Where("phone = ?", normalized)
	}
	var userIDs []string
	if err := query.Distinct("user_id").Order("user_id ASC").Limit(2).Pluck("user_id", &userIDs).Error; err != nil {
		return "", err
	}
	if len(userIDs) != 1 {
		return "", ErrInvalidUserCredentials
	}
	return userIDs[0], nil
}

func LoginIdentifierThrottleHash(identifier string) string {
	kind, normalized, err := NormalizeLoginIdentifier(identifier)
	if err != nil {
		return TokenHash("login/invalid/" + strings.ToLower(strings.TrimSpace(identifier)))
	}
	return TokenHash("login/" + string(kind) + "/" + normalized)
}
