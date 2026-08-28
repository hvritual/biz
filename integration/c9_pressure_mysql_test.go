//go:build integration

package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	deviceapp "github.com/hvritual/biz/internal/deviceops/application"
	"github.com/hvritual/biz/internal/deviceops/domain"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	devicepolicy "github.com/hvritual/biz/internal/deviceops/policy"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/event"
	"yunka.io/framework/event/outbox"
	"yunka.io/framework/operation"
	"yunka.io/gateway/authz"
)

type pressureObserver struct {
	mu             sync.Mutex
	securityStarts map[string]int
}

func newPressureObserver() *pressureObserver {
	return &pressureObserver{securityStarts: map[string]int{}}
}

func (observer *pressureObserver) Observe(_ context.Context, value operation.Event) {
	if value.Phase != operation.PhaseSecurity || value.Outcome != operation.OutcomeStarted {
		return
	}
	observer.mu.Lock()
	observer.securityStarts[value.OperationID]++
	observer.mu.Unlock()
}

func (observer *pressureObserver) starts(operationID string) int {
	observer.mu.Lock()
	defer observer.mu.Unlock()
	return observer.securityStarts[operationID]
}

func pressureExecutor(t *testing.T, accessStore *accesspersistence.Store, observer operation.Observer, operationIDs ...string) operation.Executor {
	t.Helper()
	authorizer, err := authz.NewGrantAuthorizer(accessStore)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := devicesecurity.NewGuard(accessStore)
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[authz.OperationID]authz.OperationGuard, len(operationIDs))
	for _, operationID := range operationIDs {
		values[authz.OperationID(operationID)] = guard
	}
	security, err := authz.NewExecutionSecurity(authorizer, authz.NewStaticGuardResolver(values))
	if err != nil {
		t.Fatal(err)
	}
	return operation.NewExecutor(security, observer)
}

func authenticatedContext(t *testing.T, store *accesspersistence.Store, token string) context.Context {
	t.Helper()
	principal, err := store.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	return identity.WithPrincipal(context.Background(), principal)
}

func TestC9LocalCompositionUsesOneExecutorAndOneUoW(t *testing.T) {
	db := openDB(t)
	tenant, ownerToken, sourceSite, targetSite := "tenant-c9-local", "owner-c9-local-token", "site-c9-source", "site-c9-target"
	module := startModule(t, db, tenant, ownerToken, sourceSite)
	created := postDevice(t, "http://"+module.HTTPAddress(), ownerToken, sourceSite, "Local", "SN-C9-LOCAL")

	accessStore, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	ownerContext := authenticatedContext(t, accessStore, ownerToken)
	sites, err := devicepersistence.NewSiteRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := sites.Create(ownerContext, &domain.Site{ID: targetSite, Name: "Target Site"}); err != nil {
		t.Fatal(err)
	}

	scopes, err := devicepersistence.NewLocalTransferScopeFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	service, err := deviceapp.NewLocalTransferService(scopes)
	if err != nil {
		t.Fatal(err)
	}
	observer := newPressureObserver()
	executor := pressureExecutor(t, accessStore, observer, devicepolicy.OperationLocalTransfer)
	id, _ := created["id"].(string)
	version, _ := created["version"].(float64)
	response, err := operation.ExecuteTyped(ownerContext, executor, devicepolicy.LocalTransferPressurePlan(), &deviceopsv1.UpdateDeviceRequest{Id: id, SiteId: targetSite, Version: uint64(version)}, service.Transfer)
	if err != nil {
		t.Fatal(err)
	}
	if response.GetSiteId() != targetSite {
		t.Fatalf("local transfer site=%q want=%q", response.GetSiteId(), targetSite)
	}
	if observer.starts(devicepolicy.OperationLocalTransfer) != 1 {
		t.Fatalf("local transfer security starts=%d want=1", observer.starts(devicepolicy.OperationLocalTransfer))
	}

	readerToken := "reader-c9-local-token"
	seedReader(t, db, tenant, "reader-c9-local", readerToken, "", tenant+":reader-c9-local", "reader", "device.read", "all")
	readerContext := authenticatedContext(t, accessStore, readerToken)
	_, err = operation.ExecuteTyped(readerContext, executor, devicepolicy.LocalTransferPressurePlan(), &deviceopsv1.UpdateDeviceRequest{Id: id, SiteId: sourceSite, Version: response.GetVersion()}, service.Transfer)
	if !authz.IsDenied(err) {
		t.Fatalf("local composite permission closure must deny missing device.update: %v", err)
	}
}

