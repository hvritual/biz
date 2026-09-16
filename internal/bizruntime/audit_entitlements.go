package bizruntime

import (
	"context"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/commercial/enforcement"
)

type checkedTenantAudit struct {
	inner accessapp.TenantAuditManagementApplication
}

func (w checkedTenantAudit) ListTenantAuditRecords(ctx context.Context, r *accessv1.ListTenantAuditRecordsRequest) (*accessv1.ListTenantAuditRecordsResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "access.audit.list"); err != nil {
		return nil, err
	}
	v, err := w.inner.ListTenantAuditRecords(ctx, r)
	return v, enforcement.ExecutionError(ctx, "access.audit.list", err)
}

func (w checkedTenantAudit) GetTenantAuditRecord(ctx context.Context, r *accessv1.GetTenantAuditRecordRequest) (*accessv1.TenantAuditRecordDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "access.audit.get"); err != nil {
		return nil, err
	}
	v, err := w.inner.GetTenantAuditRecord(ctx, r)
	return v, enforcement.ExecutionError(ctx, "access.audit.get", err)
}

func (w checkedTenantAudit) ExportTenantAuditRecords(ctx context.Context, r *accessv1.ExportTenantAuditRecordsRequest) (*accessv1.ExportTenantAuditRecordsResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "access.audit.export"); err != nil {
		return nil, err
	}
	v, err := w.inner.ExportTenantAuditRecords(ctx, r)
	return v, enforcement.ExecutionError(ctx, "access.audit.export", err)
}

var _ accessapp.TenantAuditManagementApplication = checkedTenantAudit{}
