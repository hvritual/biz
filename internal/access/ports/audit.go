package ports

import (
	"context"
	"errors"

	"github.com/hvritual/biz/internal/access/domain"
)

var ErrAuditRecordNotFound = errors.New("access: audit record not found")

type AuditEventAppender interface {
	AppendAuditEvent(context.Context, domain.AuditEvent) error
}

type TenantAuditRepository interface {
	AuditEventAppender
	ListAuditRecords(context.Context, string, domain.AuditFilter) ([]domain.AuditRecord, uint64, error)
	GetAuditRecord(context.Context, string, string) (domain.AuditRecord, error)
}

type TenantAuditRepositories struct {
	Audit TenantAuditRepository
}
