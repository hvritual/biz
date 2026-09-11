package ports

import (
	"context"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"time"
)

type EntitlementState struct {
	Version uint64
	Sources []entitlement.Source
}
type OverrideReceipt struct {
	Source        entitlement.Source `json:"source"`
	SourceVersion uint64             `json:"source_version"`
}
type EntitlementAudit struct {
	TenantID      string
	ActorID       string
	Action        string
	Reason        string
	RequestID     string
	BeforeVersion uint64
	AfterVersion  uint64
	Before        *entitlement.Source
	After         entitlement.Source
	At            time.Time
}

type EntitlementRepository interface {
	Read(context.Context, string) (EntitlementState, error)
	Lock(context.Context, string) (EntitlementState, error)
	Receipt(context.Context, string, string, string) (*OverrideReceipt, error)
	Insert(context.Context, entitlement.Source) error
	Revoke(context.Context, entitlement.Source, uint64) error
	Advance(context.Context, string, uint64) error
	Audit(context.Context, EntitlementAudit) error
	SaveReceipt(context.Context, string, string, string, OverrideReceipt) error
}
type EntitlementRepositories struct {
	Entitlements EntitlementRepository
	Transitions  TimeTransitionRepository
}

// Providers must return real immutable, tenant-scoped source data or an error.
// CE-04 installs no plan/add-on providers; nil is intentionally not a fake plan.
type EntitlementSourceProvider interface {
	Kind() entitlement.SourceKind
	Load(context.Context, string, time.Time) ([]entitlement.Source, error)
}
