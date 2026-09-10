package bizruntime

import (
	"context"
	"errors"
	"testing"
	"yunka.io/framework/execution"
	"yunka.io/pkg/operationplan"
)

func TestCE08ReplayCoordinatorPreservesRequiredClaimAndOtherOperations(t *testing.T) {
	base, err := execution.NewIdempotencyCoordinator(execution.NewMemoryIdempotencyStore())
	if err != nil {
		t.Fatal(err)
	}
	c := tenantCreationReplay{base}
	plan := operationplan.Plan{OperationID: "tenant.create"}
	if _, err := c.Begin(context.Background(), plan); !errors.Is(err, execution.ErrIdempotencyKeyRequired) {
		t.Fatal(err)
	}
	ctx := execution.WithIdempotencyKey(context.Background(), "key")
	first, err := c.Begin(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Begin(ctx, plan); !errors.Is(err, execution.ErrIdempotencyInProgress) {
		t.Fatal("running claim bypassed", err)
	}
	if err := c.Complete(first, plan); err != nil {
		t.Fatal(err)
	}
	replay, err := c.Begin(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	if execution.IdempotencyKeyFrom(replay) != "key" {
		t.Fatal("original transport key lost")
	}
	if err := c.Complete(replay, plan); err != nil {
		t.Fatal("replay must own a real fenced claim", err)
	}
	other := operationplan.Plan{OperationID: "tenant.activate"}
	one, err := c.Begin(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Complete(one, other); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Begin(ctx, other); !errors.Is(err, execution.ErrIdempotencyCompleted) {
		t.Fatal("other operation replay policy changed", err)
	}
	if c.SupportsAtomicCompletion() {
		t.Fatal("memory test store claimed atomic DB completion")
	}
}
