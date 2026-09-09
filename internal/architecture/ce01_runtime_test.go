package architecture_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
	"yunka.io/framework/operation"
	"yunka.io/gateway/authz"
	"yunka.io/pkg/operationplan"
)

// These test-only recorders observe the real Yunka executor/security chain.
// They do not emulate MySQL durability; qualification separately runs B12.5
// against a real MySQL 8.4 service. No commercial implementation is introduced.
type ce01Trace struct{ events []string }

func (trace *ce01Trace) add(event string) { trace.events = append(trace.events, event) }

type ce01Checker struct {
	trace *ce01Trace
	deny  bool
}

func (checker *ce01Checker) HasPermissions(_ context.Context, tenant string, roles []string, permissions []authz.PermissionKey, mode authz.PermissionMode) (bool, error) {
	checker.trace.add("authorize")
	return !checker.deny && tenant == "ce01-tenant" && len(roles) == 1 && len(permissions) == 2 && mode == authz.PermissionAll, nil
}

type ce01GuardKey struct{}
type ce01Guard struct {
	trace            *ce01Trace
	name             string
	deny, nilContext bool
}

var ce01InjectedFailure = errors.New("CE01: injected probe failure")

func (guard *ce01Guard) Prepare(ctx context.Context, authorized authz.AuthorizedOperation, _ any) (context.Context, error) {
	guard.trace.add(guard.name)
	if _, active := execution.Current(ctx); active {
		return nil, errors.New("CE01: guard ran after root scope creation")
	}
	if !authorized.Decision.Allowed || authorized.Policy.Operation != "device.transfer" {
		return nil, errors.New("CE01: guard did not receive the authorized root")
	}
	if guard.deny {
		return nil, ce01InjectedFailure
	}
	if guard.nilContext {
		return nil, nil
	}
	if guard.name == "entitlement" {
		return context.WithValue(ctx, ce01GuardKey{}, true), nil
	}
	if ctx.Value(ce01GuardKey{}) != true {
		return nil, errors.New("CE01: guard context was not propagated")
	}
	return ctx, nil
}

type ce01Factory struct{ trace *ce01Trace }

func (factory *ce01Factory) Begin(_ context.Context, mode execution.TransactionMode) (execution.UnitOfWork, error) {
	factory.trace.add("begin")
	if string(mode) != "local" {
		return nil, fmt.Errorf("CE01: transaction mode %s", mode)
	}
	return &ce01Unit{trace: factory.trace}, nil
}

type ce01Unit struct{ trace *ce01Trace }

func (unit *ce01Unit) Commit(context.Context) error   { unit.trace.add("commit"); return nil }
func (unit *ce01Unit) Rollback(context.Context) error { unit.trace.add("rollback"); return nil }
func (unit *ce01Unit) Close() error                   { unit.trace.add("close"); return nil }

func ce01RuntimePlans(t *testing.T) map[string]operationplan.Plan {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "contracts", "generated", "operation-plans.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inventory struct {
		Operations []operationplan.Plan `json:"operations"`
	}
	if err := json.Unmarshal(raw, &inventory); err != nil {
		t.Fatal(err)
	}
	result := make(map[string]operationplan.Plan, len(inventory.Operations))
	for _, plan := range inventory.Operations {
		result[plan.OperationID] = plan
	}
	for _, id := range []string{"device.transfer", "site.validate_transfer_target"} {
		if result[id].OperationID == "" {
			t.Fatalf("CE01: missing real generated plan %s", id)
		}
	}
	return result
}

