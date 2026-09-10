package bizruntime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"yunka.io/framework/execution"
	"yunka.io/pkg/operationplan"
)

// Tenant creation has an Access-owned transactional response receipt. A
// completed transport claim starts a fresh fenced attempt to read it. Security,
// the single Executor, required header, running exclusion and atomic completion
// are unchanged. All other operations retain their original behavior.
// Access binds the original header AND request_id to the complete payload.
type tenantCreationReplay struct {
	execution.IdempotencyCoordinator
}

func (c tenantCreationReplay) Begin(ctx context.Context, p operationplan.Plan) (context.Context, error) {
	out, err := c.IdempotencyCoordinator.Begin(ctx, p)
	if p.OperationID != "tenant.create" || !errors.Is(err, execution.ErrIdempotencyCompleted) {
		return out, err
	}
	original := execution.IdempotencyKeyFrom(ctx)
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	replay, err := c.IdempotencyCoordinator.Begin(execution.WithIdempotencyKey(ctx, "tenant-replay-"+hex.EncodeToString(nonce[:])), p)
	if err != nil {
		return nil, err
	}
	return execution.WithIdempotencyKey(replay, original), nil
}
func (c tenantCreationReplay) SupportsAtomicCompletion() bool {
	v, ok := c.IdempotencyCoordinator.(execution.IdempotencyCapabilityReporter)
	return ok && v.SupportsAtomicCompletion()
}
func (c tenantCreationReplay) CompleteInTransaction(ctx context.Context, p operationplan.Plan, tx any) error {
	v, ok := c.IdempotencyCoordinator.(execution.AtomicIdempotencyCoordinator)
	if !ok {
		return execution.ErrIdempotencyAtomicUnavailable
	}
	return v.CompleteInTransaction(ctx, p, tx)
}
