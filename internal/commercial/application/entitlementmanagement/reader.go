package entitlementmanagement

import (
	"context"
	"errors"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/ports"
	"yunka.io/framework/core/identity"
)

type decisionReader struct {
	snapshots ports.EntitlementSnapshotReader
}

func BuildDecisionReader(snapshots ports.EntitlementSnapshotReader) (ports.EntitlementDecisionReader, error) {
	if snapshots == nil {
		return nil, errors.New("commercial: snapshot authority required")
	}
	return &decisionReader{snapshots: snapshots}, nil
}
func (r *decisionReader) Decide(ctx context.Context, requested []string) (entitlement.Result, error) {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.TenantID == "" || p.Subject == "" {
		return entitlement.Result{}, entitlement.ErrScope
	}
	return r.snapshots.ReadSnapshot(ctx, p.TenantID, requested)
}
