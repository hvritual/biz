package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	memberActivationStatePending  = "PENDING"
	memberActivationStateConsumed = "CONSUMED"

	memberActivationModeLink = "activation_link"
	memberActivationModeSMS  = "sms_initial_password"
)

type memberActivationRecord struct {
	TenantID            string     `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID              string     `gorm:"column:user_id;primaryKey;size:64"`
	Mode                string     `gorm:"column:mode;size:32;not null"`
	SecretHash          string     `gorm:"column:secret_hash;size:64;not null;default:'';index"`
	NewAccount          bool       `gorm:"column:new_account;not null;default:false"`
	State               string     `gorm:"column:state;size:24;not null;index"`
	ExpiresAt           time.Time  `gorm:"column:expires_at;type:datetime(6);not null;index"`
	ConsumedAt          *time.Time `gorm:"column:consumed_at;type:datetime(6)"`
	NotificationEventID string     `gorm:"column:notification_event_id;size:64;not null;default:''"`
	NotificationState   string     `gorm:"column:notification_state;size:24;not null;default:''"`
	CreatedAt           time.Time  `gorm:"column:created_at;type:datetime(6);not null"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;type:datetime(6);not null"`
}

func (memberActivationRecord) TableName() string { return "biz_member_activations" }

var (
	ErrMemberActivationInvalid  = errors.New("access: member activation invalid")
	ErrMemberActivationExpired  = errors.New("access: member activation expired")
	ErrMemberActivationConsumed = errors.New("access: member activation consumed")
)

type MemberActivationView struct {
	TenantID        string
	UserID          string
	Mode            string
	NewAccount      bool
	RequiresPassword bool
	ExpiresAt       time.Time
}


type TenantMemberActivationRepository struct {
	database     *gorm.DB
	verification *VerificationRepository
}

func NewTenantMemberActivationRepository(database *gorm.DB, protection *VerificationProtection) (*TenantMemberActivationRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: tenant member activation database is required")
	}
	var verification *VerificationRepository
	var err error
	if protection != nil {
		verification, err = NewVerificationRepository(database, protection)
		if err != nil {
			return nil, err
		}
	}
	return &TenantMemberActivationRepository{database: database, verification: verification}, nil
}

func (repository *TenantMemberActivationRepository) AssertAdminActivationAllowed(ctx context.Context, tenantID, userID string) error {
	if repository == nil || repository.database == nil {
		return nil
	}
	var count int64
	if err := repository.database.WithContext(ctx).Model(&memberActivationRecord{}).
		Where("tenant_id = ? AND user_id = ? AND state = ? AND consumed_at IS NULL AND expires_at > ?", strings.TrimSpace(tenantID), strings.TrimSpace(userID), memberActivationStatePending, time.Now().UTC()).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ports.ErrTenantMemberActivationPending
	}
	return nil
}

