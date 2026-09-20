package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MemberAppealStatePending = "PENDING"
	memberAppealInterval     = 5 * time.Minute
)

var (
	ErrMemberAppealNotEligible = errors.New("access: member status appeal is not eligible")
	ErrMemberAppealUnavailable = errors.New("access: member status appeal is unavailable")
)

type MemberAppealRateLimitError struct {
	RetryAfter time.Duration
}

func (err MemberAppealRateLimitError) Error() string {
	return "access: member status appeal rate limited"
}

type memberStatusAppealRecord struct {
	TenantID             string    `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID               string    `gorm:"column:user_id;primaryKey;size:64"`
	AppealID              string    `gorm:"column:appeal_id;size:64;not null;uniqueIndex"`
	MembershipStatus      string    `gorm:"column:membership_status;size:32;not null"`
	State                 string    `gorm:"column:state;size:24;not null;index"`
	Reason                string    `gorm:"column:reason;size:500;not null"`
	NotificationEventJSON string    `gorm:"column:notification_event_ids;type:text"`
	SubmittedAt           time.Time `gorm:"column:submitted_at;type:datetime(6);not null;index"`
	UpdatedAt             time.Time `gorm:"column:updated_at;type:datetime(6);not null"`
}

func (memberStatusAppealRecord) TableName() string { return "biz_member_status_appeals" }

type MemberAppealEligibility struct {
	TenantID      string     `json:"tenant_id"`
	TenantName    string     `json:"tenant_name"`
	Status        string     `json:"status"`
	AppealID      string     `json:"appeal_id,omitempty"`
	AppealState   string     `json:"appeal_state,omitempty"`
	LastSubmitted *time.Time `json:"last_submitted,omitempty"`
}

type MemberAppealReceipt struct {
	AppealID             string    `json:"appeal_id"`
	TenantID             string    `json:"tenant_id"`
	Status               string    `json:"membership_status"`
	State                string    `json:"state"`
	SubmittedAt          time.Time `json:"submitted_at"`
	NotificationEventIDs []string  `json:"notification_event_ids"`
}

type MemberAppealService struct {
	database          *gorm.DB
	contactProtection *ContactProtection
	verification      *VerificationRepository
}

func NewMemberAppealService(
	database *gorm.DB,
	contactProtection *ContactProtection,
	verificationProtection *VerificationProtection,
) (*MemberAppealService, error) {
	if database == nil {
		return nil, ErrMemberAppealUnavailable
	}
	if verificationProtection == nil {
		return nil, ErrVerificationKeyUnavailable
	}
	verification, err := NewVerificationRepository(database, verificationProtection)
	if err != nil {
		return nil, err
	}
	return &MemberAppealService{
		database: database, contactProtection: contactProtection, verification: verification,
	}, nil
}

func (service *MemberAppealService) EnsureSchema(ctx context.Context) error {
	if service == nil || service.database == nil {
		return ErrMemberAppealUnavailable
	}
	return service.database.WithContext(ctx).AutoMigrate(&memberStatusAppealRecord{})
}

func (service *MemberAppealService) ListEligible(ctx context.Context, userID string) ([]MemberAppealEligibility, error) {
	if service == nil || service.database == nil {
		return nil, ErrMemberAppealUnavailable
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrMemberAppealNotEligible
	}
	type row struct {
		TenantID, TenantName, Status, AppealID, AppealState string
		SubmittedAt                                         *time.Time
	}
	var rows []row
	if err := service.database.WithContext(ctx).Table("biz_memberships m").
		Select("m.tenant_id, t.name AS tenant_name, m.status, COALESCE(a.appeal_id, '') AS appeal_id, COALESCE(a.state, '') AS appeal_state, a.submitted_at").
		Joins("JOIN biz_tenants t ON t.id = m.tenant_id AND t.status = ?", domain.TenantStatusActive).
		Joins("LEFT JOIN biz_member_status_appeals a ON a.tenant_id = m.tenant_id AND a.user_id = m.user_id").
		Where("m.user_id = ? AND m.status IN ?", userID, []string{domain.TenantMemberStatusSuspended, domain.TenantMemberStatusRemoved}).
		Order("t.name ASC, t.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]MemberAppealEligibility, 0, len(rows))
	for _, row := range rows {
		result = append(result, MemberAppealEligibility{
			TenantID: row.TenantID, TenantName: row.TenantName, Status: row.Status,
			AppealID: row.AppealID, AppealState: row.AppealState, LastSubmitted: row.SubmittedAt,
		})
	}
	return result, nil
}

func (service *MemberAppealService) Submit(
	ctx context.Context,
	userID, tenantID, reason string,
) (MemberAppealReceipt, error) {
	if service == nil || service.database == nil || service.verification == nil {
		return MemberAppealReceipt{}, ErrMemberAppealUnavailable
	}
	userID = strings.TrimSpace(userID)
	tenantID = strings.TrimSpace(tenantID)
	reason = strings.TrimSpace(reason)
	if userID == "" || tenantID == "" || reason == "" || len([]rune(reason)) > 500 {
		return MemberAppealReceipt{}, ErrMemberAppealNotEligible
	}
	var receipt MemberAppealReceipt
	err := service.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		var membership membershipRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&membership).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMemberAppealNotEligible
			}
			return err
		}
		if membership.Status != domain.TenantMemberStatusSuspended && membership.Status != domain.TenantMemberStatusRemoved {
			return ErrMemberAppealNotEligible
		}

		var existing memberStatusAppealRecord
		existingErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&existing).Error
		existingFound := existingErr == nil
		switch {
		case existingFound:
			nextAllowed := existing.SubmittedAt.Add(memberAppealInterval)
			if nextAllowed.After(now) {
				return MemberAppealRateLimitError{RetryAfter: nextAllowed.Sub(now)}
			}
		case errors.Is(existingErr, gorm.ErrRecordNotFound):
		default:
			return existingErr
		}

		secret, err := randomVerificationSecret(18)
		if err != nil {
			return err
		}
		appealID := "map-" + secret
		var ownerIDs []string
		if err := tx.Table("biz_member_roles mr").
			Select("mr.user_id").
			Joins("JOIN biz_roles r ON r.tenant_id = mr.tenant_id AND r.id = mr.role_id AND r.name = ? AND r.status = ?", domain.TenantOwnerRoleName, domain.TenantRoleStatusActive).
			Joins("JOIN biz_memberships m ON m.tenant_id = mr.tenant_id AND m.user_id = mr.user_id AND m.status = ?", domain.TenantMemberStatusActive).
			Where("mr.tenant_id = ?", tenantID).
			Order("mr.user_id ASC").Pluck("mr.user_id", &ownerIDs).Error; err != nil {
			return err
		}
		if len(ownerIDs) == 0 {
			return ErrMemberAppealUnavailable
		}

		verification := &VerificationRepository{database: tx, protection: service.verification.protection}
		notifier := &TenantMemberLifecycleNotificationRepository{
			database: tx, contactProtection: service.contactProtection, verification: verification,
		}
		eventIDs := make([]string, 0, len(ownerIDs))
		for _, ownerID := range ownerIDs {
			channel, destination, err := notifier.memberNotificationDestination(ctx, tenantID, ownerID)
			if err != nil {
				return err
			}
			delivery, err := verification.EnqueueSecurityNotification(ctx, domain.SecurityNotificationRequest{
				BusinessEventID: fmt.Sprintf("member-appeal/%s/%s/%s/%s", tenantID, userID, appealID, ownerID),
				Kind:            domain.SecurityNotificationMemberAppeal,
				Purpose:         domain.VerificationPurposeMemberAppeal,
				UserID:          ownerID,
				TenantID:        tenantID,
				FlowID:          appealID,
				Channel:         channel,
				Destination:     destination,
				Secret:          "applicant_user_id=" + userID + "\nreason=" + reason,
				ExpiresAt:       now.Add(24 * time.Hour),
			})
			if err != nil {
				return err
			}
			eventIDs = append(eventIDs, delivery.EventID)
		}
		encoded, err := json.Marshal(eventIDs)
		if err != nil {
			return err
		}
		record := memberStatusAppealRecord{
			TenantID: tenantID, UserID: userID, AppealID: appealID,
			MembershipStatus: membership.Status, State: MemberAppealStatePending, Reason: reason,
			NotificationEventJSON: string(encoded), SubmittedAt: now, UpdatedAt: now,
		}
		if existingFound {
			result := tx.Model(&memberStatusAppealRecord{}).
				Where("tenant_id = ? AND user_id = ?", tenantID, userID).
				Updates(map[string]any{
					"appeal_id": appealID, "membership_status": membership.Status, "state": MemberAppealStatePending,
					"reason": reason, "notification_event_ids": string(encoded), "submitted_at": now, "updated_at": now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrMemberAppealUnavailable
			}
		} else {
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		}
		receipt = MemberAppealReceipt{
			AppealID: appealID, TenantID: tenantID, Status: membership.Status,
			State: MemberAppealStatePending, SubmittedAt: now, NotificationEventIDs: eventIDs,
		}
		return nil
	})
	return receipt, err
}
