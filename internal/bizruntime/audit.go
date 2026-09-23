package bizruntime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/framework/execution"
	"yunka.io/framework/operation"
	"yunka.io/pkg/operationplan"
)

type auditStateContextKey struct{}

type auditState struct{ event domain.AuditEvent }

func newAuditedOperationExecutor(database *gorm.DB, security operation.SecurityPhase, transactions execution.TransactionFactory, idempotency execution.IdempotencyCoordinator) (operation.Executor, error) {
	repository, err := accesspersistence.NewAuditRepository(database)
	if err != nil {
		return nil, err
	}
	wrapped := auditedSecurity{inner: security, sink: repository}
	return operation.NewExecutorWithOptions(wrapped, operation.ExecutorOptions{Transactions: transactions, Idempotency: idempotency}, auditObserver{sink: repository}), nil
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
		outcome.DecisionReason = authorizationDecisionReason(err)
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

type auditObserver struct {
	sink accessports.AuditEventAppender
}

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
	transport, _ := runtimecontext.MetadataFrom(ctx)
	idempotencyRef := opaqueRef(execution.IdempotencyKeyFrom(ctx))
	seed := principal.TenantID + "\x00" + plan.OperationID + "\x00"
	if idempotencyRef != "" {
		seed += idempotencyRef
	} else if transport.RequestID != "" {
		seed += transport.RequestID
	} else {
		seed += uuid.NewString()
	}
	sum := sha256.Sum256([]byte(seed))
	auditID := "audit-" + hex.EncodeToString(sum[:16])
	target, reason := auditTargetAndReason(input, principal.TenantID)
	resourceTenantID := auditResourceTenant(input, principal.TenantID)
	requestDigest := digestRequest(input)
	receiptRef := ""
	if idempotencyRef != "" {
		receiptRef = "idempotency:" + idempotencyRef
	} else if transport.RequestID != "" {
		receiptRef = "request:" + transport.RequestID
	}
	attributes := transport.Attributes
	return auditState{event: domain.AuditEvent{
		EventID: auditID + "-a", AuditID: auditID, EventType: domain.AuditEventAttempt,
		TenantID: principal.TenantID, ActorSubject: principal.Subject, ActorUserID: principal.UserID,
		AuthMethod: principal.AuthMethod, AuthChannel: attributes["auth_channel"], SessionRef: attributes["session_ref"],
		RequestID: transport.RequestID, TraceID: transport.RequestID, IdempotencyRef: idempotencyRef, OperationID: plan.OperationID,
		Module: plan.Domain, Target: target, ResourceTenantID: resourceTenantID, RequestDigest: requestDigest,
		ReceiptRef: receiptRef, Reason: reason,
		Risk: auditRisk(plan.OperationID), Outcome: domain.AuditResultPending, OccurredAt: time.Now().UTC(),
	}}
}

func withAuditHTTPMetadata(request *http.Request, principal identity.Principal, webAuth *runtimeWebAuth) *http.Request {
	if request == nil {
		return request
	}
	ctx := identity.WithPrincipal(request.Context(), principal)
	current, _ := runtimecontext.MetadataFrom(ctx)
	attributes := cloneAuditAttributes(current.Attributes)
	attributes["auth_channel"] = "api-key"
	secret := parseBearer(request.Header.Get("Authorization"))
	if secret == "" {
		attributes["auth_channel"] = "web-session"
		if webAuth != nil {
			if cookie, err := request.Cookie(webAuth.sessionCookieName()); err == nil {
				secret = cookie.Value
			}
		}
	}
	attributes["session_ref"] = opaqueRef(secret)
	current.Transport = "http"
	current.Protocol = request.Proto
	current.Route = request.URL.Path
	current.Method = request.Method
	current.RequestID = uuid.NewString()
	current.Attributes = attributes
	return request.WithContext(runtimecontext.WithMetadata(ctx, current))
}

func withAuditGRPCMetadata(ctx context.Context, principal identity.Principal, incoming metadata.MD) context.Context {
	ctx = identity.WithPrincipal(ctx, principal)
	current, _ := runtimecontext.MetadataFrom(ctx)
	attributes := cloneAuditAttributes(current.Attributes)
	attributes["auth_channel"] = "api-key"
	secret := ""
	if values := incoming.Get("authorization"); len(values) > 0 {
		secret = parseBearer(values[0])
	}
	attributes["session_ref"] = opaqueRef(secret)
	current.Transport = "grpc"
	current.Protocol = "grpc"
	current.RequestID = uuid.NewString()
	current.Attributes = attributes
	return runtimecontext.WithMetadata(ctx, current)
}

func cloneAuditAttributes(source map[string]string) map[string]string {
	result := make(map[string]string, len(source)+2)
	for key, value := range source {
		result[key] = value
	}
	return result
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
				return string(name) + ":" + value, auditReason(reflection, fields)
			}
		}
	}
	return "tenant:" + tenantID, auditReason(reflection, fields)
}

func auditResourceTenant(input any, trustedTenantID string) string {
	message, ok := input.(proto.Message)
	if !ok || message == nil {
		return trustedTenantID
	}
	reflection := message.ProtoReflect()
	field := reflection.Descriptor().Fields().ByName("tenant_id")
	if field == nil || field.Kind() != protoreflect.StringKind || !reflection.Has(field) {
		return trustedTenantID
	}
	value := strings.TrimSpace(reflection.Get(field).String())
	if value == "" {
		return trustedTenantID
	}
	return value
}

func authorizationDecisionReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, operation.ErrSecurityUnavailable):
		return "SECURITY_UNAVAILABLE"
	case errors.Is(err, operation.ErrSecurityNilContext):
		return "SECURITY_INVALID_CONTEXT"
	default:
		return "AUTHORIZATION_DENIED"
	}
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
	for _, marker := range []string{"role", "permission", "business_scope", "suspend", "remove", "delete", "close", "transfer", "subscription.change.confirm", "audit.export"} {
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
