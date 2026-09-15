package bizruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hvritual/biz/internal/access/domain"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/framework/execution"
	"yunka.io/framework/operation"
	"yunka.io/pkg/operationplan"
)

type auditStateContextKey struct{}

type auditState struct {
	event domain.AuditEvent
}

type auditedSecurity struct {
	inner operation.SecurityPhase
	sink  accessports.AuditEventAppender
}

func (security auditedSecurity) Prepare(ctx context.Context, plan operationplan.Plan, input any) (context.Context, error) {
	if security.inner == nil {
		return nil, operation.ErrSecurityUnavailable
	}
	principal, _ := identity.FromContext(ctx)
	if !shouldAuditOperation(plan, principal) {
		return security.inner.Prepare(ctx, plan, input)
	}
	if security.sink == nil {
		return nil, errors.New("biz audit: audit sink unavailable")
	}
	state := buildAuditState(ctx, plan, input, principal)
	if err := security.sink.AppendAuditEvent(ctx, state.event); err != nil {
		return nil, fmt.Errorf("biz audit: persist attempt: %w", err)
	}
	secured, err := security.inner.Prepare(ctx, plan, input)
	if err != nil {
		outcome := state.event
		outcome.EventID = state.event.AuditID + "-o"
		outcome.EventType = domain.AuditEventOutcome
		outcome.Outcome = domain.AuditResultFailure
		outcome.OccurredAt = time.Now().UTC()
		if auditErr := security.sink.AppendAuditEvent(ctx, outcome); auditErr != nil {
			return nil, errors.Join(err, fmt.Errorf("biz audit: persist denied outcome: %w", auditErr))
		}
		return nil, err
	}
	if secured == nil {
		return nil, operation.ErrSecurityNilContext
	}
	return context.WithValue(secured, auditStateContextKey{}, state), nil
}

type auditObserver struct{ sink accessports.AuditEventAppender }

func (observer auditObserver) Observe(ctx context.Context, event operation.Event) {
	if observer.sink == nil || event.Kind != operation.InvocationRoot || event.Phase != operation.PhaseOutcome || event.Outcome == operation.OutcomeStarted {
		return
	}
	state, ok := ctx.Value(auditStateContextKey{}).(auditState)
	if !ok || state.event.AuditID == "" {
		return
	}
	outcome := state.event
	outcome.EventID = state.event.AuditID + "-o"
	outcome.EventType = domain.AuditEventOutcome
	outcome.Outcome = auditOutcome(event.Outcome)
	outcome.OccurredAt = time.Now().UTC()
	_ = observer.sink.AppendAuditEvent(ctx, outcome)
}

func shouldAuditOperation(plan operationplan.Plan, principal identity.Principal) bool {
	if !principal.Authenticated || strings.TrimSpace(principal.TenantID) == "" || !plan.Security.TenantRequired {
		return false
	}
	if plan.OperationID == "access.audit.export" {
		return true
	}
	return plan.Execution.Transaction != "read_only"
}

func buildAuditState(ctx context.Context, plan operationplan.Plan, input any, principal identity.Principal) auditState {
	metadata, _ := runtimecontext.MetadataFrom(ctx)
	idempotencyRef := opaqueRef(execution.IdempotencyKeyFrom(ctx))
	seed := principal.TenantID + "\x00" + plan.OperationID + "\x00"
	if idempotencyRef != "" {
		seed += idempotencyRef
	} else if metadata.RequestID != "" {
		seed += metadata.RequestID
	} else {
		seed += uuid.NewString()
	}
	sum := sha256.Sum256([]byte(seed))
	auditID := "audit-" + hex.EncodeToString(sum[:16])
	target, reason := auditTargetAndReason(input, principal.TenantID)
	requestDigest := digestRequest(input)
	receiptRef := ""
	if idempotencyRef != "" {
		receiptRef = "idempotency:" + idempotencyRef
	} else if metadata.RequestID != "" {
		receiptRef = "request:" + metadata.RequestID
	}
	attributes := metadata.Attributes
	return auditState{event: domain.AuditEvent{
		EventID: auditID + "-a", AuditID: auditID, EventType: domain.AuditEventAttempt,
		TenantID: principal.TenantID, ActorSubject: principal.Subject, ActorUserID: principal.UserID,
		AuthMethod: principal.AuthMethod, AuthChannel: attributes["auth_channel"], SessionRef: attributes["session_ref"],
		RequestID: metadata.RequestID, IdempotencyRef: idempotencyRef, OperationID: plan.OperationID,
		Module: plan.Domain, Target: target, RequestDigest: requestDigest, ReceiptRef: receiptRef, Reason: reason,
		Risk: auditRisk(plan.OperationID), Outcome: domain.AuditResultPending, OccurredAt: time.Now().UTC(),
	}}
}

func auditOutcome(value operation.Outcome) string {
	switch value {
	case operation.OutcomeSuccess:
		return domain.AuditResultSuccess
	case operation.OutcomePanic:
		return domain.AuditResultPanic
	default:
		return domain.AuditResultFailure
	}
}

func opaqueRef(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(sum[:12])
}

func digestRequest(input any) string {
	message, ok := input.(proto.Message)
	if !ok || message == nil {
		return ""
	}
	data, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func auditTargetAndReason(input any, tenantID string) (string, string) {
	message, ok := input.(proto.Message)
	if !ok || message == nil {
		return "tenant:" + tenantID, ""
	}
	reflection := message.ProtoReflect()
	fields := reflection.Descriptor().Fields()
	for _, name := range []protoreflect.Name{"member_id", "user_id", "role_id", "department_id", "delegation_id", "change_id", "target_plan_code", "plan_code", "device_id", "site_id", "id"} {
		if field := fields.ByName(name); field != nil && field.Kind() == protoreflect.StringKind && reflection.Has(field) {
			if value := strings.TrimSpace(reflection.Get(field).String()); value != "" {
				reason := auditReason(reflection, fields)
				return string(name) + ":" + value, reason
			}
		}
	}
	return "tenant:" + tenantID, auditReason(reflection, fields)
}

func auditReason(message protoreflect.Message, fields protoreflect.FieldDescriptors) string {
	field := fields.ByName("reason")
	if field == nil || field.Kind() != protoreflect.StringKind || !message.Has(field) {
		return ""
	}
	value := strings.TrimSpace(message.Get(field).String())
	if len([]rune(value)) > 500 {
		return string([]rune(value)[:500])
	}
	return value
}

func auditRisk(operationID string) string {
	value := strings.ToLower(operationID)
	for _, marker := range []string{"role", "permission", "suspend", "remove", "delete", "close", "transfer", "subscription.change.confirm", "audit.export"} {
		if strings.Contains(value, marker) {
			return domain.AuditRiskHigh
		}
	}
	for _, marker := range []string{"create", "update", "activate", "invite", "profile", "department", "organization", "switch"} {
		if strings.Contains(value, marker) {
			return domain.AuditRiskMedium
		}
	}
	return domain.AuditRiskLow
}