func TestCE01RuntimeOrderAndFailureBoundaries(t *testing.T) {
	plans := ce01RuntimePlans(t)
	cases := []struct {
		name     string
		expected []string
	}{
		{"success", []string{"authorize", "entitlement", "scope", "begin", "root", "child", "commit", "close"}},
		{"child-fails", []string{"authorize", "entitlement", "scope", "begin", "root", "child", "rollback", "close"}},
		{"guard-denies", []string{"authorize", "entitlement"}},
		{"guard-nil-context", []string{"authorize", "entitlement"}},
		{"authorization-denies", []string{"authorize"}},
		{"undeclared-child", []string{"authorize", "entitlement", "scope", "begin", "root", "rollback", "close"}},
		{"nested-root", []string{"authorize", "entitlement", "scope", "begin", "root", "rollback", "close"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trace := &ce01Trace{}
			authorizer, err := authz.NewRBACAuthorizer(&ce01Checker{trace: trace, deny: tc.name == "authorization-denies"})
			if err != nil {
				t.Fatal(err)
			}
			first := &ce01Guard{trace: trace, name: "entitlement", deny: tc.name == "guard-denies", nilContext: tc.name == "guard-nil-context"}
			second := &ce01Guard{trace: trace, name: "scope"}
			security, err := authz.NewExecutionSecurity(authorizer, authz.NewStaticGuardChainResolver(map[authz.OperationID][]authz.OperationGuard{"device.transfer": {first, second}}))
			if err != nil {
				t.Fatal(err)
			}
			coordinator, err := execution.NewIdempotencyCoordinator(execution.NewMemoryIdempotencyStore())
			if err != nil {
				t.Fatal(err)
			}
			executor := operation.NewExecutorWithOptions(security, operation.ExecutorOptions{Transactions: &ce01Factory{trace: trace}, Idempotency: coordinator})
			ctx := identity.WithPrincipal(context.Background(), identity.Principal{Subject: "ce01-user", UserID: "ce01-user", TenantID: "ce01-tenant", Roles: []string{"operator"}, AuthMethod: "api-key", Authenticated: true})
			ctx = execution.WithIdempotencyKey(ctx, "ce01-"+tc.name)
			_, err = executor.Execute(ctx, plans["device.transfer"], nil, func(rootCtx context.Context) (any, error) {
				trace.add("root")
				frame, ok := execution.Current(rootCtx)
				if !ok || frame.Depth != 0 || frame.RootOperationID != "device.transfer" {
					return nil, errors.New("CE01: invalid root scope")
				}
				if tc.name == "nested-root" {
					_, nestedErr := executor.Execute(rootCtx, plans["device.transfer"], nil, func(context.Context) (any, error) { trace.add("unexpected-nested-root"); return nil, nil })
					if !errors.Is(nestedErr, execution.ErrScopeAlreadyActive) {
						return nil, fmt.Errorf("CE01: nested root result: %v", nestedErr)
					}
					return nil, nestedErr
				}
				child := plans["site.validate_transfer_target"]
				if tc.name == "undeclared-child" {
					child.OperationID = "ce01.undeclared"
				}
				return operation.ExecuteChild(rootCtx, executor, child, nil, func(childCtx context.Context) (any, error) {
					trace.add("child")
					joined, ok := execution.Current(childCtx)
					if !ok || joined.Depth != 1 || joined.RootOperationID != frame.RootOperationID || joined.OperationID != child.OperationID {
						return nil, errors.New("CE01: child did not join the root scope")
					}
					if childCtx.Value(ce01GuardKey{}) != true {
						return nil, errors.New("CE01: child lost the guard context")
					}
					if tc.name == "child-fails" {
						return nil, ce01InjectedFailure
					}
					return "observed", nil
				})
			})
			if tc.name == "success" && err != nil {
				t.Fatal(err)
			}
			if tc.name != "success" && err == nil {
				t.Fatal("CE01: negative probe was accepted")
			}
			if (tc.name == "child-fails" || tc.name == "guard-denies") && !errors.Is(err, ce01InjectedFailure) {
				t.Fatalf("CE01: injected cause lost: %v", err)
			}
			if !reflect.DeepEqual(trace.events, tc.expected) {
				t.Fatalf("CE01: got %v, want %v (error=%v)", trace.events, tc.expected, err)
			}
			t.Logf("CE01_TRACE=%v", trace.events)
		})
	}
}

func TestCE01ChildWithoutRootIsRejected(t *testing.T) {
	executor := operation.NewExecutor(nil)
	called := false
	_, err := operation.ExecuteChild(context.Background(), executor, ce01RuntimePlans(t)["site.validate_transfer_target"], nil, func(context.Context) (any, error) { called = true; return nil, nil })
	if !errors.Is(err, operation.ErrChildExecutionRequired) || called {
		t.Fatalf("CE01: child without root: called=%v err=%v", called, err)
	}
}
