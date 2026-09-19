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
	"yunka.io/framework/requestscope"
)

type auditEventRecord struct {
	EventID          string    `gorm:"column:event_id;primaryKey;size:64"`
	AuditID          string    `gorm:"column:audit_id;size:64;not null;uniqueIndex:uniq_audit_event_kind,priority:1;index:idx_audit_tenant_time,priority:2"`
	EventType        string    `gorm:"column:event_type;size:16;not null;uniqueIndex:uniq_audit_event_kind,priority:2"`
	TenantID         string    `gorm:"column:tenant_id;size:64;not null;index:idx_audit_tenant_time,priority:1;index:idx_audit_tenant_operation,priority:1"`
	ActorSubject     string    `gorm:"column:actor_subject;size:200;not null"`
	ActorUserID      string    `gorm:"column:actor_user_id;size:64;not null;default:''"`
	AuthMethod       string    `gorm:"column:auth_method;size:32;not null;default:''"`
	AuthChannel      string    `gorm:"column:auth_channel;size:32;not null;default:''"`
	SessionRef       string    `gorm:"column:session_ref;size:80;not null;default:''"`
	RequestID        string    `gorm:"column:request_id;size:128;not null;default:''"`
	TraceID          string    `gorm:"column:trace_id;size:128;not null;default:''"`
	IdempotencyRef   string    `gorm:"column:idempotency_ref;size:80;not null;default:''"`
	OperationID      string    `gorm:"column:operation_id;size:160;not null;index:idx_audit_tenant_operation,priority:2"`
	Module           string    `gorm:"column:module;size:80;not null;default:''"`
	Target           string    `gorm:"column:target;size:320;not null;default:''"`
	ResourceTenantID string    `gorm:"column:resource_tenant_id;size:64;not null;default:''"`
	DecisionReason   string    `gorm:"column:decision_reason;size:64;not null;default:''"`
	RequestDigest    string    `gorm:"column:request_digest;size:64;not null;default:''"`
	ReceiptRef       string    `gorm:"column:receipt_ref;size:128;not null;default:''"`
	Reason           string    `gorm:"column:reason;size:500;not null;default:''"`
	Risk             string    `gorm:"column:risk;size:16;not null"`
	Outcome          string    `gorm:"column:outcome;size:16;not null"`
	OccurredAt       time.Time `gorm:"column:occurred_at;not null;index:idx_audit_tenant_time,priority:3"`
}

func (auditEventRecord) TableName() string { return "biz_audit_events" }

type AuditRepository struct{ database *gorm.DB }

func NewAuditRepository(database *gorm.DB) (*AuditRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: audit database is required")
	}
	return &AuditRepository{database: database}, nil
}

