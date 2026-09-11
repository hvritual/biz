package ports

import (
	"context"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	p "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	"time"
)

// ProvisioningPolicy is trusted local configuration. It must not call external
// services while the caller holds its root transaction. Ordered requirements
// are bound into the immutable preview.
type ProvisioningPolicy interface {
	Requirements(context.Context, plan.Version) ([]p.Requirement, error)
}
type DatabaseOnlyProvisioning struct{}

func (DatabaseOnlyProvisioning) Requirements(context.Context, plan.Version) ([]p.Requirement, error) {
	return nil, nil
}

// Both adapter calls execute outside database transactions. Every retry keeps
// the same key. Reconcile inspects actual provider state after uncertainty;
// elapsed time or blindly resending a write is not successful reconciliation.
type PreparationRequest struct {
	TaskID            string
	TenantID          string
	ChangeID          string
	TargetPlanCode    string
	TargetPlanVersion uint64
	Step              p.Requirement
	IdempotencyKey    string
}
type PreparationAdapter interface {
	Prepare(context.Context, PreparationRequest) p.Observation
	Reconcile(context.Context, PreparationRequest) p.Observation
}
type RegisteredPreparation struct {
	AdapterID string
	Version   string
	Adapter   PreparationAdapter
}

type ProvisioningRepository interface {
	// Mutable paths lock catalog shared -> subscription -> task in the same root.
	LockTask(context.Context, string, string) (*p.Task, error)
	Get(context.Context, string, string) (*p.Task, error)
	List(context.Context, string, string, uint32) ([]p.Task, error)
	Due(context.Context, time.Time, uint32) ([]p.Task, error)
	Insert(context.Context, p.Task) error
	Save(context.Context, p.Task, p.Task, string, string) error
	ActionReceipt(context.Context, string, string, string, string) (*p.Task, error)
	StoreActionReceipt(context.Context, string, string, string, string, p.Task) error
	Now(context.Context) (time.Time, error)
}
type OutboxRepository interface {
	Append(context.Context, p.Event) error
	Claim(context.Context, string, time.Duration) (*p.Delivery, error)
	Deliver(context.Context, string, string, uint64) (p.DeliveryReceipt, error)
	Fail(context.Context, string, string, uint64, string) error
	List(context.Context, string, string, uint32) ([]p.Delivery, error)
}
type ProvisioningRepositories struct {
	Tasks  ProvisioningRepository
	Events OutboxRepository
}
