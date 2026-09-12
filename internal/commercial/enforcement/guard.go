// Package enforcement applies commercial constraints after the existing IAM
// authorizer. It never grants IAM permissions or creates an Executor/UoW.
package enforcement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	commercialassets "github.com/hvritual/biz/contracts/commercial"
	"github.com/hvritual/biz/internal/commercial/capabilitymap"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/domain/snapshot"
	"github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
	"github.com/hvritual/biz/internal/commercial/ports"
	"log/slog"
	"sort"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/core/runtimecontext"
	"yunka.io/framework/execution"
	"yunka.io/gateway/authz"
)

type AuditEvent struct {
	CorrelationID  string `json:"correlation_id"`
	TenantID       string `json:"tenant_id"`
	Subject        string `json:"subject"`
	RootOperation  string `json:"root_operation"`
	Operation      string `json:"operation"`
	Stage          string `json:"stage"`
	Allowed        bool   `json:"allowed"`
	Reason         string `json:"reason"`
	SourceVersion  uint64 `json:"source_version"`
	MappingVersion string `json:"mapping_version"`
}
type Auditor interface {
	Record(context.Context, AuditEvent)
}
type LogAuditor struct{}

func (LogAuditor) Record(ctx context.Context, e AuditEvent) {
	slog.InfoContext(ctx, "commercial_access", "decision", e)
}

type Guard struct {
	reader     ports.EntitlementDecisionReader
	audit      Auditor
	version    string
	operations map[string]capabilitymap.CompiledOperation
}
type frameKey struct{}
type frame struct {
	guard                 *Guard
	root, subject, tenant string
}

func New(reader ports.EntitlementDecisionReader, audit Auditor) (*Guard, error) {
	var doc capabilitymap.Document
	if err := json.Unmarshal(commercialassets.Catalog(), &doc); err != nil {
		return nil, err
	}
	return newGuard(doc, reader, audit)
}
func newGuard(doc capabilitymap.Document, reader ports.EntitlementDecisionReader, audit Auditor) (*Guard, error) {
	if reader == nil || audit == nil || doc.MappingVersion == "" || len(doc.Operations) == 0 {
		return nil, errors.New("commercial: validated policy, reader and audit required")
	}
	g := &Guard{reader: reader, audit: audit, version: doc.MappingVersion, operations: map[string]capabilitymap.CompiledOperation{}}
	for _, o := range doc.Operations {
		if o.OperationID == "" {
			return nil, errors.New("commercial: empty operation")
		}
		if _, ok := g.operations[o.OperationID]; ok {
			return nil, errors.New("commercial: duplicate operation")
		}
		switch o.Classification {
		case capabilitymap.TenantBusiness:
			if !o.TenantRequired || len(o.AlwaysRequiredCapabilityCodes) == 0 {
				return nil, errors.New("commercial: tenant business has no requirement")
			}
		case capabilitymap.PlatformManagement, capabilitymap.Recovery, capabilitymap.FoundationExempt:
			if o.ExemptionReason == "" {
				return nil, errors.New("commercial: unreasoned exemption")
			}
		default:
			return nil, errors.New("commercial: unknown classification")
		}
		if (o.ConsumerType == "worker") != (o.WorkerPermission != "") {
			return nil, errors.New("commercial: inconsistent worker entry declaration")
		}
		if o.ConsumerType == "worker" && (o.Classification != capabilitymap.PlatformManagement || o.TenantRequired || o.RPC != "") {
			return nil, errors.New("commercial: worker entry has public or tenant authority")
		}
		g.operations[o.OperationID] = o
	}
	for _, o := range doc.Operations {
		for _, c := range o.Children {
			if _, ok := g.operations[c.OperationID]; !ok {
				return nil, errors.New("commercial: unknown child")
			}
		}
	}
	return g, nil
}

// ResolveGuard returns a rejecting commercial guard even for an unknown entry.
// Missing mappings can never fall through a permissive static resolver.
func (g *Guard) ResolveGuard(_ authz.OperationID) (authz.OperationGuard, bool) { return g, true }