func (repository *TenantMemberActivationRepository) Stage(ctx context.Context, input ports.TenantMemberActivationInput) (ports.TenantMemberActivationReceipt, error) {
	if repository == nil || repository.database == nil || repository.verification == nil {
		return ports.TenantMemberActivationReceipt{}, ports.ErrTenantMemberActivationUnavailable
	}
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Mode = strings.TrimSpace(input.Mode)
	input.Secret = strings.TrimSpace(input.Secret)
	input.NotificationSecret = strings.TrimSpace(input.NotificationSecret)
	if input.TenantID == "" || input.UserID == "" || input.Username == "" || input.ExpiresAt.IsZero() || !input.ExpiresAt.After(time.Now().UTC()) {
		return ports.TenantMemberActivationReceipt{}, ports.ErrTenantMemberActivationUnavailable
	}

	channel := domain.SecurityNotificationEmail
	destination := input.Email
	secretHash := ""
	switch input.Mode {
	case memberActivationModeLink:
		if input.Secret == "" || input.NotificationSecret == "" {
			return ports.TenantMemberActivationReceipt{}, ports.ErrTenantMemberActivationUnavailable
		}
		secretHash = TokenHash(input.Secret)
		if destination == "" {
			channel = domain.SecurityNotificationSMS
			destination = input.Phone
		}
	case memberActivationModeSMS:
		if !input.NewAccount {
			return ports.TenantMemberActivationReceipt{}, ports.ErrTenantMemberExistingAccountSMS
		}
		if input.Phone == "" || input.Secret == "" || input.NotificationSecret == "" {
			return ports.TenantMemberActivationReceipt{}, ports.ErrTenantMemberActivationUnavailable
		}
		channel = domain.SecurityNotificationSMS
		destination = input.Phone
		if err := setInitialUserPassword(ctx, repository.database, input.UserID, input.Secret, input.ExpiresAt); err != nil {
			return ports.TenantMemberActivationReceipt{}, err
		}
	default:
		return ports.TenantMemberActivationReceipt{}, ports.ErrTenantMemberActivationUnavailable
	}
	if destination == "" {
		return ports.TenantMemberActivationReceipt{}, ports.ErrTenantMemberActivationUnavailable
	}

	now := time.Now().UTC()
	record := memberActivationRecord{
		TenantID: input.TenantID, UserID: input.UserID, Mode: input.Mode, SecretHash: secretHash,
		NewAccount: input.NewAccount, State: memberActivationStatePending, ExpiresAt: input.ExpiresAt.UTC(),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.database.WithContext(ctx).Create(&record).Error; err != nil {
		return ports.TenantMemberActivationReceipt{}, err
	}

	notification, err := repository.verification.EnqueueSecurityNotification(ctx, domain.SecurityNotificationRequest{
		BusinessEventID: "member-activation/" + input.TenantID + "/" + input.UserID,
		Kind:            domain.SecurityNotificationInitialCredential,
		Purpose:         domain.VerificationPurposeMemberActivation,
		UserID:          input.UserID,
		TenantID:        input.TenantID,
		FlowID:          input.TenantID + "/" + input.UserID,
		Channel:         channel,
		Destination:     destination,
		Secret:          input.NotificationSecret,
		ExpiresAt:       input.ExpiresAt.UTC(),
	})
	if err != nil {
		return ports.TenantMemberActivationReceipt{}, err
	}
	if err := repository.database.WithContext(ctx).Model(&memberActivationRecord{}).
		Where("tenant_id = ? AND user_id = ?", input.TenantID, input.UserID).
		Updates(map[string]any{
			"notification_event_id": notification.EventID,
			"notification_state":    notification.State,
			"updated_at":            now,
		}).Error; err != nil {
		return ports.TenantMemberActivationReceipt{}, err
	}
	return ports.TenantMemberActivationReceipt{
		NotificationEventID: notification.EventID,
		DeliveryState:       notification.State,
		MaskedDestination:   maskActivationDestination(channel, destination),
	}, nil
}

func maskActivationDestination(channel domain.SecurityNotificationChannel, destination string) string {
	if channel == domain.SecurityNotificationSMS {
		return MaskPhone(destination)
	}
	return MaskEmail(destination)
}

func (store *Store) InspectMemberActivation(ctx context.Context, token string) (MemberActivationView, error) {
	if store == nil || store.database == nil || strings.TrimSpace(token) == "" {
		return MemberActivationView{}, ErrMemberActivationInvalid
	}
	var record memberActivationRecord
	if err := store.database.WithContext(ctx).
		Where("secret_hash = ? AND mode = ?", TokenHash(strings.TrimSpace(token)), memberActivationModeLink).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return MemberActivationView{}, ErrMemberActivationInvalid
		}
		return MemberActivationView{}, err
	}
	if record.State == memberActivationStateConsumed || record.ConsumedAt != nil {
		return MemberActivationView{}, ErrMemberActivationConsumed
	}
	if !record.ExpiresAt.After(time.Now().UTC()) {
		return MemberActivationView{}, ErrMemberActivationExpired
	}
	return MemberActivationView{
		TenantID: record.TenantID,
		UserID: record.UserID,
		Mode: record.Mode,
		NewAccount: record.NewAccount,
		RequiresPassword: record.NewAccount,
		ExpiresAt: record.ExpiresAt,
	}, nil
}

func (store *Store) CompleteMemberActivationLink(ctx context.Context, token, newPassword, confirmation string) (MemberActivationView, error) {
	if store == nil || store.database == nil || strings.TrimSpace(token) == "" {
		return MemberActivationView{}, ErrMemberActivationInvalid
	}
	var view MemberActivationView
	err := store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record memberActivationRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("secret_hash = ? AND mode = ?", TokenHash(strings.TrimSpace(token)), memberActivationModeLink).
			First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMemberActivationInvalid
			}
			return err
		}
		if record.State == memberActivationStateConsumed || record.ConsumedAt != nil {
			return ErrMemberActivationConsumed
		}
		now := time.Now().UTC()
		if !record.ExpiresAt.After(now) {
			return ErrMemberActivationExpired
		}
		if record.NewAccount {
			if newPassword != confirmation {
				return ErrPasswordMismatch
			}
			if err := ValidateUserChosenPassword(newPassword); err != nil {
				return err
			}
			if err := setUserPassword(ctx, tx, record.UserID, newPassword); err != nil {
				return err
			}
		}
		if err := completeMemberActivation(ctx, tx, &record, now); err != nil {
			return err
		}
		view = MemberActivationView{
			TenantID: record.TenantID, UserID: record.UserID, Mode: record.Mode,
			NewAccount: record.NewAccount, RequiresPassword: record.NewAccount, ExpiresAt: record.ExpiresAt,
		}
		return nil
	})
	return view, err
}

