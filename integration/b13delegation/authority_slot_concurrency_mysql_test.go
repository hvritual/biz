//go:build integration

package b13delegation

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/execution"
	"yunka.io/framework/requestscope"
)

func TestB136StableAuthoritySlotRepeatedExpiredRegrant(t *testing.T) {
	db := openDB(t)
	if err := accesspersistence.AutoMigrateTenantDelegation(context.Background(), db); err != nil {
		t.Fatal(err)
	}

	transactions, err := requestscope.NewGORMExecutionFactory(db)
	if err != nil {
		t.Fatal(err)
	}
	delegationFactory, err := accesspersistence.NewTenantDelegationRepositoryFactory(db)
	if err != nil {
		t.Fatal(err)
	}

	stamp := fmt.Sprint(time.Now().UnixNano())
	ownerTenantID := "b13-slot-owner-" + stamp
	granteeTenantID := "b13-slot-grantee-" + stamp
	resourceID := "device-b13-slot-" + stamp

	runGrant := func(delegationID string, expiresAt *time.Time) (accessdomain.TenantDelegation, error) {
		value := accessdomain.NewTenantDelegation(
			delegationID,
			ownerTenantID,
			granteeTenantID,
			accessdomain.TenantDelegationResourceDevice,
			resourceID,
			[]string{"device.read"},
			expiresAt,
			time.Now().UTC(),
		)
		ctx, root, err := b136Root(transactions, ownerTenantID, "tenant.delegation.grant_device", execution.TransactionLocal)
		if err != nil {
			return accessdomain.TenantDelegation{}, err
		}
		created, err := requestscope.JoinValue(ctx, delegationFactory, func(scope *requestscope.View[accessports.TenantDelegationRepositories]) (accessdomain.TenantDelegation, error) {
			return scope.Repositories().Delegation.CreateOrGetActive(scope.Context(), &value)
		})
		if err != nil {
			_ = root.Rollback(context.Background())
			return accessdomain.TenantDelegation{}, err
		}
		if err := root.Commit(context.Background()); err != nil {
			return accessdomain.TenantDelegation{}, err
		}
		return created, nil
	}

	initialExpiry := time.Now().UTC().Truncate(time.Millisecond).Add(time.Hour)
	current, err := runGrant("delegation-b13-slot-initial-"+stamp, &initialExpiry)
	if err != nil {
		t.Fatal(err)
	}

	const (
		rounds  = 6
		workers = 8
	)
	for round := 0; round < rounds; round++ {
		past := time.Now().UTC().Add(-time.Minute)
		if err := db.Table("biz_tenant_delegations").Where("id = ?", current.ID).Update("expires_at", past).Error; err != nil {
			t.Fatal(err)
		}

		freshExpiry := time.Now().UTC().Truncate(time.Millisecond).Add(time.Duration(round+2) * time.Hour)
		start := make(chan struct{})
		results := make(chan concurrentDelegationResult, workers)
		var group sync.WaitGroup
		for index := 0; index < workers; index++ {
			group.Add(1)
			go func(index int) {
				defer group.Done()
				<-start
				value, err := runGrant(fmt.Sprintf("delegation-b13-slot-%s-r%d-w%d", stamp, round, index), &freshExpiry)
				results <- concurrentDelegationResult{value: value, err: err}
			}(index)
		}
		close(start)
		group.Wait()
		close(results)

		previousID := current.ID
		winnerID := ""
		for result := range results {
			if result.err != nil {
				t.Fatalf("round %d expired concurrent regrant: %v", round, result.err)
			}
			if result.value.ID == previousID {
				t.Fatalf("round %d reused expired authority %s", round, previousID)
			}
			if winnerID == "" {
				winnerID = result.value.ID
			} else if result.value.ID != winnerID {
				t.Fatalf("round %d produced multiple fresh authorities: %s and %s", round, winnerID, result.value.ID)
			}
			current = result.value
		}
		if winnerID == "" {
			t.Fatalf("round %d produced no winner", round)
		}

		var oldActiveKeyCount int64
		if err := db.Table("biz_tenant_delegations").Where("id = ? AND active_key IS NOT NULL", previousID).Count(&oldActiveKeyCount).Error; err != nil {
			t.Fatal(err)
		}
		if oldActiveKeyCount != 0 {
			t.Fatalf("round %d expired historical authority still owns active key", round)
		}
		assertSingleEffectiveAuthority(t, db, ownerTenantID, granteeTenantID, resourceID)
	}

	var slotCount int64
	if err := db.Table("biz_tenant_delegation_authority_slots").Count(&slotCount).Error; err != nil {
		t.Fatal(err)
	}
	if slotCount != 1 {
		t.Fatalf("stable authority slot count=%d want=1", slotCount)
	}
}
