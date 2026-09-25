//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	ap "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

// This is the real Access-directory prerequisite for Notification configuration,
// not configuration CRUD, generated HTTP authority, or browser acceptance.
func TestEnterprise184NotificationDirectoryUsesCurrentTenantAndRootLocks(t *testing.T) {
	db := openDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	store, err := ap.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	stamp := fmt.Sprint(time.Now().UnixNano())
	a, b, user := "nd184-a-"+stamp, "nd184-b-"+stamp, "nd184-u-"+stamp
	for _, tenant := range []string{a, b} {
		if err = store.Bootstrap(ctx, ap.Bootstrap{TenantID: tenant, TenantName: tenant, UserID: user, Email: "nd184-" + stamp + "@example.invalid", Token: "nd184-token-" + tenant}, []authz.PermissionKey{"tenant.notification.create"}); err != nil {
			t.Fatal(err)
		}
	}
	p, err := store.Authenticate(ctx, "nd184-token-"+a)
	if err != nil {
		t.Fatal(err)
	}
	actor := identity.WithPrincipal(ctx, p)
	root := func(fn func(*ap.NotificationRecipientDirectory) error) error {
		return db.WithContext(actor).Transaction(func(tx *gorm.DB) error {
			d, err := ap.NewNotificationRecipientDirectory(tx)
			if err != nil {
				return err
			}
			return fn(d)
		})
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Run("autocommit-cannot-claim-row-lock-protection", func(t *testing.T) {
		d, err := ap.NewNotificationRecipientDirectory(db)
		must(err)
		if d.LockCaller(actor, a, true) == nil {
			t.Fatal("autocommit identity lock accepted")
		}
		if d.Validate(actor, a, []string{user}, true) == nil {
			t.Fatal("autocommit recipient lock accepted")
		}
	})
	t.Run("independent-tenant-user-membership-locking-queries", func(t *testing.T) {
		must(root(func(d *ap.NotificationRecipientDirectory) error {
			if err := d.LockCaller(actor, a, true); err != nil {
				return err
			}
			if err := d.Validate(actor, a, []string{user}, true); err != nil {
				return err
			}
			scope, err := d.ManagementScope(actor, a, user, "tenant.notification.create", true)
			if err != nil {
				return err
			}
			if !scope.All {
				t.Fatalf("legitimate grant lost: %+v", scope)
			}
			return nil
		}))
	})
	t.Run("filtered-total-and-pages-share-current-tenant", func(t *testing.T) {
		must(root(func(d *ap.NotificationRecipientDirectory) error {
			for page := 1; page <= 2; page++ {
				items, total, err := d.List(actor, a, user, page, 1)
				if err != nil {
					return err
				}
				if total != 1 || len(items) != 2-page {
					t.Fatalf("page=%d total=%d items=%+v", page, total, items)
				}
				if page == 1 && items[0].UserID != user {
					t.Fatal("wrong identity")
				}
			}
			items, total, err := d.List(actor, a, "absent", 1, 100)
			if err != nil {
				return err
			}
			if total != 0 || len(items) != 0 {
				t.Fatal("filter ignored")
			}
			return nil
		}))
	})
	t.Run("missing-session-foreign-tenant-and-invalid-recipient-rejected", func(t *testing.T) {
		must(root(func(d *ap.NotificationRecipientDirectory) error {
			for _, c := range []struct {
				ctx    context.Context
				tenant string
			}{{ctx, a}, {actor, b}} {
				if _, _, err := d.List(c.ctx, c.tenant, "", 1, 100); !errors.Is(err, ap.ErrUnauthorized) {
					t.Fatalf("directory leak: %v", err)
				}
				if err := d.LockCaller(c.ctx, c.tenant, true); !errors.Is(err, ap.ErrUnauthorized) {
					t.Fatalf("identity override: %v", err)
				}
			}
			for _, ids := range [][]string{nil, {"not-a-member"}, {user, user}} {
				if err := d.Validate(actor, a, ids, true); !errors.Is(err, ap.ErrUnauthorized) {
					t.Fatalf("invalid recipients %v: %v", ids, err)
				}
			}
			return nil
		}))
	})
	t.Run("grant-removal-takes-effect-on-next-read", func(t *testing.T) {
		must(db.Table("biz_permission_grants").Where("tenant_id = ? AND permission = ?", a, "tenant.notification.create").Delete(map[string]any{}).Error)
		must(root(func(d *ap.NotificationRecipientDirectory) error {
			scope, err := d.ManagementScope(actor, a, user, "tenant.notification.create", true)
			if err != nil {
				return err
			}
			if scope.All || scope.Self || scope.Sites {
				t.Fatalf("revoked scope survived: %+v", scope)
			}
			return nil
		}))
	})
	t.Run("disabled-member-excluded-and-cannot-lock-caller", func(t *testing.T) {
		must(db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", a, user).Update("status", "disabled").Error)
		must(root(func(d *ap.NotificationRecipientDirectory) error {
			if err := d.LockCaller(actor, a, true); !errors.Is(err, ap.ErrUnauthorized) {
				t.Fatalf("disabled identity allowed: %v", err)
			}
			if err := d.Validate(actor, a, []string{user}, true); !errors.Is(err, ap.ErrUnauthorized) {
				t.Fatalf("disabled recipient allowed: %v", err)
			}
			items, total, err := d.List(actor, a, "", 1, 100)
			if err != nil {
				return err
			}
			if total != 0 || len(items) != 0 {
				t.Fatal("disabled member remained a candidate")
			}
			return nil
		}))
	})
}