type failingTransactionalOutbox struct{ err error }

func (store failingTransactionalOutbox) EnqueueTx(context.Context, any, event.Envelope) error {
	return store.err
}

func TestC9RemoteSagaStagesBusinessWriteAndOutboxAtomically(t *testing.T) {
	db := openDB(t)
	tenant, ownerToken, site := "tenant-c9-saga", "owner-c9-saga-token", "site-c9-saga"
	_ = startModule(t, db, tenant, ownerToken, site)
	accessStore, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	ownerContext := authenticatedContext(t, accessStore, ownerToken)
	scopes, err := devicepersistence.NewScopedScopeFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("DROP TABLE IF EXISTS biz_pressure_outbox_remote").Error; err != nil {
		t.Fatal(err)
	}
	store, err := outbox.NewGORMStore(db, outbox.WithTable("biz_pressure_outbox_remote"), outbox.WithSkipLocked(true))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(ownerContext); err != nil {
		t.Fatal(err)
	}
	service, err := deviceapp.NewProvisioningService(scopes, store)
	if err != nil {
		t.Fatal(err)
	}
	observer := newPressureObserver()
	executor := pressureExecutor(t, accessStore, observer, devicepolicy.OperationRemoteProvision)
	serial := "SN-C9-SAGA-SUCCESS"
	response, err := operation.ExecuteTyped(ownerContext, executor, devicepolicy.RemoteProvisionPressurePlan(), &deviceopsv1.CreateDeviceRequest{SiteId: site, Name: "Saga", Serial: serial}, service.Provision)
	if err != nil {
		t.Fatal(err)
	}
	if response.GetSerial() != serial {
		t.Fatalf("provisioned serial=%q want=%q", response.GetSerial(), serial)
	}
	if observer.starts(devicepolicy.OperationRemoteProvision) != 1 {
		t.Fatalf("remote saga security starts=%d want=1", observer.starts(devicepolicy.OperationRemoteProvision))
	}
	snapshot, err := store.Snapshot(ownerContext)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Pending != 2 {
		t.Fatalf("pending saga commands=%d want=2", snapshot.Pending)
	}
	records, err := store.Claim(ownerContext, outbox.ClaimOptions{Owner: "c9-pressure", Limit: 10, Lease: time.Minute, Now: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("claimed saga commands=%d want=2", len(records))
	}
	types := map[string]bool{}
	for _, record := range records {
		types[record.Envelope.Type] = true
		if record.Envelope.Metadata["idempotency.key"] != "device-provision:"+serial {
			t.Fatalf("unexpected saga idempotency metadata: %#v", record.Envelope.Metadata)
		}
	}
	if !types["inventory.reserve"] || !types["device.activate"] {
		t.Fatalf("unexpected saga commands: %#v", types)
	}

	failure := errors.New("forced outbox failure")
	failingService, err := deviceapp.NewProvisioningService(scopes, failingTransactionalOutbox{err: failure})
	if err != nil {
		t.Fatal(err)
	}
	failedSerial := "SN-C9-SAGA-ROLLBACK"
	_, err = operation.ExecuteTyped(ownerContext, executor, devicepolicy.RemoteProvisionPressurePlan(), &deviceopsv1.CreateDeviceRequest{SiteId: site, Name: "Rollback", Serial: failedSerial}, failingService.Provision)
	if !errors.Is(err, failure) {
		t.Fatalf("expected forced outbox failure, got %v", err)
	}
	var count int64
	if err := db.Table("biz_deviceops_device").Where("tenant_id = ? AND serial = ?", tenant, failedSerial).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("business row survived outbox failure: count=%d", count)
	}
}