func (repository *AuditRepository) AppendAuditEvent(ctx context.Context, event domain.AuditEvent) error {
	if repository == nil || repository.database == nil || strings.TrimSpace(event.EventID) == "" || strings.TrimSpace(event.AuditID) == "" || strings.TrimSpace(event.TenantID) == "" {
		return errors.New("access persistence: valid audit event is required")
	}
	row := auditEventRecord{
		EventID: event.EventID, AuditID: event.AuditID, EventType: event.EventType, TenantID: event.TenantID,
		ActorSubject: event.ActorSubject, ActorUserID: event.ActorUserID, AuthMethod: event.AuthMethod, AuthChannel: event.AuthChannel,
		SessionRef: event.SessionRef, RequestID: event.RequestID, TraceID: event.TraceID, IdempotencyRef: event.IdempotencyRef,
		OperationID: event.OperationID, Module: event.Module, Target: event.Target, ResourceTenantID: event.ResourceTenantID,
		DecisionReason: event.DecisionReason, RequestDigest: event.RequestDigest, ReceiptRef: event.ReceiptRef,
		Reason: event.Reason, Risk: event.Risk, Outcome: event.Outcome, OccurredAt: event.OccurredAt.UTC(),
	}
	return repository.database.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

type foldedAuditRow struct {
	AuditID          string    `gorm:"column:audit_id"`
	TenantID         string    `gorm:"column:tenant_id"`
	ActorSubject     string    `gorm:"column:actor_subject"`
	ActorUserID      string    `gorm:"column:actor_user_id"`
	AuthMethod       string    `gorm:"column:auth_method"`
	AuthChannel      string    `gorm:"column:auth_channel"`
	SessionRef       string    `gorm:"column:session_ref"`
	RequestID        string    `gorm:"column:request_id"`
	TraceID          string    `gorm:"column:trace_id"`
	IdempotencyRef   string    `gorm:"column:idempotency_ref"`
	OperationID      string    `gorm:"column:operation_id"`
	Module           string    `gorm:"column:module"`
	Target           string    `gorm:"column:target"`
	ResourceTenantID string    `gorm:"column:resource_tenant_id"`
	DecisionReason   string    `gorm:"column:decision_reason"`
	RequestDigest    string    `gorm:"column:request_digest"`
	ReceiptRef       string    `gorm:"column:receipt_ref"`
	Reason           string    `gorm:"column:reason"`
	Risk             string    `gorm:"column:risk"`
	Result           string    `gorm:"column:result"`
	OccurredAt       time.Time `gorm:"column:occurred_at"`
}

func (repository *AuditRepository) folded(tenantID string, filter domain.AuditFilter) *gorm.DB {
	query := repository.database.Table("biz_audit_events AS a").
		Select(`a.audit_id, a.tenant_id, a.actor_subject, a.actor_user_id, a.auth_method, a.auth_channel,
			a.session_ref, a.request_id, a.trace_id, a.idempotency_ref, a.operation_id, a.module, a.target,
			a.resource_tenant_id, COALESCE(o.decision_reason, a.decision_reason, '') AS decision_reason,
			a.request_digest, a.receipt_ref, a.reason, a.risk,
			COALESCE(o.outcome, ?) AS result, a.occurred_at`, domain.AuditResultPending).
		Joins("LEFT JOIN biz_audit_events AS o ON o.audit_id = a.audit_id AND o.event_type = ?", domain.AuditEventOutcome).
		Where("a.tenant_id = ? AND a.event_type = ?", tenantID, domain.AuditEventAttempt)
	if value := strings.TrimSpace(filter.OperationID); value != "" {
		query = query.Where("a.operation_id = ?", value)
	}
	if value := strings.TrimSpace(filter.Risk); value != "" {
		query = query.Where("a.risk = ?", value)
	}
	if value := strings.TrimSpace(filter.Result); value != "" {
		query = query.Where("COALESCE(o.outcome, ?) = ?", domain.AuditResultPending, value)
	}
	if value := strings.TrimSpace(filter.Query); value != "" {
		pattern := "%" + value + "%"
		query = query.Where("(a.operation_id LIKE ? OR a.target LIKE ? OR a.actor_subject LIKE ? OR a.request_id LIKE ?)", pattern, pattern, pattern, pattern)
	}
	return query
}

func (repository *AuditRepository) ListAuditRecords(ctx context.Context, tenantID string, filter domain.AuditFilter) ([]domain.AuditRecord, uint64, error) {
	if repository == nil || repository.database == nil || strings.TrimSpace(tenantID) == "" {
		return nil, 0, errors.New("access persistence: audit tenant is required")
	}
	base := repository.folded(tenantID, filter).WithContext(ctx)
	var count int64
	if err := base.Session(&gorm.Session{}).Count(&count).Error; err != nil {
		return nil, 0, err
	}
	limit := filter.Limit
	if limit <= 0 || limit > 5000 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	var rows []foldedAuditRow
	if err := repository.folded(tenantID, filter).WithContext(ctx).
		Order("a.occurred_at DESC, a.audit_id DESC").Offset(offset).Limit(limit).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	result := make([]domain.AuditRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.domain())
	}
	return result, uint64(count), nil
}

func (repository *AuditRepository) GetAuditRecord(ctx context.Context, tenantID, auditID string) (domain.AuditRecord, error) {
	if repository == nil || repository.database == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(auditID) == "" {
		return domain.AuditRecord{}, ports.ErrAuditRecordNotFound
	}
	var row foldedAuditRow
	if err := repository.folded(tenantID, domain.AuditFilter{}).WithContext(ctx).Where("a.audit_id = ?", auditID).Limit(1).Scan(&row).Error; err != nil {
		return domain.AuditRecord{}, err
	}
	if row.AuditID == "" {
		return domain.AuditRecord{}, ports.ErrAuditRecordNotFound
	}
	return row.domain(), nil
}

func (row foldedAuditRow) domain() domain.AuditRecord {
	return domain.AuditRecord{
		AuditID: row.AuditID, TenantID: row.TenantID, ActorSubject: row.ActorSubject, ActorUserID: row.ActorUserID,
		AuthMethod: row.AuthMethod, AuthChannel: row.AuthChannel, SessionRef: row.SessionRef, RequestID: row.RequestID,
		TraceID: row.TraceID, IdempotencyRef: row.IdempotencyRef, OperationID: row.OperationID, Module: row.Module, Target: row.Target,
		ResourceTenantID: row.ResourceTenantID, DecisionReason: row.DecisionReason, RequestDigest: row.RequestDigest,
		ReceiptRef: row.ReceiptRef, Reason: row.Reason, Risk: row.Risk, Result: row.Result, OccurredAt: row.OccurredAt,
	}
}

func NewTenantAuditRepositoryFactory(database *gorm.DB) (requestscope.RepositoryFactory[ports.TenantAuditRepositories], error) {
	if database == nil {
		return nil, errors.New("access persistence: audit database is required")
	}
	return requestscope.GORMRepositories(func(_ context.Context, transaction *gorm.DB) (ports.TenantAuditRepositories, error) {
		repository, err := NewAuditRepository(transaction)
		if err != nil {
			return ports.TenantAuditRepositories{}, err
		}
		return ports.TenantAuditRepositories{Audit: repository}, nil
	}), nil
}
