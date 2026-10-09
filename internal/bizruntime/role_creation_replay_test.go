package bizruntime

import (
	"context"
	"errors"
	"testing"

	accessports "github.com/hvritual/biz/internal/access/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"yunka.io/framework/execution"
	"yunka.io/pkg/operationplan"
)

func TestCE293RoleCreationReplayKeepsFencingAndRequiresExistingReceipt(t *testing.T) {
	base, err := execution.NewIdempotencyCoordinator(execution.NewMemoryIdempotencyStore())
	if err != nil {
		t.Fatal(err)
	}
	coordinator := tenantCreationReplay{base}
	plan := operationplan.Plan{OperationID: "tenant.role.create"}
	if _, err := coordinator.Begin(context.Background(), plan); !errors.Is(err, execution.ErrIdempotencyKeyRequired) {
		t.Fatal("missing key accepted", err)
	}
	ctx := execution.WithIdempotencyKey(context.Background(), "role-key")
	first, err := coordinator.Begin(ctx, plan)
	if err != nil || accessports.IsRoleCreationReplay(first) {
		t.Fatal("new claim marked replay", err)
	}
	if _, err := coordinator.Begin(ctx, plan); !errors.Is(err, execution.ErrIdempotencyInProgress) {
		t.Fatal("running claim bypassed", err)
	}
	if err := coordinator.Complete(first, plan); err != nil {
		t.Fatal(err)
	}
	replayed, err := coordinator.Begin(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !accessports.IsRoleCreationReplay(replayed) || execution.IdempotencyKeyFrom(replayed) != "role-key" {
		t.Fatal("existing receipt requirement or original key lost")
	}
	if err := coordinator.Complete(replayed, plan); err != nil {
		t.Fatal("replay has no real fenced claim", err)
	}
	for _, id := range []string{"tenant.role.update", "tenant.role.delete", "tenant.role.set_permissions", "tenant.role.assign_member", "tenant.role.revoke_member"} {
		if durableReceiptOperation(id) {
			t.Fatalf("unrelated mutation %s became replayable", id)
		}
	}
}

func TestCE293ForeignRoleNotFoundKeepsTypedCauseAndTransportCode(t *testing.T) {
	for _, operation := range []string{"tenant.role.get", "tenant.role.update"} {
		err := roleExecutionError(context.Background(), operation, accessports.ErrTenantRoleNotFound)
		if status.Code(err) != codes.NotFound || !errors.Is(err, accessports.ErrTenantRoleNotFound) {
			t.Fatalf("foreign object error lost transport semantics or original cause: %v", err)
		}
	}
	if roleExecutionError(context.Background(), "tenant.role.get", nil) != nil {
		t.Fatal("successful read changed")
	}
}
