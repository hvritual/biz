//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	devicev1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
)

func ce06Await(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal("probe barrier not reached")
	}
}
func ce06Result(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("request did not finish")
		return nil
	}
}

// Pause the real generated endpoint in its actual root, not a mocked provider.
func ce06PauseQuery(t *testing.T, e *ce05Environment, table string, oldRead bool) (<-chan struct{}, func()) {
	t.Helper()
	entered, release := make(chan struct{}), make(chan struct{})
	var used, closed atomic.Bool
	name := "ce06:pause:" + ce04Random(t)
	err := e.db.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		f, ok := execution.Current(tx.Statement.Context)
		p, _ := identity.FromContext(tx.Statement.Context)
		if !ok || f.RootOperationID != "device.create" || p.TenantID != e.tenantA || tx.Statement.Table != table || !used.CompareAndSwap(false, true) {
			return
		}
		if oldRead {
			var n int64
			if err := tx.Session(&gorm.Session{NewDB: true}).Raw("SELECT COUNT(*) FROM biz_commercial_entitlement_sources WHERE tenant_id=?", e.tenantA).Scan(&n).Error; err != nil {
				tx.AddError(err)
			}
		}
		close(entered)
		select {
		case <-release:
		case <-time.After(10 * time.Second):
			tx.AddError(errors.New("CE06 query pause timed out"))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	unblock := func() {
		if closed.CompareAndSwap(false, true) {
			close(release)
		}
	}
	t.Cleanup(func() { unblock(); _ = e.db.Callback().Query().Remove(name) })
	return entered, unblock
}
func ce06PauseCreate(t *testing.T, e *ce05Environment) (<-chan struct{}, func()) {
	t.Helper()
	entered, release := make(chan struct{}), make(chan struct{})
	var used, closed atomic.Bool
	name := "ce06:create:" + ce04Random(t)
	err := e.db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		p, _ := identity.FromContext(tx.Statement.Context)
		if tx.Statement.Table != "biz_deviceops_device" || p.TenantID != e.tenantA || !used.CompareAndSwap(false, true) {
			return
		}
		close(entered)
		select {
		case <-release:
		case <-time.After(10 * time.Second):
			tx.AddError(errors.New("CE06 create pause timed out"))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	unblock := func() {
		if closed.CompareAndSwap(false, true) {
			close(release)
		}
	}
	t.Cleanup(func() { unblock(); _ = e.db.Callback().Create().Remove(name) })
	return entered, unblock
}
func ce06StartCreate(t *testing.T, e *ce05Environment, key string) <-chan error {
	t.Helper()
	serial := ce04Random(t)
	ch := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(ce04Context(e.tokenA, key), 10*time.Second)
		defer cancel()
		_, err := e.devices.CreateDevice(ctx, &devicev1.CreateDeviceRequest{SiteId: e.siteA, Name: "CE06 write", Serial: serial})
		ch <- err
	}()
	return ch
}
func ce06Count(t *testing.T, e *ce05Environment, want int64) {
	t.Helper()
	var n int64
	if err := e.db.Table("biz_deviceops_device").Where("tenant_id=?", e.tenantA).Count(&n).Error; err != nil || n != want {
		t.Fatalf("device side effect count=%d want=%d err=%v", n, want, err)
	}
}
func ce06ObserveWait(t *testing.T, db *gorm.DB, table string) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		var n int64
		err := db.Raw(`SELECT COUNT(*) FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks l ON l.ENGINE_LOCK_ID=w.REQUESTING_ENGINE_LOCK_ID AND l.ENGINE=w.ENGINE WHERE l.OBJECT_SCHEMA=DATABASE() AND l.OBJECT_NAME=?`, table).Scan(&n).Error
		if err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no actual InnoDB waiter on %s", table)
}
func TestCE06MySQLRevocationWinsAgainstOldRootSnapshot(t *testing.T) {
	e := ce05New(t)
	g := e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", 0)
	ce06My(t, e, e.tokenA)
	entered, release := ce06PauseQuery(t, e, "biz_commercial_entitlement_state", true)
	write := ce06StartCreate(t, e, ce04Random(t))
	ce06Await(t, entered)
	e.revoke(g.Source.Id, ce04Random(t), 1)
	release()
	ce05RPCDenied(t, ce06Result(t, write), codes.PermissionDenied, "MODULE_NOT_ENTITLED")
	ce06Count(t, e, 0)
	view := ce06My(t, e, e.tokenA)
	if view.SourceVersion != 2 {
		t.Fatal(view)
	}
}
func TestCE06MySQLWriteWinsThenRevocationBlocksNewRequests(t *testing.T) {
	e := ce05New(t)
	g := e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", 0)
	entered, release := ce06PauseCreate(t, e)
	write := ce06StartCreate(t, e, ce04Random(t))
	ce06Await(t, entered)
	revoke := make(chan error, 1)
	key := ce04Random(t)
	start := time.Now()
	go func() {
		ctx, cancel := context.WithTimeout(ce04Context(e.token, key), 10*time.Second)
		defer cancel()
		_, err := e.client.RevokeEntitlementOverride(ctx, &commercialv1.RevokeEntitlementOverrideRequest{TenantId: e.tenantA, Id: g.Source.Id, ExpectedVersion: 1, RequestId: key, Reason: "CE06 ordering"})
		revoke <- err
	}()
	ce06ObserveWait(t, e.db, "biz_commercial_entitlement_state")
	release()
	if err := ce06Result(t, write); err != nil {
		t.Fatal(err)
	}
	if err := ce06Result(t, revoke); err != nil {
		t.Fatal(err)
	}
	t.Logf("CE06 tenant revocation observed lock wait; completion latency=%s", time.Since(start))
	ce06Count(t, e, 1)
	ce05RPCDenied(t, ce06Result(t, ce06StartCreate(t, e, ce04Random(t))), codes.PermissionDenied, "MODULE_NOT_ENTITLED")
	ce06Count(t, e, 1)
}
func ce06Disable(t *testing.T, e *ce05Environment) <-chan error {
	t.Helper()
	m, err := e.catalog.GetModule(ce04Context(e.token, ""), &commercialv1.GetModuleRequest{ModuleCode: "device-operations"})
	if err != nil {
		t.Fatal(err)
	}
	key := ce04Random(t)
	result := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(ce04Context(e.token, key), 10*time.Second)
		defer cancel()
		_, err := e.catalog.SetModuleTechnicalStatus(ctx, &commercialv1.SetModuleTechnicalStatusRequest{RequestId: key, ModuleCode: m.ModuleCode, Version: m.Version, TechnicalStatus: commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_DISABLED, Reason: "CE06 global ordering"})
		result <- err
	}()
	return result
}
func TestCE06MySQLGlobalDisableSerializesWithWrites(t *testing.T) {
	for _, writeFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("write_first=%v", writeFirst), func(t *testing.T) {
			e := ce05New(t)
			e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", 0)
			ce06My(t, e, e.tokenA)
			var entered <-chan struct{}
			var release func()
			if writeFirst {
				entered, release = ce06PauseCreate(t, e)
			} else {
				entered, release = ce06PauseQuery(t, e, "biz_commercial_catalog_state", true)
			}
			write := ce06StartCreate(t, e, ce04Random(t))
			ce06Await(t, entered)
			disable := ce06Disable(t, e)
			if writeFirst {
				ce06ObserveWait(t, e.db, "biz_commercial_catalog_state")
				release()
				if err := ce06Result(t, write); err != nil {
					t.Fatal(err)
				}
				if err := ce06Result(t, disable); err != nil {
					t.Fatal(err)
				}
				ce06Count(t, e, 1)
			} else {
				if err := ce06Result(t, disable); err != nil {
					t.Fatal(err)
				}
				release()
				ce05RPCDenied(t, ce06Result(t, write), codes.PermissionDenied, "TECHNICAL_UNAVAILABLE")
				ce06Count(t, e, 0)
			}
			_, err := e.devices.ListDevices(ce04Context(e.tokenA, ""), &devicev1.ListDevicesRequest{})
			ce05RPCDenied(t, err, codes.PermissionDenied, "TECHNICAL_UNAVAILABLE")
		})
	}
}
func TestCE06MySQLExpiryRecheckedAfterPreflight(t *testing.T) {
	e := ce05New(t)
	at, err := consistency.Now(e.db)
	if err != nil {
		t.Fatal(err)
	}
	end := at.Add(600 * time.Millisecond)
	req := ce04Request(e.tenantA, ce04Random(t), 0, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT)
	req.ExpiresAt = end.Format(time.RFC3339Nano)
	e.mustCreate(req)
	entered, release := ce06PauseQuery(t, e, "biz_commercial_entitlement_state", false)
	write := ce06StartCreate(t, e, ce04Random(t))
	ce06Await(t, entered)
	ce06WaitDB(t, e.db, end)
	release()
	ce05RPCDenied(t, ce06Result(t, write), codes.PermissionDenied, "MODULE_NOT_ENTITLED")
	ce06Count(t, e, 0)
}
func TestCE06MySQLDeadlockRetriesWholeRequest(t *testing.T) {
	e := ce05New(t)
	e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", 0)
	ce06My(t, e, e.tokenA)
	// Isolated real InnoDB lock probe, never a mocked business/source provider.
	table := "ce06_lock_probe_" + ce04Random(t)[:12]
	if err := e.db.Exec("CREATE TABLE " + table + " (id INT PRIMARY KEY,n INT NOT NULL) ENGINE=InnoDB").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.db.Exec("DROP TABLE IF EXISTS " + table).Error })
	values := make([]string, 128)
	for i := range values {
		values[i] = fmt.Sprintf("(%d,0)", i)
	}
	if err := e.db.Exec("INSERT INTO " + table + " VALUES " + strings.Join(values, ",")).Error; err != nil {
		t.Fatal(err)
	}
	heavy := e.db.Begin()
	if heavy.Error != nil {
		t.Fatal(heavy.Error)
	}
	defer heavy.Rollback()
	if err := heavy.Exec("UPDATE " + table + " SET n=n+1").Error; err != nil {
		t.Fatal(err)
	}
	var once atomic.Bool
	actual := make(chan error, 1)
	name := "ce06:deadlock"
	if err := e.db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		p, _ := identity.FromContext(tx.Statement.Context)
		if tx.Statement.Table != "biz_deviceops_device" || p.TenantID != e.tenantA || !once.CompareAndSwap(false, true) {
			return
		}
		err := tx.Session(&gorm.Session{NewDB: true}).Exec("UPDATE " + table + " SET n=n+1 WHERE id=0").Error
		actual <- err
		if err != nil {
			tx.AddError(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = e.db.Callback().Create().Remove(name) })
	key, serial := ce04Random(t), ce04Random(t)
	request := &devicev1.CreateDeviceRequest{SiteId: e.siteA, Name: "deadlock", Serial: serial}
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(ce04Context(e.tokenA, key), 10*time.Second)
		defer cancel()
		_, err := e.devices.CreateDevice(ctx, request)
		done <- err
	}()
	ce06ObserveWait(t, e.db, table)
	// Force a real cycle; the lighter business root must receive the SQL error.
	err := heavy.Exec("UPDATE biz_commercial_entitlement_state SET version=version WHERE tenant_id=?", e.tenantA).Error
	if err != nil {
		t.Fatal("unexpected heavy victim", err)
	}
	if err := heavy.Rollback().Error; err != nil {
		t.Fatal(err)
	}
	deadlock := ce06Result(t, actual)
	if !consistency.Transient(deadlock) {
		t.Fatal("no real transient SQL error", deadlock)
	}
	ce05RPCDenied(t, ce06Result(t, done), codes.Unavailable, "ENTITLEMENT_RETRY_REQUIRED")
	ce06Count(t, e, 0)
	// A new request/Executor/transaction with the same key, not a callback retry.
	v, err := e.devices.CreateDevice(ce04Context(e.tokenA, key), request)
	if err != nil || v == nil {
		t.Fatal("whole request retry", err)
	}
	ce06Count(t, e, 1)
}
