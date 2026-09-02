//go:build integration

package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	deviceapp "github.com/hvritual/biz/internal/deviceops/application"
	"github.com/hvritual/biz/internal/deviceops/domain"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	devicepolicy "github.com/hvritual/biz/internal/deviceops/policy"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	"gorm.io/gorm"
	"yunka.io/framework/execution"
	"yunka.io/framework/execution/idempotencygorm"
	"yunka.io/framework/operation"
	"yunka.io/framework/requestscope"
	"yunka.io/gateway/authz"
)

type c98Observer struct {
	mu           sync.Mutex
	rootSecurity int
	children     map[string]int
}

func newC98Observer() *c98Observer {
	return &c98Observer{children: map[string]int{}}
}

func (observer *c98Observer) Observe(_ context.Context, event operation.Event) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if event.Kind == operation.InvocationRoot && event.Phase == operation.PhaseSecurity && event.Outcome == operation.OutcomeStarted {
		observer.rootSecurity++
	}
	if event.Kind == operation.InvocationChild && event.Phase == operation.PhaseApplication && event.Outcome == operation.OutcomeStarted {
		observer.children[event.OperationID]++
	}
}

func (observer *c98Observer) snapshot() (int, map[string]int) {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	children := make(map[string]int, len(observer.children))
	for key, value := range observer.children {
		children[key] = value
	}
	return observer.rootSecurity, children
}

type countingTransactionFactory struct {
	inner    execution.TransactionFactory
	mu       sync.Mutex
	begin    int
	commit   int
	rollback int
}

func (factory *countingTransactionFactory) Begin(ctx context.Context, mode execution.TransactionMode) (execution.UnitOfWork, error) {
	unit, err := factory.inner.Begin(ctx, mode)
	if err != nil {
		return nil, err
	}
	factory.mu.Lock()
	factory.begin++
	factory.mu.Unlock()
	return &countingUnit{inner: unit, owner: factory}, nil
}

func (factory *countingTransactionFactory) snapshot() (int, int, int) {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	return factory.begin, factory.commit, factory.rollback
}

type countingUnit struct {
	inner execution.UnitOfWork
	owner *countingTransactionFactory
}

func (unit *countingUnit) Commit(ctx context.Context) error {
	err := unit.inner.Commit(ctx)
	if err == nil {
		unit.owner.mu.Lock()
		unit.owner.commit++
		unit.owner.mu.Unlock()
	}
	return err
}

func (unit *countingUnit) Rollback(ctx context.Context) error {
	err := unit.inner.Rollback(ctx)
	unit.owner.mu.Lock()
	unit.owner.rollback++
	unit.owner.mu.Unlock()
	return err
}

func (unit *countingUnit) Close() error { return unit.inner.Close() }

// GORM preserves the repository capability of the wrapped root UoW. The
// wrapper exists only to count lifecycle calls and must otherwise be transparent.
func (unit *countingUnit) GORM() *gorm.DB {
	provider, ok := unit.inner.(requestscope.GORMUnitOfWork)
	if !ok || provider == nil {
		return nil
	}
	return provider.GORM()
}

func (unit *countingUnit) TransactionHandle() any {
	provider, ok := unit.inner.(execution.TransactionHandleProvider)
	if !ok || provider == nil {
		return nil
	}
	return provider.TransactionHandle()
}