func (g *Guard) Prepare(ctx context.Context, a authz.AuthorizedOperation, _ any) (context.Context, error) {
	if g == nil {
		return nil, errors.New("commercial: missing guard")
	}
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || !a.Decision.Allowed || p.Subject != a.Principal.Subject || p.TenantID != a.Principal.TenantID {
		return nil, g.fail(ctx, string(a.Policy.Operation), "root", "ENTITLEMENT_CONTEXT_INVALID", 0, false)
	}
	ctx = ensureOutcome(ctx)
	id := string(a.Policy.Operation)
	o, ok := g.operations[id]
	if !ok {
		return nil, g.fail(ctx, id, "root", "OPERATION_NOT_CLASSIFIED", 0, false)
	}
	if o.ConsumerType == "internal_child" {
		return nil, g.fail(ctx, id, "root", "INTERNAL_OPERATION_ROOT_DENIED", 0, false)
	}
	if o.ConsumerType == "worker" {
		w, present := ctx.Value(workerContextKey{}).(workerContext)
		if !present || w.guard != g || w.subject != p.Subject || w.tenant != p.TenantID || p.TenantID != "" || a.Policy.Mode != authz.PermissionAll || !hasWorkerPermission(a.Policy.Permissions, o.WorkerPermission) {
			return nil, g.fail(ctx, id, "root", "WORKER_CONTEXT_REQUIRED", 0, false)
		}
	}
	if o.ConsumerType == "platform" && p.TenantID != "" {
		return nil, g.fail(ctx, id, "root", "PLATFORM_CONTEXT_REQUIRED", 0, false)
	}
	if o.TenantRequired && p.TenantID == "" {
		return nil, g.fail(ctx, id, "root", "TENANT_CONTEXT_REQUIRED", 0, false)
	}
	ctx = context.WithValue(ctx, frameKey{}, frame{guard: g, root: id, subject: p.Subject, tenant: p.TenantID})
	if err := g.check(ctx, o, "root"); err != nil {
		return nil, err
	}
	return ctx, nil
}

