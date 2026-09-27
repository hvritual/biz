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

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	ap "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	dp "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	deviceports "github.com/hvritual/biz/internal/deviceops/ports"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

var _ accessports.NotificationManagementScopes = (*ap.NotificationRecipientDirectory)(nil)

// This qualification covers the resource-directory prerequisite only. It does
// not claim that Notification configuration writes or HTTP/UI are connected.
func TestEnterprise184NotificationSiteDirectoryUsesCurrentScopesAndRootLocks(t *testing.T) {
	db := openDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	must := func(t *testing.T, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	store, err := ap.New(db)
	must(t, err)
	must(t, store.AutoMigrate(ctx))
	must(t, dp.AutoMigrate(ctx, db))
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenant, other, user := "ns184-a-"+stamp, "ns184-b-"+stamp, "ns184-u-"+stamp
	permission := "tenant.notification.read"
	for _, id := range []string{tenant, other} {
		must(t, store.Bootstrap(ctx, ap.Bootstrap{TenantID: id, TenantName: id, UserID: user, Email: "ns184-" + stamp + "@example.invalid", Token: "ns184-token-" + id}, []authz.PermissionKey{authz.PermissionKey(permission)}))
	}
	principal, err := store.Authenticate(ctx, "ns184-token-"+tenant)
	must(t, err)
	actor := identity.WithPrincipal(ctx, principal)
	now := time.Now().UTC()
	const count = 103
	ids := make([]string, 0, count)
	site := func(id, owner, name string, retired bool) {
		t.Helper()
		row := dp.SitePORecord{SitePO: dp.SitePO{Name: name}, SitePOBase: dp.SitePOBase{ID: id, TenantID: owner, Version: 1, CreatedAt: now, UpdatedAt: now}}
		if retired {
			row.DeletedAt = gorm.DeletedAt{Time: now, Valid: true}
		}
		must(t, db.WithContext(ctx).Create(&row).Error)
	}
	assign := func(tx *gorm.DB, id string) error {
		return tx.Table("biz_member_sites").Create(map[string]any{"tenant_id": tenant, "user_id": user, "site_id": id}).Error
	}
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("ns184-%s-%03d", stamp, i)
		ids = append(ids, id)
		name := fmt.Sprintf("Group %03d", i)
		if i == 7 {
			name = "Literal_100%"
		}
		site(id, tenant, name, false)
		must(t, assign(db, id))
	}
	foreign, retired, outside := "ns184-foreign-"+stamp, "ns184-retired-"+stamp, "ns184-outside-"+stamp
	site(foreign, other, "Foreign", false)
	site(retired, tenant, "Retired", true)
	site(outside, tenant, "Outside", false)
	// Deliberately inconsistent references must not defeat resource authority.
	must(t, assign(db, foreign))
	must(t, assign(db, retired))
	directory := func(tx *gorm.DB, action string) (*dp.NotificationSiteDirectory, error) {
		scopes, err := ap.NewNotificationRecipientDirectory(tx)
		if err != nil {
			return nil, err
		}
		return dp.NewNotificationSiteDirectory(tx, scopes, action)
	}
	root := func(fn func(*gorm.DB, *dp.NotificationSiteDirectory) error) error {
		return db.WithContext(actor).Transaction(func(tx *gorm.DB) error {
			d, err := directory(tx, permission)
			if err != nil {
				return err
			}
			return fn(tx, d)
		})
	}
	rollback := errors.New("rollback-directory-fixture")
	rolledBack := func(t *testing.T, fn func(*gorm.DB, *dp.NotificationSiteDirectory) error) {
		t.Helper()
		err := root(fn)
		if !errors.Is(err, rollback) {
			t.Fatalf("fixture rollback: %v", err)
		}
	}
	t.Run("constructor-and-autocommit-fail-closed", func(t *testing.T) {
		scopes, err := ap.NewNotificationRecipientDirectory(db)
		must(t, err)
		for _, p := range []string{"", "device.read", "tenant.notification.read ", "tenant.notification"} {
			if _, err := dp.NewNotificationSiteDirectory(db, scopes, p); !errors.Is(err, deviceports.ErrNotificationSiteDirectoryUnavailable) {
				t.Fatalf("bad adapter permission %q: %v", p, err)
			}
		}
		if _, err := dp.NewNotificationSiteDirectory(nil, scopes, permission); err == nil {
			t.Fatal("nil db accepted")
		}
		if _, err := dp.NewNotificationSiteDirectory(db, nil, permission); err == nil {
			t.Fatal("nil scope reader accepted")
		}
		d, err := directory(db, permission)
		must(t, err)
		if _, _, err := d.List(actor, "", 1, 100); !errors.Is(err, deviceports.ErrNotificationSiteDirectoryUnavailable) {
			t.Fatalf("autocommit read: %v", err)
		}
		if _, err := d.Resolve(actor, ids[:1], true); !errors.Is(err, deviceports.ErrNotificationSiteDirectoryUnavailable) {
			t.Fatalf("autocommit lock: %v", err)
		}
	})
	t.Run("trusted-identity-required", func(t *testing.T) {
		must(t, root(func(_ *gorm.DB, d *dp.NotificationSiteDirectory) error {
			for _, missing := range []context.Context{ctx, nil} {
				rows, total, err := d.List(missing, "", 1, 10)
				if !errors.Is(err, deviceports.ErrNotificationSiteScopeDenied) || rows != nil || total != 0 {
					t.Fatalf("identity leak: %v %d %v", rows, total, err)
				}
			}
			invalid := principal
			invalid.Authenticated = false
			if _, err := d.Resolve(identity.WithPrincipal(ctx, invalid), ids[:1], true); !errors.Is(err, deviceports.ErrNotificationSiteScopeDenied) {
				t.Fatalf("unauthenticated actor: %v", err)
			}
			return nil
		}))
	})
	t.Run("all-grant-cannot-replace-explicit-member-scope", func(t *testing.T) {
		rolledBack(t, func(tx *gorm.DB, d *dp.NotificationSiteDirectory) error {
			if err := tx.Table("biz_member_sites").Where("tenant_id = ? AND user_id = ?", tenant, user).Delete(map[string]any{}).Error; err != nil {
				return err
			}
			rows, total, err := d.List(actor, "", 1, 10)
			if err != nil {
				return err
			}
			if rows == nil || len(rows) != 0 || total != 0 {
				t.Fatalf("ALL widened explicit scope: %+v %d", rows, total)
			}
			if _, err := d.Resolve(actor, ids[:1], true); !errors.Is(err, deviceports.ErrNotFound) {
				t.Fatalf("unassigned site: %v", err)
			}
			return rollback
		})
	})
	t.Run("single-statement-pagination-count-and-resource-tenant", func(t *testing.T) {
		must(t, root(func(_ *gorm.DB, d *dp.NotificationSiteDirectory) error {
			seen := map[string]bool{}
			for page, want := range []int{100, 3, 0} {
				rows, total, err := d.List(actor, "", page+1, 100)
				if err != nil {
					return err
				}
				if int(total) != count || len(rows) != want || rows == nil {
					t.Fatalf("page=%d total=%d rows=%d", page+1, total, len(rows))
				}
				for _, row := range rows {
					if row.ID == foreign || row.ID == retired || row.ID == outside || seen[row.ID] || row.Version != 1 {
						t.Fatalf("invalid visible row: %+v", row)
					}
					seen[row.ID] = true
				}
			}
			if len(seen) != count {
				t.Fatalf("lost rows: %d", len(seen))
			}
			return nil
		}))
	})
	t.Run("literal-filtering-invalid-input-and-large-page", func(t *testing.T) {
		must(t, root(func(_ *gorm.DB, d *dp.NotificationSiteDirectory) error {
			for _, q := range []string{"%", "_", "Literal_100%"} {
				rows, total, err := d.List(actor, q, 1, 100)
				if err != nil {
					return err
				}
				if total != 1 || len(rows) != 1 || rows[0].ID != ids[7] {
					t.Fatalf("literal query %q: %+v %d", q, rows, total)
				}
			}
			for _, q := range []string{"missing", "' OR 1=1 --", "literal_100%"} {
				rows, total, err := d.List(actor, q, 1, 10)
				if err != nil {
					return err
				}
				if total != 0 || rows == nil || len(rows) != 0 {
					t.Fatalf("filter %q ignored", q)
				}
			}
			rows, total, err := d.List(actor, "", 1000000, 100)
			if err != nil {
				return err
			}
			if total != count || len(rows) != 0 {
				t.Fatal("out-of-range total lost")
			}
			for _, q := range []string{strings.Repeat("x", 201), "bad\nquery", string([]byte{0xff})} {
				if _, _, err := d.List(actor, q, 1, 10); !errors.Is(err, deviceports.ErrNotificationSiteQueryInvalid) {
					t.Fatalf("bad query accepted: %v", err)
				}
			}
			for _, p := range [][2]int{{0, 1}, {1, 0}, {-1, 1}, {1000001, 1}, {1, 101}} {
				if _, _, err := d.List(actor, "", p[0], p[1]); !errors.Is(err, deviceports.ErrNotificationSiteQueryInvalid) {
					t.Fatalf("bad page accepted: %v", err)
				}
			}
			return nil
		}))
	})
	t.Run("resolution-is-whole-set-exact-and-current", func(t *testing.T) {
		must(t, root(func(_ *gorm.DB, d *dp.NotificationSiteDirectory) error {
			input := []string{ids[1], ids[0]}
			before := append([]string{}, input...)
			rows, err := d.Resolve(actor, input, true)
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(input, before) || len(rows) != 2 || rows[0].ID != ids[0] {
				t.Fatalf("unstable resolution: %v %+v", input, rows)
			}
			for _, bad := range []string{foreign, retired, outside, "missing", strings.ToUpper(ids[0])} {
				rows, err := d.Resolve(actor, []string{ids[1], bad}, true)
				if !errors.Is(err, deviceports.ErrNotFound) || rows != nil {
					t.Fatalf("partial/foreign resolution for %s: %+v %v", bad, rows, err)
				}
			}
			for _, bad := range [][]string{nil, {}, {ids[0], ids[0]}, {" "}, {strings.Repeat("x", 65)}} {
				if rows, err := d.Resolve(actor, bad, true); !errors.Is(err, deviceports.ErrNotificationSiteQueryInvalid) || rows != nil {
					t.Fatalf("invalid set: %+v %v", rows, err)
				}
			}
			return nil
		}))
	})
	t.Run("read-grant-does-not-authorize-write-directory", func(t *testing.T) {
		must(t, root(func(tx *gorm.DB, _ *dp.NotificationSiteDirectory) error {
			for _, p := range []string{"tenant.notification.create", "tenant.notification.update", "tenant.notification.delete"} {
				d, err := directory(tx, p)
				if err != nil {
					return err
				}
				rows, err := d.Resolve(actor, ids[:1], true)
				if !errors.Is(err, deviceports.ErrNotificationSiteScopeDenied) || rows != nil {
					t.Fatalf("read grant used for %s: %v", p, err)
				}
			}
			return nil
		}))
	})
	t.Run("retired-site-is-rechecked-after-candidate-selection", func(t *testing.T) {
		rolledBack(t, func(tx *gorm.DB, d *dp.NotificationSiteDirectory) error {
			if _, err := d.Resolve(actor, ids[:1], false); err != nil {
				return err
			}
			if err := tx.Model(&dp.SitePORecord{}).Where("id = ?", ids[0]).Update("deleted_at", now).Error; err != nil {
				return err
			}
			if rows, err := d.Resolve(actor, ids[:1], true); !errors.Is(err, deviceports.ErrNotFound) || rows != nil {
				t.Fatalf("retired candidate survived: %+v %v", rows, err)
			}
			return rollback
		})
	})
	t.Run("committed-member-scope-contraction-next-request", func(t *testing.T) {
		must(t, db.Table("biz_member_sites").Where("tenant_id = ? AND user_id = ? AND site_id = ?", tenant, user, ids[0]).Delete(map[string]any{}).Error)
		defer func() { must(t, assign(db, ids[0])) }()
		must(t, root(func(_ *gorm.DB, d *dp.NotificationSiteDirectory) error {
			rows, err := d.Resolve(actor, ids[:1], true)
			if !errors.Is(err, deviceports.ErrNotFound) || rows != nil {
				t.Fatalf("stale member scope: %v", err)
			}
			_, total, err := d.List(actor, "", 1, 100)
			if err != nil {
				return err
			}
			if total != count-1 {
				t.Fatalf("stale total %d", total)
			}
			return nil
		}))
	})
	t.Run("policy-ceiling-and-revocation-do-not-widen", func(t *testing.T) {
		rolledBack(t, func(tx *gorm.DB, d *dp.NotificationSiteDirectory) error {
			policyID := "ns184-policy-" + stamp
			p, err := ap.NewTenantDataPolicyRepository(tx)
			if err != nil {
				return err
			}
			if err := p.Create(actor, &accessdomain.DataPolicy{ID: policyID, TenantID: tenant, Name: "Notification scope", Status: "active", Version: 1, CreatedAt: now, UpdatedAt: now, SiteIDs: []string{ids[0], foreign, outside}}); err != nil {
				return err
			}
			if err := tx.Table("biz_roles").Where("tenant_id = ?", tenant).Update("data_policy_id", policyID).Error; err != nil {
				return err
			}
			rows, total, err := d.List(actor, "", 1, 100)
			if err != nil {
				return err
			}
			if total != 1 || len(rows) != 1 || rows[0].ID != ids[0] {
				t.Fatalf("policy/resource intersection wrong: %+v %d", rows, total)
			}
			if err := tx.Table("biz_data_policies").Where("tenant_id = ? AND id = ?", tenant, policyID).Update("status", "revoked").Error; err != nil {
				return err
			}
			if rows, _, err := d.List(actor, "", 1, 100); !errors.Is(err, accessdomain.ErrInvalidTenantDataPolicy) || rows != nil {
				t.Fatalf("revoked policy fallback: %+v %v", rows, err)
			}
			return rollback
		})
	})
	t.Run("scope-reader-and-database-errors-are-not-empty-success", func(t *testing.T) {
		rolledBack(t, func(tx *gorm.DB, d *dp.NotificationSiteDirectory) error {
			if err := tx.Table("biz_permission_grants").Where("tenant_id = ? AND permission = ?", tenant, permission).Delete(map[string]any{}).Error; err != nil {
				return err
			}
			rows, total, err := d.List(actor, "", 1, 100)
			if !errors.Is(err, deviceports.ErrNotificationSiteScopeDenied) || rows != nil || total != 0 {
				t.Fatalf("revoked grant became empty success: %v", err)
			}
			return rollback
		})
		tx := db.WithContext(actor).Begin()
		must(t, tx.Error)
		d, err := directory(tx, permission)
		must(t, err)
		must(t, tx.Rollback().Error)
		if rows, _, err := d.List(actor, "", 1, 10); err == nil || rows != nil {
			t.Fatalf("closed transaction became empty success: %v", err)
		}
		c, cancelled := context.WithCancel(actor)
		cancelled()
		must(t, root(func(_ *gorm.DB, d *dp.NotificationSiteDirectory) error {
			if rows, _, err := d.List(c, "", 1, 10); !errors.Is(err, context.Canceled) || rows != nil {
				t.Fatalf("cancelled query: %v", err)
			}
			return nil
		}))
	})
	t.Run("site-shared-lock-blocks-retirement-until-root-ends", func(t *testing.T) {
		tx := db.WithContext(actor).Begin()
		must(t, tx.Error)
		defer tx.Rollback()
		d, err := directory(tx, permission)
		must(t, err)
		_, err = d.Resolve(actor, ids[:1], true)
		must(t, err)
		connection := make(chan uint64, 1)
		result := make(chan error, 1)
		go func() {
			c, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			result <- db.WithContext(c).Transaction(func(other *gorm.DB) error {
				var id uint64
				if err := other.Raw("SELECT CONNECTION_ID()").Scan(&id).Error; err != nil {
					return err
				}
				connection <- id
				return other.Model(&dp.SitePORecord{}).Where("id = ?", ids[0]).Update("deleted_at", now).Error
			})
		}()
		var connectionID uint64
		select {
		case connectionID = <-connection:
		case err := <-result:
			t.Fatalf("contender failed before update: %v", err)
		case <-time.After(2 * time.Second):
			t.Fatal("contender did not start")
		}
		deadline := time.Now().Add(2 * time.Second)
		waiting := false
		for time.Now().Before(deadline) {
			var n int64
			must(t, db.Raw("SELECT COUNT(*) FROM information_schema.innodb_trx WHERE trx_mysql_thread_id = ? AND trx_state = 'LOCK WAIT'", connectionID).Scan(&n).Error)
			if n > 0 {
				waiting = true
				break
			}
			select {
			case err := <-result:
				t.Fatalf("site retired while root still holds lock: %v", err)
			case <-time.After(20 * time.Millisecond):
			}
		}
		if !waiting {
			t.Fatal("server did not report a real InnoDB lock wait")
		}
		must(t, tx.Rollback().Error)
		select {
		case err := <-result:
			must(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("contender did not resume after root ended")
		}
		must(t, db.Unscoped().Model(&dp.SitePORecord{}).Where("id = ?", ids[0]).Update("deleted_at", nil).Error)
	})
}
