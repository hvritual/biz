package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TrustedAudit describes an audit fact whose actor/tenant/resource identity has
// already been resolved from server authority. Callers must never copy tenant
// or actor identity from an unauthenticated request into this structure.
type TrustedAudit struct {
	EventKey         string
	OperationID      string
	Module           string
	TenantID         string
	ActorSubject     string
	ActorUserID      string
	AuthMethod       string
	AuthChannel      string
	SessionRef       string
	RequestID        string
	TraceID          string
	IdempotencyRef   string
	Target           string
	ResourceTenantID string
	DecisionReason   string
	RequestDigest    string
	ReceiptRef       string
	Reason           string
	Risk             string
	Outcome          string
	OccurredAt       time.Time
}

func AppendTrustedUserAuditPairsTx(ctx context.Context, tx *gorm.DB, userID string, input TrustedAudit) (int, error) {
	if tx == nil {
		return 0, errors.New("access audit: root transaction is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return 0, errors.New("access audit: trusted user identity is required")
	}
	var tenants []string
	if err := tx.WithContext(ctx).Model(&membershipRecord{}).
		Where("user_id = ? AND status = ? AND self_deleted_at IS NULL", userID, domain.TenantMemberStatusActive).
		Order("tenant_id ASC").Pluck("tenant_id", &tenants).Error; err != nil {
		return 0, err
	}
	count := 0
	for _, tenantID := range tenants {
		item := input
		item.TenantID = tenantID
		item.ResourceTenantID = tenantID
		if item.ActorSubject == "" && item.ActorUserID == "" {
			item.ActorSubject = "user:" + userID
			item.ActorUserID = userID
		} else if item.ActorSubject == "" {
			item.ActorSubject = "user:" + item.ActorUserID
		}
		if err := AppendTrustedAuditPairTx(ctx, tx, item); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func AppendTrustedAuditPairTx(ctx context.Context, tx *gorm.DB, input TrustedAudit) error {
	if tx == nil {
		return errors.New("access audit: root transaction is required")
	}
	input.EventKey = strings.TrimSpace(input.EventKey)
	input.OperationID = strings.TrimSpace(input.OperationID)
	input.Module = strings.TrimSpace(input.Module)
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.ActorSubject = strings.TrimSpace(input.ActorSubject)
	input.ActorUserID = strings.TrimSpace(input.ActorUserID)
	input.Target = strings.TrimSpace(input.Target)
	input.ResourceTenantID = strings.TrimSpace(input.ResourceTenantID)
	input.DecisionReason = safeAuditToken(input.DecisionReason, 64)
	input.Reason = safeAuditText(input.Reason, 500)
	input.ReceiptRef = safeAuditText(input.ReceiptRef, 128)
	input.RequestID = safeAuditText(input.RequestID, 128)
	input.TraceID = safeAuditText(input.TraceID, 128)
	input.IdempotencyRef = safeAuditText(input.IdempotencyRef, 80)
	input.SessionRef = safeAuditText(input.SessionRef, 80)
	input.RequestDigest = safeAuditToken(input.RequestDigest, 64)
	if input.EventKey == "" || input.OperationID == "" || input.TenantID == "" || input.ActorSubject == "" {
		return errors.New("access audit: trusted audit identity is required")
	}
	if input.ResourceTenantID == "" {
		input.ResourceTenantID = input.TenantID
	}
	if input.Target == "" {
		input.Target = "tenant:" + input.ResourceTenantID
	}
	if input.Module == "" {
		input.Module = "access"
	}
	if input.Risk == "" {
		input.Risk = domain.AuditRiskMedium
	}
	switch input.Risk {
	case domain.AuditRiskLow, domain.AuditRiskMedium, domain.AuditRiskHigh:
	default:
		return errors.New("access audit: invalid risk")
	}
	switch input.Outcome {
	case domain.AuditResultSuccess, domain.AuditResultFailure, domain.AuditResultPanic:
	default:
		return errors.New("access audit: invalid outcome")
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	} else {
		input.OccurredAt = input.OccurredAt.UTC()
	}
	seed := strings.Join([]string{
		input.OperationID, input.TenantID, input.ActorSubject, input.Target, input.EventKey,
	}, "\x00")
	auditID := "audit-" + TokenHash(seed)[:32]
	base := auditEventRecord{
		AuditID: auditID, TenantID: input.TenantID,
		ActorSubject: input.ActorSubject, ActorUserID: input.ActorUserID,
		AuthMethod: input.AuthMethod, AuthChannel: input.AuthChannel, SessionRef: input.SessionRef,
		RequestID: input.RequestID, TraceID: input.TraceID, IdempotencyRef: input.IdempotencyRef,
		OperationID: input.OperationID, Module: input.Module, Target: input.Target,
		ResourceTenantID: input.ResourceTenantID, DecisionReason: input.DecisionReason,
		RequestDigest: input.RequestDigest, ReceiptRef: input.ReceiptRef, Reason: input.Reason,
		Risk: input.Risk, OccurredAt: input.OccurredAt,
	}
	attempt := base
	attempt.EventID = auditID + "-a"
	attempt.EventType = domain.AuditEventAttempt
	attempt.Outcome = domain.AuditResultPending
	outcome := base
	outcome.EventID = auditID + "-o"
	outcome.EventType = domain.AuditEventOutcome
	outcome.Outcome = input.Outcome
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&attempt).Error; err != nil {
		return err
	}
	return tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&outcome).Error
}

func safeAuditToken(value string, max int) string {
	value = strings.TrimSpace(value)
	if max > 0 && len(value) > max {
		value = value[:max]
	}
	var builder strings.Builder
	for _, char := range value {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || char == '_' || char == '-' || char == '.' || char == ':' {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func safeAuditText(value string, max int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if max > 0 && len(runes) > max {
		runes = runes[:max]
	}
	for i, char := range runes {
		if char < 0x20 || char == 0x7f {
			runes[i] = ' '
		}
	}
	return strings.TrimSpace(string(runes))
}
