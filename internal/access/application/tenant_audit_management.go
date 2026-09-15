package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/framework/requestscope"
)

var ErrInvalidTenantAuditRequest = errors.New("access: invalid tenant audit request")

type TenantAuditManagementService struct {
	repositories requestscope.RepositoryFactory[ports.TenantAuditRepositories]
}

func NewTenantAuditManagementService(repositories requestscope.RepositoryFactory[ports.TenantAuditRepositories]) (*TenantAuditManagementService, error) {
	if repositories == nil {
		return nil, errors.New("access: tenant audit repository factory is required")
	}
	return &TenantAuditManagementService{repositories: repositories}, nil
}

func (service *TenantAuditManagementService) ListTenantAuditRecords(ctx context.Context, request *accessv1.ListTenantAuditRecordsRequest) (*accessv1.ListTenantAuditRecordsResponse, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	page, pageSize, filter, err := tenantAuditFilter(request)
	if err != nil {
		return nil, err
	}
	records, total, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantAuditRepositories]) (auditPage, error) {
		items, count, err := scope.Repositories().Audit.ListAuditRecords(scope.Context(), tenantID, filter)
		return auditPage{records: items, total: count}, err
	})
	if err != nil {
		return nil, err
	}
	return &accessv1.ListTenantAuditRecordsResponse{Records: auditDTOs(records.records), Total: records.total, Page: page, PageSize: pageSize}, nil
}

type auditPage struct {
	records []domain.AuditRecord
	total   uint64
}

func (service *TenantAuditManagementService) GetTenantAuditRecord(ctx context.Context, request *accessv1.GetTenantAuditRecordRequest) (*accessv1.TenantAuditRecordDTO, error) {
	if request == nil || strings.TrimSpace(request.GetAuditId()) == "" {
		return nil, ErrInvalidTenantAuditRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	record, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantAuditRepositories]) (domain.AuditRecord, error) {
		return scope.Repositories().Audit.GetAuditRecord(scope.Context(), tenantID, strings.TrimSpace(request.GetAuditId()))
	})
	if err != nil {
		return nil, err
	}
	return auditDTO(record), nil
}

func (service *TenantAuditManagementService) ExportTenantAuditRecords(ctx context.Context, request *accessv1.ExportTenantAuditRecordsRequest) (*accessv1.ExportTenantAuditRecordsResponse, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	filter, err := tenantAuditExportFilter(request)
	if err != nil {
		return nil, err
	}
	page, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantAuditRepositories]) (auditPage, error) {
		records, total, err := scope.Repositories().Audit.ListAuditRecords(scope.Context(), tenantID, filter)
		return auditPage{records: records, total: total}, err
	})
	if err != nil {
		return nil, err
	}
	exportID := "export-" + uuid.NewString()
	if metadata, ok := runtimecontext.MetadataFrom(ctx); ok && strings.TrimSpace(metadata.RequestID) != "" {
		exportID = "export-" + strings.TrimSpace(metadata.RequestID)
	}
	return &accessv1.ExportTenantAuditRecordsResponse{
		ExportId: exportID, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Records: auditDTOs(page.records),
	}, nil
}

func tenantAuditFilter(request *accessv1.ListTenantAuditRecordsRequest) (uint32, uint32, domain.AuditFilter, error) {
	if request == nil {
		request = &accessv1.ListTenantAuditRecordsRequest{}
	}
	page := request.GetPage()
	if page == 0 {
		page = 1
	}
	pageSize := request.GetPageSize()
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		return 0, 0, domain.AuditFilter{}, ErrInvalidTenantAuditRequest
	}
	filter, err := validateAuditFilter(request.GetQuery(), request.GetOperationId(), request.GetResult(), request.GetRisk())
	if err != nil {
		return 0, 0, domain.AuditFilter{}, err
	}
	filter.Offset = int((page - 1) * pageSize)
	filter.Limit = int(pageSize)
	return page, pageSize, filter, nil
}

func tenantAuditExportFilter(request *accessv1.ExportTenantAuditRecordsRequest) (domain.AuditFilter, error) {
	if request == nil {
		request = &accessv1.ExportTenantAuditRecordsRequest{}
	}
	maxRows := request.GetMaxRows()
	if maxRows == 0 {
		maxRows = 1000
	}
	if maxRows > 5000 {
		return domain.AuditFilter{}, ErrInvalidTenantAuditRequest
	}
	filter, err := validateAuditFilter(request.GetQuery(), request.GetOperationId(), request.GetResult(), request.GetRisk())
	if err != nil {
		return domain.AuditFilter{}, err
	}
	filter.Limit = int(maxRows)
	return filter, nil
}

func validateAuditFilter(query, operationID, result, risk string) (domain.AuditFilter, error) {
	result = strings.TrimSpace(result)
	if result != "" && result != domain.AuditResultPending && result != domain.AuditResultSuccess && result != domain.AuditResultFailure && result != domain.AuditResultPanic {
		return domain.AuditFilter{}, ErrInvalidTenantAuditRequest
	}
	risk = strings.TrimSpace(risk)
	if risk != "" && risk != domain.AuditRiskLow && risk != domain.AuditRiskMedium && risk != domain.AuditRiskHigh {
		return domain.AuditFilter{}, ErrInvalidTenantAuditRequest
	}
	return domain.AuditFilter{Query: strings.TrimSpace(query), OperationID: strings.TrimSpace(operationID), Result: result, Risk: risk}, nil
}

func auditDTOs(records []domain.AuditRecord) []*accessv1.TenantAuditRecordDTO {
	result := make([]*accessv1.TenantAuditRecordDTO, 0, len(records))
	for _, record := range records {
		result = append(result, auditDTO(record))
	}
	return result
}

func auditDTO(record domain.AuditRecord) *accessv1.TenantAuditRecordDTO {
	return &accessv1.TenantAuditRecordDTO{
		AuditId: record.AuditID, OccurredAt: record.OccurredAt.UTC().Format(time.RFC3339Nano), ActorSubject: record.ActorSubject,
		ActorUserId: record.ActorUserID, AuthMethod: record.AuthMethod, AuthChannel: record.AuthChannel, SessionRef: record.SessionRef,
		RequestId: record.RequestID, IdempotencyRef: record.IdempotencyRef, OperationId: record.OperationID, Module: record.Module,
		Target: record.Target, Result: record.Result, Risk: record.Risk, ReceiptRef: record.ReceiptRef, Reason: record.Reason,
		RequestDigest: record.RequestDigest,
	}
}