// RequireExecuted is called by the typed DeviceOps Application decorators,
// including generated child adapters. The root token is request-bound; a raw
// Application invocation or a different principal/operation cannot reuse it.
func RequireExecuted(ctx context.Context, id string) error {
	if ctx == nil {
		return errors.New("commercial: context required")
	}
	f, ok := ctx.Value(frameKey{}).(frame)
	if !ok || f.guard == nil {
		return authz.Denied(authz.Decision{Operation: authz.OperationID(id), Reason: authz.Reason("ENTITLEMENT_CONTEXT_REQUIRED")})
	}
	g := f.guard
	p, present := identity.FromContext(ctx)
	metadata, metaOK := runtimecontext.MetadataFrom(ctx)
	_, scopeOK := execution.Current(ctx)
	a, authOK := authz.AuthorizedOperationFromContext(ctx)
	if !present || p.Subject != f.subject || p.TenantID != f.tenant || !p.Authenticated || !metaOK || metadata.Operation != id || !scopeOK || !authOK || !a.Decision.Allowed || string(a.Policy.Operation) != f.root {
		return g.fail(ctx, id, "application", "ENTITLEMENT_CONTEXT_INVALID", 0, false)
	}
	if !g.reachable(f.root, id, map[string]bool{}) {
		return g.fail(ctx, id, "application", "UNDECLARED_CHILD_OPERATION", 0, false)
	}
	o, exists := g.operations[id]
	if !exists {
		return g.fail(ctx, id, "application", "OPERATION_NOT_CLASSIFIED", 0, false)
	}
	return g.check(ctx, o, "application")
}
func (g *Guard) reachable(from, to string, seen map[string]bool) bool {
	if from == to {
		return true
	}
	if seen[from] {
		return false
	}
	seen[from] = true
	for _, child := range g.operations[from].Children {
		if g.reachable(child.OperationID, to, seen) {
			return true
		}
	}
	return false
}
func (g *Guard) check(ctx context.Context, o capabilitymap.CompiledOperation, stage string) error {
	if o.Classification != capabilitymap.TenantBusiness {
		g.record(ctx, o.OperationID, stage, true, "EXPLICIT_COMMERCIAL_EXEMPTION", 0)
		return nil
	}
	required := append([]string(nil), o.AlwaysRequiredCapabilityCodes...)
	sort.Strings(required)
	result, err := g.reader.Decide(ctx, required)
	if err != nil {
		reason := "ENTITLEMENT_SOURCE_UNAVAILABLE"
		switch {
		case errors.Is(err, consistency.ErrRetryRequired) || consistency.Transient(err):
			reason = "ENTITLEMENT_RETRY_REQUIRED"
		case errors.Is(err, snapshot.ErrStale):
			reason = "ENTITLEMENT_SNAPSHOT_STALE"
		case errors.Is(err, consistency.ErrStateLost):
			reason = "ENTITLEMENT_AUTHORITY_STATE_LOST"
		}
		return g.fail(ctx, o.OperationID, stage, reason, 0, true)
	}
	p, _ := identity.FromContext(ctx)
	if result.TenantID != p.TenantID {
		return g.fail(ctx, o.OperationID, stage, "ENTITLEMENT_CONTEXT_INVALID", 0, true)
	}
	for _, code := range required {
		found := false
		for _, decision := range result.Decisions {
			if decision.Kind == entitlement.Capability && decision.Key == code {
				found = true
				if !decision.Allowed {
					return g.fail(ctx, o.OperationID, stage, decision.Reason, result.SourceVersion, false)
				}
				break
			}
		}
		if !found {
			return g.fail(ctx, o.OperationID, stage, "MODULE_NOT_ENTITLED", result.SourceVersion, false)
		}
	}
	g.record(ctx, o.OperationID, stage, true, "ALLOWED", result.SourceVersion)
	return nil
}
func (g *Guard) record(ctx context.Context, id, stage string, allowed bool, reason string, revision uint64) {
	p, _ := identity.FromContext(ctx)
	f, _ := ctx.Value(frameKey{}).(frame)
	root := f.root
	if root == "" {
		root = id
	}
	g.audit.Record(ctx, AuditEvent{CorrelationID: correlation(ctx), TenantID: p.TenantID, Subject: p.Subject, RootOperation: root, Operation: id, Stage: stage, Allowed: allowed, Reason: reason, SourceVersion: revision, MappingVersion: g.version})
}
func (g *Guard) fail(ctx context.Context, id, stage, reason string, revision uint64, unavailable bool) error {
	f := &Failure{Code: reason, Operation: id, CorrelationID: correlation(ctx), Unavailable: unavailable}
	remember(ctx, f)
	g.record(ctx, id, stage, false, reason, revision)
	return f
}

type Failure struct {
	Code          string `json:"code"`
	Operation     string `json:"operation"`
	CorrelationID string `json:"correlation_id"`
	Unavailable   bool   `json:"-"`
}

func (f *Failure) Error() string { return fmt.Sprintf("%s: %s", f.Code, f.Operation) }
func (f *Failure) Unwrap() error {
	return authz.Denied(authz.Decision{Operation: authz.OperationID(f.Operation), Reason: authz.Reason(f.Code)})
}

// ExecutionError marks a transient SQL failure before transport projection.
// Never retry a SQL statement inside a potentially rolled-back root.
func ExecutionError(ctx context.Context, id string, err error) error {
	if err == nil {
		return nil
	}
	if !consistency.Transient(err) && !errors.Is(err, consistency.ErrRetryRequired) {
		return err
	}
	f, ok := ctx.Value(frameKey{}).(frame)
	if !ok || f.guard == nil {
		return err
	}
	return f.guard.fail(ctx, id, "transaction", "ENTITLEMENT_RETRY_REQUIRED", 0, true)
}
