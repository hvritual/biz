package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
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
