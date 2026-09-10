//go:build integration

package persistence

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	p "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	"github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// This narrowly isolates the production repository's locking/time boundary.
// The separate integration API matrix exercises the complete production schema.
func TestCE10MySQLDeliveryLeaseExpiresDuringInnoDBLockWait(t *testing.T) {
	dsn := os.Getenv("YUNKA_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Fatal("real isolated MySQL is required; no skipped acceptance")
	}
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var nonce [8]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	name := "ce10_lock_" + hex.EncodeToString(nonce[:])
	if err = admin.Exec("CREATE DATABASE " + name).Error; err != nil {
		t.Fatal(err)
	}
	config.DBName = name
	db, err := gorm.Open(gormmysql.Open(config.FormatDSN()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
		if err := admin.Exec("DROP DATABASE " + name).Error; err != nil {
			t.Error(err)
		}
		sqlAdmin, _ := admin.DB()
		if sqlAdmin != nil {
			_ = sqlAdmin.Close()
		}
	})
	// No synthetic repository is used: the production outboxRow and Claim/locked
	// methods run against actual InnoDB. Only an unrelated FK fixture is omitted.
	if err = db.AutoMigrate(&outboxRow{}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var claim *p.Delivery
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, e := consistency.Now(tx)
		if e != nil {
			return e
		}
		r := &outboxRepository{tx: tx}
		event := p.Event{TenantID: "tenant-lock", AggregateID: "subscription-lock", AggregateVersion: 2, ChangeID: "change-lock", Status: p.Applied, SourceVersion: 2, EntitlementVersion: 2, OccurredAt: now}.Seal()
		if e = r.Append(ctx, event); e != nil {
			return e
		}
		claim, e = r.Claim(ctx, "worker-a", 5*time.Second)
		return e
	})
	if err != nil || claim == nil || claim.LeaseUntil == nil {
		t.Fatalf("claim=%v err=%v", claim, err)
	}
	blocker := db.WithContext(ctx).Begin()
	if blocker.Error != nil {
		t.Fatal(blocker.Error)
	}
	defer blocker.Rollback()
	var id string
	if err = blocker.Raw("SELECT event_id FROM biz_commercial_outbox WHERE event_id=? FOR UPDATE", claim.ID).Scan(&id).Error; err != nil {
		t.Fatal(err)
	}
	waiter := db.WithContext(ctx).Begin()
	if waiter.Error != nil {
		t.Fatal(waiter.Error)
	}
	defer waiter.Rollback()
	result := make(chan error, 1)
	go func() {
		_, _, e := (&outboxRepository{tx: waiter}).locked(ctx, claim.ID, "worker-a", claim.LeaseToken)
		result <- e
	}()
	observed := false
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		var n int64
		err = db.WithContext(ctx).Raw(`SELECT COUNT(*) FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks l ON l.ENGINE_LOCK_ID=w.REQUESTING_ENGINE_LOCK_ID AND l.ENGINE=w.ENGINE WHERE l.OBJECT_SCHEMA=DATABASE() AND l.OBJECT_NAME='biz_commercial_outbox'`).Scan(&n).Error
		if err != nil {
			t.Fatal(err)
		}
		if n > 0 {
			observed = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !observed {
		t.Fatal("test did not observe an actual InnoDB lock wait")
	}
	for {
		now, e := consistency.Now(db.WithContext(ctx))
		if e != nil {
			t.Fatal(e)
		}
		if !now.Before(*claim.LeaseUntil) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err = blocker.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-result:
		if !errors.Is(err, p.ErrLease) {
			t.Fatalf("expired lease accepted after observed row-lock wait: got=%v want=%v", err, p.ErrLease)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
