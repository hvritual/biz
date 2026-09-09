package ports

import (
	"context"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
)

// Preflight query ports do not open a second Executor/UoW. Each owner supplies
// its own read-only projection. They are not writable cross-owner repositories.
type EntitlementStateReader interface {
	ReadCurrent(context.Context, string) (EntitlementState, error)
}
type EntitlementCatalogReader interface {
	ReadCurrentCatalog(context.Context) (entitlement.Catalog, error)
}
type EntitlementDecisionReader interface {
	Decide(context.Context, []string) (entitlement.Result, error)
}
