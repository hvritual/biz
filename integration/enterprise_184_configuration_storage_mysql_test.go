//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	ad "github.com/hvritual/biz/internal/access/domain"
	ap "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	nd "github.com/hvritual/biz/internal/notification/domain"
	np "github.com/hvritual/biz/internal/notification/infrastructure/persistence"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

// Storage and current Access-directory evidence only: this test intentionally
// does not claim HTTP, generated-Action or browser completion. It reuses the
// canonical serial MySQL lane and creates only namespaced fixture records.
func TestEnterprise184ConfigurationStorageAndAccessDirectory(t *testing.T) {
	db := openDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	store, err := ap.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = np.MigrateConfigurations(ctx, db); err != nil {
		t.Fatal(err)
	}
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenantA, tenantB, user := "n184-a-"+stamp, "n184-b-"+stamp, "n184-u-"+stamp
	for _, tenant := range []string{tenantA, tenantB} {
		if err = store.Bootstrap(ctx, ap.Bootstrap{TenantID: tenant, TenantName: tenant, UserID: user, Email: "n184-" + stamp + "@example.invalid", Token: "n184-token-" + tenant}, []authz.PermissionKey{"tenant.notification.create"}); err != nil {
			t.Fatal(err)
		}
	}
	principalA, err := store.Authenticate(ctx, "n184-token-"+tenantA)
	if err != nil {
		t.Fatal(err)
	}
	principalB, err := store.Authenticate(ctx, "n184-token-"+tenantB)
	if err != nil {
		t.Fatal(err)
	}
	ctxA, ctxB := identity.WithPrincipal(ctx, principalA), identity.WithPrincipal(ctx, principalB)
	transact := func(c context.Context, fn func(*np.ConfigurationRepository, *gorm.DB) error) error {
		return db.WithContext(c).Transaction(func(tx *gorm.DB) error {
			r, e := np.NewConfigurationRepository(tx)
			if e != nil {
				return e
			}
			return fn(r, tx)
		})
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	config := func(id, tenant, group string, level nd.MessageLevel) nd.Configuration {
		return nd.Configuration{ID: id + "-" + stamp, TenantID: tenant, GroupID: group + "-" + stamp, Level: level, Channels: []string{"in_app"}, PrimaryUserID: user, AdditionalUserIDs: []string{}, Version: 1, CreatedAt: now, UpdatedAt: now}
	}
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	get := func(c context.Context, tenant, id string) nd.Configuration {
		t.Helper()
		var out nd.Configuration
		must(transact(c, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			var e error
			out, e = r.Get(c, tenant, id, false)
			return e
		}))
		return out
	}
	count := func(table string, tenant string) int64 {
		t.Helper()
		var n int64
		must(db.Table(table).Where("tenant_id = ?", tenant).Count(&n).Error)
		return n
	}
	initial := []nd.Configuration{config("first", tenantA, "site", nd.LevelUrgent), config("second", tenantA, "site", nd.LevelGeneral)}

	t.Run("requires-root-transaction", func(t *testing.T) {
		if _, e := np.NewConfigurationRepository(db); e == nil {
			t.Fatal("autocommit accepted")
		}
		d, e := ap.NewNotificationRecipientDirectory(db)
		must(e)
		if e = d.LockCaller(ctxA, tenantA, true); e == nil {
			t.Fatal("autocommit eligibility locks accepted")
		}
		if e = d.Validate(ctxA, tenantA, []string{user}, true); e == nil {
			t.Fatal("autocommit recipient locks accepted")
		}
	})
	t.Run("multi-level-create-and-authoritative-read", func(t *testing.T) {
		must(transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error { return r.CreateMany(ctxA, initial) }))
		for _, c := range initial {
			got := get(ctxA, tenantA, c.ID)
			if got.GroupID != c.GroupID || got.Level != c.Level || got.Version != 1 || !reflect.DeepEqual(got.Channels, c.Channels) {
				t.Fatalf("readback %+v", got)
			}
		}
		if n := count("biz_notification_configurations", tenantA); n != 2 {
			t.Fatalf("count %d", n)
		}
	})
	t.Run("duplicate-multi-level-batch-rolls-back-every-row", func(t *testing.T) {
		batch := []nd.Configuration{config("new-important", tenantA, "site", nd.LevelImportant), config("duplicate", tenantA, "site", nd.LevelUrgent)}
		e := transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error { return r.CreateMany(ctxA, batch) })
		if !errors.Is(e, nd.ErrConfigurationDuplicate) {
			t.Fatalf("expected duplicate: %v", e)
		}
		if n := count("biz_notification_configurations", tenantA); n != 2 {
			t.Fatalf("partial batch survived: %d", n)
		}
	})
	t.Run("tenant-isolation-and-canonical-identifiers", func(t *testing.T) {
		must(transact(ctxB, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			return r.CreateMany(ctxB, []nd.Configuration{config("tenant-b", tenantB, "site", nd.LevelUrgent)})
		}))
		for _, c := range []context.Context{ctxB, ctx} {
			e := transact(c, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
				_, e := r.Get(c, tenantA, initial[0].ID, true)
				return e
			})
			if !errors.Is(e, nd.ErrConfigurationNotFound) {
				t.Fatalf("cross identity read: %v", e)
			}
		}
		e := transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			_, e := r.Get(ctxA, tenantA, strings.ToUpper(initial[0].ID), false)
			return e
		})
		if !errors.Is(e, nd.ErrConfigurationNotFound) {
			t.Fatalf("alias read: %v", e)
		}
	})
	t.Run("same-filter-total-paging-and-empty-scope", func(t *testing.T) {
		must(transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			for page := 1; page <= 3; page++ {
				items, total, e := r.List(ctxA, tenantA, nd.ConfigurationFilter{GroupIDs: []string{initial[0].GroupID}, RecipientID: user, Page: page, PageSize: 1})
				if e != nil {
					return e
				}
				want := 1
				if page == 3 {
					want = 0
				}
				if len(items) != want || total != 2 {
					t.Fatalf("page=%d items=%d total=%d", page, len(items), total)
				}
			}
			items, total, e := r.List(ctxA, tenantA, nd.ConfigurationFilter{Page: 1, PageSize: 10})
			if e != nil {
				return e
			}
			if total != 0 || len(items) != 0 {
				t.Fatal("empty scope expanded")
			}
			return nil
		}))
	})
	t.Run("replace-recipients-and-stale-version", func(t *testing.T) {
		before := get(ctxA, tenantA, initial[0].ID)
		next := before
		next.Version++
		next.AdditionalUserIDs = []string{"receiver-a", "receiver-b"}
		next.UpdatedAt = now.Add(time.Second)
		must(transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error { return r.Replace(ctxA, next, 1) }))
		next.Version++
		next.AdditionalUserIDs = []string{"receiver-c"}
		next.UpdatedAt = now.Add(2 * time.Second)
		must(transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error { return r.Replace(ctxA, next, 2) }))
		got := get(ctxA, tenantA, next.ID)
		if !reflect.DeepEqual(got.AdditionalUserIDs, []string{"receiver-c"}) || got.Version != 3 {
			t.Fatalf("replacement %+v", got)
		}
		stale := before
		stale.Version = 2
		e := transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error { return r.Replace(ctxA, stale, 1) })
		if !errors.Is(e, nd.ErrConfigurationConflict) {
			t.Fatalf("stale accepted: %v", e)
		}
	})
	t.Run("stable-durable-receipt-does-not-overwrite-later-state", func(t *testing.T) {
		c := config("receipt", tenantA, "receipt-site", nd.LevelGeneral)
		digest, _ := nd.DigestConfigurationRequest(c)
		want := nd.ConfigurationReceipt{ReceiptID: "receipt-" + stamp, TenantID: tenantA, Configurations: []nd.Configuration{c}}
		must(transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			old, e := r.ClaimReceipt(ctxA, tenantA, user, "create", "original-key", digest)
			if e != nil {
				return e
			}
			if old != nil {
				t.Fatal("unexpected replay")
			}
			if e = r.CreateMany(ctxA, []nd.Configuration{c}); e != nil {
				return e
			}
			return r.CompleteReceipt(ctxA, tenantA, user, "create", "original-key", want)
		}))
		next := get(ctxA, tenantA, c.ID)
		next.Version = 2
		next.Notes = "newer"
		next.UpdatedAt = now.Add(time.Second)
		must(transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error { return r.Replace(ctxA, next, 1) }))
		must(transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			got, e := r.ClaimReceipt(ctxA, tenantA, user, "create", "original-key", digest)
			if e != nil {
				return e
			}
			if got == nil || !reflect.DeepEqual(*got, want) {
				t.Fatalf("unstable replay %+v", got)
			}
			return nil
		}))
		if get(ctxA, tenantA, c.ID).Version != 2 {
			t.Fatal("replay rewrote current record")
		}
		e := transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			_, e := r.ClaimReceipt(ctxA, tenantA, user, "create", "original-key", strings.Repeat("a", 64))
			return e
		})
		if !errors.Is(e, nd.ErrConfigurationReplayConflict) {
			t.Fatalf("changed payload accepted %v", e)
		}
	})
	t.Run("receipt-failure-rolls-back-config-and-relations", func(t *testing.T) {
		c := config("rollback", tenantA, "rollback-site", nd.LevelGeneral)
		c.AdditionalUserIDs = []string{"extra"}
		injected := errors.New("injected receipt failure")
		callback := "notification184:receipt"
		must(db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
			if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "biz_notification_configuration_receipts" {
				tx.AddError(injected)
			}
		}))
		e := transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			if e := r.CreateMany(ctxA, []nd.Configuration{c}); e != nil {
				return e
			}
			_, e := r.ClaimReceipt(ctxA, tenantA, user, "create", "rollback-key", strings.Repeat("a", 64))
			return e
		})
		must(db.Callback().Create().Remove(callback))
		if !errors.Is(e, injected) {
			t.Fatalf("injection missed: %v", e)
		}
		e = transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			_, e := r.Get(ctxA, tenantA, c.ID, false)
			return e
		})
		if !errors.Is(e, nd.ErrConfigurationNotFound) {
			t.Fatalf("partial state survived %v", e)
		}
	})
	t.Run("audit-failure-rolls-back-root-transaction", func(t *testing.T) {
		c := config("audit-rollback", tenantA, "audit-site", nd.LevelGeneral)
		injected := errors.New("injected audit failure")
		callback := "notification184:audit"
		must(db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
			if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "biz_audit_events" {
				tx.AddError(injected)
			}
		}))
		e := transact(ctxA, func(r *np.ConfigurationRepository, tx *gorm.DB) error {
			if e := r.CreateMany(ctxA, []nd.Configuration{c}); e != nil {
				return e
			}
			audit, e := ap.NewAuditRepository(tx)
			if e != nil {
				return e
			}
			return audit.AppendAuditEvent(ctxA, ad.AuditEvent{EventID: "n184-event-" + stamp, AuditID: "n184-audit-" + stamp, EventType: ad.AuditEventOutcome, TenantID: tenantA, ActorUserID: user, OperationID: "notification.configuration.create", Module: "notification", Outcome: ad.AuditResultSuccess, Risk: ad.AuditRiskLow, OccurredAt: now})
		})
		must(db.Callback().Create().Remove(callback))
		if !errors.Is(e, injected) {
			t.Fatalf("audit injection missed: %v", e)
		}
		e = transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			_, e := r.Get(ctxA, tenantA, c.ID, false)
			return e
		})
		if !errors.Is(e, nd.ErrConfigurationNotFound) {
			t.Fatalf("audit failure committed config: %v", e)
		}
	})
	t.Run("concurrent-unique-combination-has-one-winner", func(t *testing.T) {
		start := make(chan struct{})
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			c := config(fmt.Sprintf("race-%d", i), tenantA, "race-site", nd.LevelGeneral)
			go func(c nd.Configuration) {
				<-start
				results <- transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
					return r.CreateMany(ctxA, []nd.Configuration{c})
				})
			}(c)
		}
		close(start)
		wins, conflicts := 0, 0
		for i := 0; i < 2; i++ {
			e := <-results
			if e == nil {
				wins++
			} else if errors.Is(e, nd.ErrConfigurationDuplicate) {
				conflicts++
			} else {
				t.Errorf("unexpected race result %v", e)
			}
		}
		if wins != 1 || conflicts != 1 {
			t.Fatalf("wins=%d conflicts=%d", wins, conflicts)
		}
	})
	t.Run("delete-CAS-and-no-orphan-relations", func(t *testing.T) {
		c := get(ctxA, tenantA, initial[0].ID)
		e := transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error { return r.Delete(ctxA, tenantA, c.ID, 1) })
		if !errors.Is(e, nd.ErrConfigurationConflict) {
			t.Fatalf("stale delete accepted %v", e)
		}
		must(transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error { return r.Delete(ctxA, tenantA, c.ID, c.Version) }))
		for _, table := range []string{"biz_notification_configuration_channels", "biz_notification_configuration_recipients"} {
			var n int64
			must(db.Table(table).Where("tenant_id = ? AND configuration_id = ?", tenantA, c.ID).Count(&n).Error)
			if n != 0 {
				t.Fatalf("orphan %s count=%d", table, n)
			}
		}
	})
	t.Run("locked-read-does-not-mix-current-parent-with-old-relations", func(t *testing.T) {
		initial := config("snapshot", tenantA, "snapshot-site", nd.LevelGeneral)
		initial.AdditionalUserIDs = []string{"before-recipient"}
		must(transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			return r.CreateMany(ctxA, []nd.Configuration{initial})
		}))
		must(transact(ctxA, func(r *np.ConfigurationRepository, _ *gorm.DB) error {
			before, err := r.Get(ctxA, tenantA, initial.ID, false)
			if err != nil {
				return err
			}
			// Establish a Repeatable Read snapshot, then commit another transaction.
			next := before
			next.Version = 2
			next.Channels = []string{"email"}
			next.AdditionalUserIDs = []string{"after-recipient"}
			next.UpdatedAt = now.Add(time.Second)
			if err := transact(ctxA, func(other *np.ConfigurationRepository, _ *gorm.DB) error { return other.Replace(ctxA, next, 1) }); err != nil {
				return err
			}
			current, err := r.Get(ctxA, tenantA, initial.ID, true)
			if err != nil {
				return err
			}
			if current.Version != 2 || !reflect.DeepEqual(current.Channels, next.Channels) || !reflect.DeepEqual(current.AdditionalUserIDs, next.AdditionalUserIDs) {
				t.Fatalf("mixed current/snapshot read: %+v", current)
			}
			return nil
		}))
	})
	t.Run("Access-directory-current-member-and-scope", func(t *testing.T) {
		must(transact(ctxA, func(_ *np.ConfigurationRepository, tx *gorm.DB) error {
			d, e := ap.NewNotificationRecipientDirectory(tx)
			if e != nil {
				return e
			}
			if e = d.LockCaller(ctxA, tenantA, true); e != nil {
				return e
			}
			items, total, e := d.List(ctxA, tenantA, "", 1, 100)
			if e != nil {
				return e
			}
			if total != 1 || len(items) != 1 || items[0].UserID != user {
				t.Fatalf("directory %+v total=%d", items, total)
			}
			if e = d.Validate(ctxA, tenantA, []string{user}, true); e != nil {
				return e
			}
			if e = d.Validate(ctxA, tenantA, []string{"not-a-member"}, true); !errors.Is(e, ap.ErrUnauthorized) {
				t.Fatalf("invalid recipient accepted: %v", e)
			}
			if e = d.Validate(ctxA, tenantB, []string{user}, true); !errors.Is(e, ap.ErrUnauthorized) {
				t.Fatalf("cross-tenant directory accepted: %v", e)
			}
			scope, e := d.ManagementScope(ctxA, tenantA, user, "tenant.notification.create", true)
			if e != nil {
				return e
			}
			if !scope.All {
				t.Fatalf("lost current grant %+v", scope)
			}
			return nil
		}))
		must(db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", tenantA, user).Update("status", "disabled").Error)
		e := transact(ctxA, func(_ *np.ConfigurationRepository, tx *gorm.DB) error {
			d, e := ap.NewNotificationRecipientDirectory(tx)
			if e != nil {
				return e
			}
			return d.LockCaller(ctxA, tenantA, true)
		})
		if !errors.Is(e, ap.ErrUnauthorized) {
			t.Fatalf("disabled caller accepted: %v", e)
		}
	})
}
