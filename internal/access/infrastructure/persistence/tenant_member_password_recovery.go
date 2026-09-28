package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const tenantMemberPasswordRecoveryInterval = 5 * time.Minute

var (
	ErrTenantMemberPasswordRecoveryUnavailable = errors.New("access: tenant member password recovery unavailable")
	ErrTenantMemberPasswordRecoveryNotFound    = errors.New("access: tenant member password recovery target not found")
)

type TenantMemberPasswordRecoveryRateLimitError struct {
	RetryAfter time.Duration
}

func (err TenantMemberPasswordRecoveryRateLimitError) Error() string {
	return "access: tenant member password recovery rate limited"
}

type TenantMemberPasswordRecoveryReceipt struct {
	NotificationEventID string
	NotificationState   string
	RequestedAt         time.Time
}

type TenantMemberPasswordRecoveryService struct {
	database          *gorm.DB
	contactProtection *ContactProtection
	verification      *VerificationRepository
}

func NewTenantMemberPasswordRecoveryService(
	database *gorm.DB,
	contactProtection *ContactProtection,
	verificationProtection *VerificationProtection,
) (*TenantMemberPasswordRecoveryService, error) {
	if database == nil || verificationProtection == nil {
		return nil, ErrTenantMemberPasswordRecoveryUnavailable
	}
	verification, err := NewVerificationRepository(database, verificationProtection)
	if err != nil {
		return nil, err
	}
	return &TenantMemberPasswordRecoveryService{
		database: database, contactProtection: contactProtection, verification: verification,
	}, nil
}

func (service *TenantMemberPasswordRecoveryService) Request(
	ctx context.Context,
	tenantID, targetUserID, actorUserID string,
) (TenantMemberPasswordRecoveryReceipt, error) {
	if service == nil || service.database == nil || service.verification == nil {
		return TenantMemberPasswordRecoveryReceipt{}, ErrTenantMemberPasswordRecoveryUnavailable
	}
	tenantID = strings.TrimSpace(tenantID)
	targetUserID = strings.TrimSpace(targetUserID)
	actorUserID = strings.TrimSpace(actorUserID)
	if tenantID == "" || targetUserID == "" || actorUserID == "" {
		return TenantMemberPasswordRecoveryReceipt{}, ErrTenantMemberPasswordRecoveryUnavailable
	}

	var receipt TenantMemberPasswordRecoveryReceipt
	var expectedOutcome error
	err := service.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := verificationDatabaseNow(ctx, tx)
		if err != nil {
			return err
		}
		var member membershipRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ? AND status = ? AND self_deleted_at IS NULL", tenantID, targetUserID, domain.TenantMemberStatusActive).
			First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrTenantMemberPasswordRecoveryNotFound
			}
			return err
		}

		var latest securityNotificationOutboxRecord
		latestErr := tx.Where("tenant_id = ? AND user_id = ? AND kind = ?", tenantID, targetUserID, string(domain.SecurityNotificationRecoveryRequest)).
			Order("created_at DESC").Limit(1).Take(&latest).Error
		switch {
		case latestErr == nil:
			nextAllowed := latest.CreatedAt.Add(tenantMemberPasswordRecoveryInterval)
			if nextAllowed.After(now) {
				if err := AppendTrustedAuditPairTx(ctx, tx, TrustedAudit{
					EventKey:    fmt.Sprintf("admin-recovery-rate/%s/%s/%s/%d", tenantID, targetUserID, actorUserID, now.UnixNano()),
					OperationID: "tenant.member.password_recovery.request", Module: "access",
					TenantID: tenantID, ActorSubject: "user:" + actorUserID, ActorUserID: actorUserID,
					AuthMethod: AuthMethodWeb, AuthChannel: "web",
					Target: "user_id:" + targetUserID, ResourceTenantID: tenantID,
					DecisionReason: "PASSWORD_RECOVERY_RATE_LIMITED",
					RequestDigest:  TokenHash("password-recovery-request/v1"),
					Reason:         "changed_fields=password_recovery_request", Risk: domain.AuditRiskHigh,
					Outcome: domain.AuditResultFailure, OccurredAt: now,
				}); err != nil {
					return err
				}
				expectedOutcome = TenantMemberPasswordRecoveryRateLimitError{RetryAfter: nextAllowed.Sub(now)}
				return nil
			}
		case errors.Is(latestErr, gorm.ErrRecordNotFound):
		default:
			return latestErr
		}

		verification := &VerificationRepository{database: tx, protection: service.verification.protection}
		notifier := &TenantMemberLifecycleNotificationRepository{
			database: tx, contactProtection: service.contactProtection, verification: verification,
		}
		channel, destination, err := notifier.memberNotificationDestination(ctx, tenantID, targetUserID)
		if err != nil {
			return err
		}
		businessEventID := "admin-recovery/" + TokenHash(fmt.Sprintf("%s/%s/%s/%d", tenantID, targetUserID, actorUserID, now.UnixNano()))
		delivery, err := verification.EnqueueSecurityNotification(ctx, domain.SecurityNotificationRequest{
			BusinessEventID: businessEventID,
			Kind:            domain.SecurityNotificationRecoveryRequest,
			Purpose:         domain.VerificationPurposePasswordRecovery,
			UserID:          targetUserID,
			TenantID:        tenantID,
			FlowID:          "admin-recovery/" + TokenHash(actorUserID + "/" + targetUserID)[:32],
			Channel:         channel,
			Destination:     destination,
			ExpiresAt:       now.Add(24 * time.Hour),
		})
		if err != nil {
			return err
		}
		receipt = TenantMemberPasswordRecoveryReceipt{
			NotificationEventID: delivery.EventID,
			NotificationState:   delivery.State,
			RequestedAt:         now,
		}
		if err := AppendTrustedAuditPairTx(ctx, tx, TrustedAudit{
			EventKey:    businessEventID,
			OperationID: "tenant.member.password_recovery.request", Module: "access",
			TenantID: tenantID, ActorSubject: "user:" + actorUserID, ActorUserID: actorUserID,
			AuthMethod: AuthMethodWeb, AuthChannel: "web",
			Target: "user_id:" + targetUserID, ResourceTenantID: tenantID,
			RequestDigest: TokenHash("password-recovery-request/v1"),
			ReceiptRef:    delivery.EventID, Reason: "changed_fields=password_recovery_request",
			Risk: domain.AuditRiskHigh, Outcome: domain.AuditResultSuccess, OccurredAt: now,
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return TenantMemberPasswordRecoveryReceipt{}, err
	}
	if expectedOutcome != nil {
		return TenantMemberPasswordRecoveryReceipt{}, expectedOutcome
	}
	return receipt, nil
}
