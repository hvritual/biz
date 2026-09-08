//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/access/ports"
)

// Both transactions overlap, but commit order is controlled without sleeps.
// The loser establishes a real RR snapshot BEFORE the winner's mutation. The
// owner lock must not permit a later invariant query to reuse that stale view.
func TestAG02OwnerInvariantUsesCurrentReadAfterSnapshot(t *testing.T) {
	for _, first := range []string{"revoke", "suspend"} {
		t.Run(first, func(t *testing.T) {
			db := openDB(t)
			startB123Runtime(t, db)
			stamp := fmt.Sprint(time.Now().UnixNano())
			tenant, a, b := "ag02-snapshot-"+stamp, "ag02-a-"+stamp, "ag02-b-"+stamp
			seedB123TenantAdmin(t, db, tenant, a, a+"@example.invalid", "ag02-token-a-"+stamp)
			seedB124PlainMember(t, db, tenant, b, b+"@example.invalid", "ag02-token-b-"+stamp)
			role := tenant + ":owner"
			if err := db.Exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,?)", role, tenant, domain.TenantOwnerRoleName, domain.TenantRoleStatusActive, 1).Error; err != nil {
				t.Fatal(err)
			}
			for _, user := range []string{a, b} {
				if err := db.Exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenant, user, role).Error; err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			loser := db.WithContext(ctx).Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead})
			if loser.Error != nil {
				t.Fatal(loser.Error)
			}
			defer loser.Rollback()
			var before int64
			if err := loser.Table("biz_memberships").Where("tenant_id = ? AND status = ?", tenant, domain.TenantMemberStatusActive).Count(&before).Error; err != nil {
				t.Fatal(err)
			}
			if before != 2 {
				t.Fatalf("fixture active members=%d", before)
			}
			winner := db.WithContext(ctx).Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead})
			if winner.Error != nil {
				t.Fatal(winner.Error)
			}
			defer winner.Rollback()
			wr, err := persistence.NewTenantRoleRepository(winner)
			if err != nil {
				t.Fatal(err)
			}
			if first == "revoke" {
				if _, err := wr.RevokeMember(ctx, tenant, role, a); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := wr.AssertMemberCanDeactivate(ctx, tenant, b); err != nil {
					t.Fatal(err)
				}
				result := winner.Table("biz_memberships").Where("tenant_id = ? AND user_id = ? AND version = ?", tenant, b, 1).Updates(map[string]any{"status": domain.TenantMemberStatusSuspended, "version": 2})
				if result.Error != nil || result.RowsAffected != 1 {
					t.Fatalf("winner update rows=%d err=%v", result.RowsAffected, result.Error)
				}
			}
			if err := winner.Commit().Error; err != nil {
				t.Fatal(err)
			}
			lr, err := persistence.NewTenantRoleRepository(loser)
			if err != nil {
				t.Fatal(err)
			}
			if first == "revoke" {
				err = lr.AssertMemberCanDeactivate(ctx, tenant, b)
			} else {
				_, err = lr.RevokeMember(ctx, tenant, role, a)
			}
			if !errors.Is(err, ports.ErrLastTenantOwner) {
				t.Fatalf("stale owner snapshot accepted after %s: err=%v", first, err)
			}
			if err := loser.Rollback().Error; err != nil {
				t.Fatal(err)
			}
			var remaining int64
			if err := db.Table("biz_member_roles mr").Joins("JOIN biz_memberships m ON m.tenant_id=mr.tenant_id AND m.user_id=mr.user_id AND m.status=?", domain.TenantMemberStatusActive).Where("mr.tenant_id=? AND mr.role_id=?", tenant, role).Count(&remaining).Error; err != nil {
				t.Fatal(err)
			}
			if remaining != 1 {
				t.Fatalf("remaining owners=%d", remaining)
			}
		})
	}
}
