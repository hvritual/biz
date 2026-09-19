package application

import (
	"testing"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
)

func TestEnterprise174AuditDTOMapsAuthorizationEvidence(t *testing.T) {
	occurredAt := time.Date(2026, time.September, 19, 6, 30, 0, 123456789, time.UTC)
	dto := auditDTO(domain.AuditRecord{
		AuditID:          "audit-174",
		RequestID:        "request-174",
		TraceID:          "trace-174",
		ResourceTenantID: "tenant-resource",
		DecisionReason:   "AUTHORIZATION_DENIED",
		OccurredAt:       occurredAt,
	})

	if dto.GetAuditId() != "audit-174" {
		t.Fatalf("audit id mismatch: %q", dto.GetAuditId())
	}
	if dto.GetRequestId() != "request-174" {
		t.Fatalf("request id mismatch: %q", dto.GetRequestId())
	}
	if dto.GetTraceId() != "trace-174" {
		t.Fatalf("trace id mismatch: %q", dto.GetTraceId())
	}
	if dto.GetResourceTenantId() != "tenant-resource" {
		t.Fatalf("resource tenant id mismatch: %q", dto.GetResourceTenantId())
	}
	if dto.GetDecisionReason() != "AUTHORIZATION_DENIED" {
		t.Fatalf("decision reason mismatch: %q", dto.GetDecisionReason())
	}
	if dto.GetOccurredAt() != occurredAt.Format(time.RFC3339Nano) {
		t.Fatalf("occurred_at mismatch: %q", dto.GetOccurredAt())
	}
}
