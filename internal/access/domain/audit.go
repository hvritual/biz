package domain

import "time"

const (
	AuditEventAttempt = "attempt"
	AuditEventOutcome = "outcome"

	AuditResultPending = "pending"
	AuditResultSuccess = "success"
	AuditResultFailure = "failure"
	AuditResultPanic   = "panic"

	AuditRiskLow    = "low"
	AuditRiskMedium = "medium"
	AuditRiskHigh   = "high"
)

type AuditEvent struct {
	EventID          string
	AuditID          string
	EventType        string
	TenantID         string
	ActorSubject     string
	ActorUserID      string
	AuthMethod       string
	AuthChannel      string
	SessionRef       string
	RequestID        string
	TraceID          string
	IdempotencyRef   string
	OperationID      string
	Module           string
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

type AuditRecord struct {
	AuditID          string
	TenantID         string
	ActorSubject     string
	ActorUserID      string
	AuthMethod       string
	AuthChannel      string
	SessionRef       string
	RequestID        string
	TraceID          string
	IdempotencyRef   string
	OperationID      string
	Module           string
	Target           string
	ResourceTenantID string
	DecisionReason   string
	RequestDigest    string
	ReceiptRef       string
	Reason           string
	Risk             string
	Result           string
	OccurredAt       time.Time
}

type AuditFilter struct {
	Query       string
	OperationID string
	Result      string
	Risk        string
	Offset      int
	Limit       int
}
