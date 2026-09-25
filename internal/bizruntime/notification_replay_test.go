package bizruntime

import (
	"context"
	"errors"
	"testing"
	"yunka.io/framework/execution"
	"yunka.io/pkg/operationplan"
)

func TestNotificationReplayUsesOriginalKeyAndFencedClaims(t *testing.T) {
	for _, operation := range []string{"notification.configuration.create", "notification.configuration.update", "notification.configuration.delete"} {
		t.Run(operation, func(t *testing.T) {
			base, err := execution.NewIdempotencyCoordinator(execution.NewMemoryIdempotencyStore())
			if err != nil {
				t.Fatal(err)
			}
			c := tenantCreationReplay{base}
			plan := operationplan.Plan{OperationID: operation}
			if _, err := c.Begin(context.Background(), plan); !errors.Is(err, execution.ErrIdempotencyKeyRequired) {
				t.Fatalf("missing key: %v", err)
			}
			ctx := execution.WithIdempotencyKey(context.Background(), "notification-original-key")
			first, err := c.Begin(ctx, plan)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := c.Begin(ctx, plan); !errors.Is(err, execution.ErrIdempotencyInProgress) {
				t.Fatalf("running claim bypassed: %v", err)
			}
			if err := c.Complete(first, plan); err != nil {
				t.Fatal(err)
			}
			replay, err := c.Begin(ctx, plan)
			if err != nil {
				t.Fatal(err)
			}
			if execution.IdempotencyKeyFrom(replay) != "notification-original-key" {
				t.Fatal("original key lost")
			}
			if err := c.Complete(replay, plan); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, other := range []string{"notification.configuration.unknown", "notification.channel.list", "device.create"} {
		if durableReceiptOperation(other) {
			t.Fatalf("unexpected durable replay allowlist entry: %s", other)
		}
	}
}