func c98Executor(t *testing.T, database *gorm.DB, accessStore *accesspersistence.Store, observer operation.Observer) (operation.Executor, *countingTransactionFactory) {
	t.Helper()
	authorizer, err := authz.NewGrantAuthorizer(accessStore)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := devicesecurity.NewGuard(accessStore)
	if err != nil {
		t.Fatal(err)
	}
	security, err := authz.NewExecutionSecurity(authorizer, authz.NewStaticGuardResolver(map[authz.OperationID]authz.OperationGuard{
		authz.OperationID("device.transfer"): guard,
	}))
	if err != nil {
		t.Fatal(err)
	}
	gormTransactions, err := requestscope.NewGORMExecutionFactory(database)
	if err != nil {
		t.Fatal(err)
	}
	transactions := &countingTransactionFactory{inner: gormTransactions}
	store, err := idempotencygorm.NewStore(database, idempotencygorm.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	idempotency, err := execution.NewIdempotencyCoordinator(store)
	if err != nil {
		t.Fatal(err)
	}
	return operation.NewExecutorWithOptions(security, operation.ExecutorOptions{Transactions: transactions, Idempotency: idempotency}, observer), transactions
}

func TestC98RealCrossApplicationTransferSharesOneExecutionScope(t *testing.T) {
	db := openDB(t)
	tenant, ownerToken := "tenant-c98-cross-app", "owner-c98-cross-app-token"
	sourceSite, targetSite := "site-c98-source", "site-c98-target"
	module := startModule(t, db, tenant, ownerToken, sourceSite)
	created := postDevice(t, "http://"+module.HTTPAddress(), ownerToken, sourceSite, "Cross App", "SN-C98-CROSS-APP")

	accessStore, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	ownerContext := authenticatedContext(t, accessStore, ownerToken)
	sites, err := devicepersistence.NewSiteRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := sites.Create(ownerContext, &domain.Site{ID: targetSite, Name: "C9.8 Target"}); err != nil {
		t.Fatal(err)
	}

	repositories, err := devicepersistence.NewScopedRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	deviceService, err := deviceapp.NewService(repositories)
	if err != nil {
		t.Fatal(err)
	}
	siteService, err := deviceapp.NewSiteManagementService(repositories)
	if err != nil {
		t.Fatal(err)
	}
	observer := newC98Observer()
	executor, transactions := c98Executor(t, db, accessStore, observer)
	siteCapability, err := deviceapp.NewDeviceTransferToDeviceopsSiteManagementChildCapability(siteService, executor)
	if err != nil {
		t.Fatal(err)
	}
	deviceCapability, err := deviceapp.NewDeviceTransferToDeviceopsDeviceManagementChildCapability(deviceService, executor)
	if err != nil {
		t.Fatal(err)
	}
	transferService, err := deviceapp.NewCrossApplicationTransferService(siteCapability, deviceCapability)
	if err != nil {
		t.Fatal(err)
	}

	id, _ := created["id"].(string)
	version := jsonUint64(t, created["version"])
	ctx := execution.WithIdempotencyKey(ownerContext, "c98-transfer:"+id+":"+targetSite)
	response, err := operation.ExecuteTyped(ctx, executor, devicepolicy.OperationPlanDeviceTransferTransferDevice(), &deviceopsv1.TransferDeviceRequest{
		Id: id, TargetSiteId: targetSite, Version: version,
	}, transferService.TransferDevice)
	if err != nil {
		t.Fatal(err)
	}
	if response.GetSiteId() != targetSite {
		t.Fatalf("transferred site=%q want=%q", response.GetSiteId(), targetSite)
	}

	rootSecurity, children := observer.snapshot()
	if rootSecurity != 1 {
		t.Fatalf("root security decisions=%d want=1", rootSecurity)
	}
	if children["site.validate_transfer_target"] != 1 || children["device.update"] != 1 {
		t.Fatalf("child invocations=%v want site.validate_transfer_target=1 device.update=1", children)
	}
	begins, commits, rollbacks := transactions.snapshot()
	if begins != 1 || commits != 1 || rollbacks != 0 {
		t.Fatalf("transaction lifecycle begin=%d commit=%d rollback=%d want 1/1/0", begins, commits, rollbacks)
	}

	// The durable root idempotency record must stop the whole composite before
	// another transaction or child invocation starts.
	_, err = operation.ExecuteTyped(ctx, executor, devicepolicy.OperationPlanDeviceTransferTransferDevice(), &deviceopsv1.TransferDeviceRequest{
		Id: id, TargetSiteId: targetSite, Version: response.GetVersion(),
	}, transferService.TransferDevice)
	if !errors.Is(err, execution.ErrIdempotencyCompleted) {
		t.Fatalf("duplicate transfer err=%v want durable idempotency completion", err)
	}
	rootSecurity, children = observer.snapshot()
	begins, commits, rollbacks = transactions.snapshot()
	if rootSecurity != 2 || begins != 1 || commits != 1 || rollbacks != 0 || children["site.validate_transfer_target"] != 1 || children["device.update"] != 1 {
		t.Fatalf("duplicate escaped root gate: security=%d begin=%d commit=%d rollback=%d children=%v", rootSecurity, begins, commits, rollbacks, children)
	}

	// Missing one permission from the compiled child permission closure must be
	// denied before the root transaction and before either child Application.
	limitedToken := "limited-c98-cross-app-token"
	seedReader(t, db, tenant, "limited-c98-cross-app", limitedToken, "", tenant+":limited-c98", "limited-c98", "device.update", "all")
	limitedContext := authenticatedContext(t, accessStore, limitedToken)
	limitedContext = execution.WithIdempotencyKey(limitedContext, "c98-transfer-denied:"+id)
	_, err = operation.ExecuteTyped(limitedContext, executor, devicepolicy.OperationPlanDeviceTransferTransferDevice(), &deviceopsv1.TransferDeviceRequest{
		Id: id, TargetSiteId: sourceSite, Version: response.GetVersion(),
	}, transferService.TransferDevice)
	if !authz.IsDenied(err) {
		t.Fatalf("missing site.read permission must deny root composite: %v", err)
	}
	_, children = observer.snapshot()
	begins, commits, rollbacks = transactions.snapshot()
	if begins != 1 || commits != 1 || rollbacks != 0 || children["site.validate_transfer_target"] != 1 || children["device.update"] != 1 {
		t.Fatalf("denied composite crossed execution boundary: begin=%d commit=%d rollback=%d children=%v", begins, commits, rollbacks, children)
	}
}