func (store *Store) CompleteInitialPasswordActivationForLogin(
	ctx context.Context,
	requestID, browserSecret, csrf, newPassword, confirmation string,
) (LocalUserIdentity, uint64, error) {
	if store == nil || store.database == nil || strings.TrimSpace(requestID) == "" || strings.TrimSpace(browserSecret) == "" || strings.TrimSpace(csrf) == "" {
		return LocalUserIdentity{}, 0, ErrMemberActivationInvalid
	}
	if newPassword != confirmation {
		return LocalUserIdentity{}, 0, ErrPasswordMismatch
	}
	if err := ValidateUserChosenPassword(newPassword); err != nil {
		return LocalUserIdentity{}, 0, err
	}
	var identity LocalUserIdentity
	var loginAuditID uint64
	err := store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var request firstPartyAuthorizationRequestRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("request_hash = ? AND browser_hash = ?", TokenHash(requestID), TokenHash(browserSecret)).
			First(&request).Error; err != nil {
			return ErrMemberActivationInvalid
		}
		if !request.ExpiresAt.After(time.Now().UTC()) || !constantTimeTokenHashEqual(request.CSRFHash, TokenHash(csrf)) || strings.TrimSpace(request.AuthenticatedUserID) == "" {
			return ErrMemberActivationInvalid
		}
		var records []memberActivationRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND mode = ? AND state = ?", request.AuthenticatedUserID, memberActivationModeSMS, memberActivationStatePending).
			Order("created_at ASC").Limit(2).Find(&records).Error; err != nil {
			return err
		}
		if len(records) != 1 {
			return ErrMemberActivationInvalid
		}
		record := records[0]
		now := time.Now().UTC()
		if !record.ExpiresAt.After(now) {
			return ErrMemberActivationExpired
		}
		if err := setUserPassword(ctx, tx, record.UserID, newPassword); err != nil {
			return err
		}
		if err := completeMemberActivation(ctx, tx, &record, now); err != nil {
			return err
		}
		var user userRecord
		if err := tx.Where("id = ? AND status = ?", record.UserID, "active").First(&user).Error; err != nil {
			return err
		}
		email, err := store.userEmail(user)
		if err != nil {
			return err
		}
		identity = LocalUserIdentity{UserID: user.ID, Email: email}
		loginAuditID = request.LoginAuditID
		return nil
	})
	return identity, loginAuditID, err
}

func completeMemberActivation(ctx context.Context, tx *gorm.DB, record *memberActivationRecord, now time.Time) error {
	result := tx.WithContext(ctx).Model(&membershipRecord{}).
		Where("tenant_id = ? AND user_id = ? AND status = ?", record.TenantID, record.UserID, domain.TenantMemberStatusInvited).
		Updates(map[string]any{
			"status": domain.TenantMemberStatusActive,
			"version": gorm.Expr("version + 1"),
			"updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrMemberActivationInvalid
	}
	record.State = memberActivationStateConsumed
	record.ConsumedAt = &now
	record.UpdatedAt = now
	if err := tx.WithContext(ctx).Model(&memberActivationRecord{}).
		Where("tenant_id = ? AND user_id = ? AND state = ?", record.TenantID, record.UserID, memberActivationStatePending).
		Updates(map[string]any{"state": record.State, "consumed_at": now, "updated_at": now}).Error; err != nil {
		return err
	}
	if record.NotificationEventID != "" {
		if err := tx.WithContext(ctx).Model(&securityNotificationOutboxRecord{}).
			Where("event_id = ? AND state IN ?", record.NotificationEventID, []string{domain.NotificationStatePending, domain.NotificationStateFailed}).
			Updates(map[string]any{
				"state": domain.NotificationStateCancelled,
				"destination_ciphertext": "",
				"secret_ciphertext": "",
				"failure_code": "MEMBER_ACTIVATED",
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (store *Store) EnsureMemberActivationSchema(ctx context.Context) error {
	if store == nil || store.database == nil {
		return errors.New("access: member activation schema store unavailable")
	}
	return store.database.WithContext(ctx).AutoMigrate(&memberActivationRecord{}, &userPasswordCredentialRecord{})
}
