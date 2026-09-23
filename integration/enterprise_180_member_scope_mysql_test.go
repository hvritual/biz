//go:build integration

package integration

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

func TestEnterprise180MemberBusinessScopeAuthority(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)

	tenantA, tenantB := "enterprise180-scope-a-"+stamp, "enterprise180-scope-b-"+stamp
	adminA, adminB := "enterprise180-scope-admin-a-"+stamp, "enterprise180-scope-admin-b-"+stamp
	tokenA, tokenB := "enterprise180-scope-token-a-"+stamp, "enterprise180-scope-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, adminA, adminA+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, adminB, adminB+"@example.invalid", tokenB)

	memberA, memberB := "enterprise180-member-a-"+stamp, "enterprise180-member-b-"+stamp
	seedB124PlainMember(t, db, tenantA, memberA, memberA+"@example.invalid", "member-a-token-"+stamp)
	seedB124PlainMember(t, db, tenantB, memberB, memberB+"@example.invalid", "member-b-token-"+stamp)

	now := time.Now().UTC()
	siteA, siteA2, siteB := "scope-site-a-"+stamp, "scope-site-a2-"+stamp, "scope-site-b-"+stamp
	for _, site := range []struct{ tenant, id, name string }{
		{tenantA, siteA, "Alpha Site"}, {tenantA, siteA2, "Beta Site"}, {tenantB, siteB, "Foreign Site"},
	} {
		if err := db.Exec("INSERT INTO biz_deviceops_site (id,tenant_id,name,version,created_at,updated_at) VALUES (?,?,?,?,?,?)", site.id, site.tenant, site.name, 1, now, now).Error; err != nil {
			t.Fatal(err)
		}
	}

	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(dialCtx, started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	members := accessv1.NewTenantMemberLifecycleApplicationClient(connection)
	authCtx := func(token string) context.Context {
		return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	}
	writeCtx := func(token, key string) context.Context {
		return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token, "idempotency-key", key+":"+stamp)
	}

	candidates, err := members.ListTenantMemberScopeCandidates(authCtx(tokenA), &accessv1.ListTenantMemberScopeCandidatesRequest{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	candidateIDs := make([]string, 0, len(candidates.GetCandidates()))
	for _, candidate := range candidates.GetCandidates() {
		if !candidate.GetAssignable() || candidate.GetUnavailableReason() != "" {
			t.Fatalf("unexpected unavailable candidate=%+v", candidate)
		}
		candidateIDs = append(candidateIDs, candidate.GetId())
	}
	sort.Strings(candidateIDs)
	if got := fmt.Sprint(candidateIDs); got != fmt.Sprint([]string{siteA, siteA2}) {
		t.Fatalf("tenant A candidates=%v want current-tenant sites only", candidateIDs)
	}

	initial, err := members.GetTenantMemberBusinessScope(authCtx(tokenA), &accessv1.GetTenantMemberBusinessScopeRequest{UserId: memberA})
	if err != nil {
		t.Fatal(err)
	}
	if initial.GetTenantId() != tenantA || initial.GetVersion() != 1 || len(initial.GetSiteIds()) != 0 {
		t.Fatalf("initial scope=%+v", initial)
	}

	setRequest := &accessv1.SetTenantMemberBusinessScopeRequest{UserId: memberA, Version: initial.GetVersion(), SiteIds: []string{siteA2, siteA}}
	idempotentCtx := writeCtx(tokenA, "set-initial")
	first, err := members.SetTenantMemberBusinessScope(idempotentCtx, setRequest)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := members.SetTenantMemberBusinessScope(idempotentCtx, setRequest)
	if err != nil {
		t.Fatalf("idempotent replay failed: %v", err)
	}
	if first.GetVersion() != 2 || replayed.GetVersion() != first.GetVersion() || fmt.Sprint(first.GetSiteIds()) != fmt.Sprint([]string{siteA, siteA2}) {
		t.Fatalf("idempotent write first=%+v replay=%+v", first, replayed)
	}

	readback, err := members.GetTenantMemberBusinessScope(authCtx(tokenA), &accessv1.GetTenantMemberBusinessScopeRequest{UserId: memberA})
	if err != nil {
		t.Fatal(err)
	}
	if readback.GetVersion() != first.GetVersion() || fmt.Sprint(readback.GetSiteIds()) != fmt.Sprint(first.GetSiteIds()) {
		t.Fatalf("authoritative readback=%+v write=%+v", readback, first)
	}

	// A semantically identical write with a new idempotency key is a no-op and
	// does not manufacture a new membership/scope version.
	noOp, err := members.SetTenantMemberBusinessScope(writeCtx(tokenA, "set-same"), &accessv1.SetTenantMemberBusinessScopeRequest{
		UserId: memberA, Version: readback.GetVersion(), SiteIds: []string{siteA, siteA2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if noOp.GetVersion() != readback.GetVersion() {
		t.Fatalf("same scope bumped CAS version: before=%d after=%d", readback.GetVersion(), noOp.GetVersion())
	}

	// Direct cross-tenant object injection is rejected and leaves the authoritative
	// scope/version unchanged.
	if _, err := members.SetTenantMemberBusinessScope(writeCtx(tokenA, "cross-site"), &accessv1.SetTenantMemberBusinessScopeRequest{
		UserId: memberA, Version: noOp.GetVersion(), SiteIds: []string{siteB},
	}); err == nil {
		t.Fatal("tenant A accepted tenant B site")
	}
	afterReject, err := members.GetTenantMemberBusinessScope(authCtx(tokenA), &accessv1.GetTenantMemberBusinessScopeRequest{UserId: memberA})
	if err != nil {
		t.Fatal(err)
	}
	if afterReject.GetVersion() != noOp.GetVersion() || fmt.Sprint(afterReject.GetSiteIds()) != fmt.Sprint(noOp.GetSiteIds()) {
		t.Fatalf("rejected injection mutated scope: before=%+v after=%+v", noOp, afterReject)
	}
	if _, err := members.GetTenantMemberBusinessScope(authCtx(tokenA), &accessv1.GetTenantMemberBusinessScopeRequest{UserId: memberB}); err == nil {
		t.Fatal("tenant A read tenant B member scope")
	}

	// Same CAS version, two different writes: exactly one may commit. The loser
	// must observe conflict and no partial mixture can remain.
	type raceResult struct {
		scope *accessv1.TenantMemberBusinessScopeDTO
		err   error
	}
	start := make(chan struct{})
	results := make(chan raceResult, 2)
	var wg sync.WaitGroup
	for index, ids := range [][]string{{siteA}, {siteA2}} {
		wg.Add(1)
		go func(index int, ids []string) {
			defer wg.Done()
			<-start
			scope, err := members.SetTenantMemberBusinessScope(writeCtx(tokenA, fmt.Sprintf("race-%d", index)), &accessv1.SetTenantMemberBusinessScopeRequest{
				UserId: memberA, Version: afterReject.GetVersion(), SiteIds: ids,
			})
			results <- raceResult{scope: scope, err: err}
		}(index, ids)
	}
	close(start)
	wg.Wait()
	close(results)
	successes, failures := 0, 0
	for result := range results {
		if result.err == nil {
			successes++
			if result.scope.GetVersion() != afterReject.GetVersion()+1 {
				t.Fatalf("winning CAS version=%d", result.scope.GetVersion())
			}
		} else {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("concurrent CAS outcomes success=%d failure=%d", successes, failures)
	}

	finalScope, err := members.GetTenantMemberBusinessScope(authCtx(tokenA), &accessv1.GetTenantMemberBusinessScopeRequest{UserId: memberA})
	if err != nil {
		t.Fatal(err)
	}
	if finalScope.GetVersion() != afterReject.GetVersion()+1 || len(finalScope.GetSiteIds()) != 1 || (finalScope.GetSiteIds()[0] != siteA && finalScope.GetSiteIds()[0] != siteA2) {
		t.Fatalf("concurrent write left partial/invalid scope=%+v", finalScope)
	}

	// Authorization read paths already consume biz_member_sites. Slice 2 proves
	// its authoritative readback is the same fact set; Slice 3 will apply policy intersections.
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	authorizationSites, err := store.ResolveMemberSites(context.Background(), tenantA, memberA)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(authorizationSites) != fmt.Sprint(finalScope.GetSiteIds()) {
		t.Fatalf("scope authority split: access=%v authorization=%v", finalScope.GetSiteIds(), authorizationSites)
	}

	var successAudit, failureAudit int64
	if err := db.Table("biz_audit_events").Where(
		"tenant_id = ? AND operation_id = ? AND event_type = ? AND outcome = ?",
		tenantA, "tenant.member.business_scope.set", "outcome", "success",
	).Count(&successAudit).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_audit_events").Where(
		"tenant_id = ? AND operation_id = ? AND event_type = ? AND outcome = ?",
		tenantA, "tenant.member.business_scope.set", "outcome", "failure",
	).Count(&failureAudit).Error; err != nil {
		t.Fatal(err)
	}
	if successAudit < 2 || failureAudit < 1 {
		t.Fatalf("scope audit outcomes success=%d failure=%d", successAudit, failureAudit)
	}
	var actorCount int64
	if err := db.Table("biz_audit_events").Where(
		"tenant_id = ? AND operation_id = ? AND event_type = ? AND actor_user_id = ? AND target = ? AND risk = ?",
		tenantA, "tenant.member.business_scope.set", "attempt", adminA, "user_id:"+memberA, "high",
	).Count(&actorCount).Error; err != nil {
		t.Fatal(err)
	}
	if actorCount == 0 {
		t.Fatal("scope audit did not retain operator/target/risk evidence")
	}

	_ = tokenB
}
