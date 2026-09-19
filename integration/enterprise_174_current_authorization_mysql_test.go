//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

func TestEnterprise174CurrentGrantFactsShrinkAndTenantIsolation(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	ctx := context.Background()
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}

	const (
		userID  = "enterprise-174-user"
		tenantA = "enterprise-174-a"
		tenantB = "enterprise-174-b"
	)
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenantA, TenantName: "Enterprise 174 A", UserID: userID,
		Email: "enterprise174@example.invalid", Token: "enterprise-174-token-a",
	}, []authz.PermissionKey{"tenant.member.read", "tenant.member.manage"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenantB, TenantName: "Enterprise 174 B", UserID: userID,
		Email: "enterprise174@example.invalid", Token: "enterprise-174-token-b",
	}, []authz.PermissionKey{"tenant.member.read"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE biz_tenants SET timezone=? WHERE id=?", "Asia/Taipei", tenantA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE biz_permission_grants SET scope=? WHERE tenant_id=? AND permission=?", string(accessdomain.DataScopeSites), tenantA, "tenant.member.read").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_member_sites(tenant_id,user_id,site_id) VALUES(?,?,?)", tenantA, userID, "site-174").Error; err != nil {
		t.Fatal(err)
	}

	resolver, err := accesspersistence.NewPrincipalGrantResolver(store)
	if err != nil {
		t.Fatal(err)
	}
	requestA := authz.GrantRequest{
		Principal: identity.Principal{
			Subject: "user:" + userID, TenantID: tenantA, UserID: userID,
			Roles: []string{accessdomain.TenantOwnerRoleName}, AuthMethod: accesspersistence.AuthMethodWeb, Authenticated: true,
		},
		TenantBound: true,
		Permissions: []authz.PermissionKey{"tenant.member.manage"},
	}
	grants, err := resolver.ResolveGrants(ctx, requestA)
	if err != nil || len(grants) != 1 || grants[0].Permission != "tenant.member.manage" {
		t.Fatalf("tenant A current grant missing: grants=%+v err=%v", grants, err)
	}

	requestB := requestA
	requestB.Principal.TenantID = tenantB
	requestB.Principal.Roles = []string{accessdomain.TenantOwnerRoleName}
	grants, err = resolver.ResolveGrants(ctx, requestB)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("same role name leaked tenant A grant into tenant B: %+v", grants)
	}

	snapshot, err := store.CurrentAuthorization(ctx, requestA.Principal)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.TenantID != tenantA || snapshot.Timezone != "Asia/Taipei" || snapshot.PermissionVersion == "" {
		t.Fatalf("current snapshot missing trusted tenant facts: %+v", snapshot)
	}
	var sitesPolicy accesspersistence.CurrentDataPolicy
	for _, policy := range snapshot.DataPolicies {
		if policy.Permission == "tenant.member.read" {
			sitesPolicy = policy
		}
	}
	if sitesPolicy.Scope != "sites" || len(sitesPolicy.SiteIDs) != 1 || sitesPolicy.SiteIDs[0] != "site-174" {
		t.Fatalf("current data policy not derived from grants/sites: %+v", sitesPolicy)
	}

	if err := db.Exec("DELETE FROM biz_permission_grants WHERE tenant_id=? AND permission=?", tenantA, "tenant.member.manage").Error; err != nil {
		t.Fatal(err)
	}
	grants, err = resolver.ResolveGrants(ctx, requestA)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("revoked grant remained allowed on next resolve: %+v", grants)
	}

	if err := db.Exec("INSERT INTO biz_permission_grants(tenant_id,role_id,permission,scope) VALUES(?,?,?,?)", tenantA, tenantA+":owner", "tenant.member.manage", string(accessdomain.DataScopeAll)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE biz_roles SET status=? WHERE tenant_id=? AND id=?", accessdomain.TenantRoleStatusDisabled, tenantA, tenantA+":owner").Error; err != nil {
		t.Fatal(err)
	}
	grants, err = resolver.ResolveGrants(ctx, requestA)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("disabled role remained allowed on next resolve: %+v", grants)
	}

	if err := db.Exec("UPDATE biz_roles SET status=? WHERE tenant_id=? AND id=?", accessdomain.TenantRoleStatusActive, tenantA, tenantA+":owner").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE biz_memberships SET status=? WHERE tenant_id=? AND user_id=?", accessdomain.TenantMemberStatusSuspended, tenantA, userID).Error; err != nil {
		t.Fatal(err)
	}
	grants, err = resolver.ResolveGrants(ctx, requestA)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("suspended membership remained allowed on next resolve: %+v", grants)
	}

	if err := db.Exec("UPDATE biz_memberships SET status=? WHERE tenant_id=? AND user_id=?", accessdomain.TenantMemberStatusActive, tenantA, userID).Error; err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	grants, err = resolver.ResolveGrants(ctx, requestA)
	if err == nil {
		t.Fatalf("database failure reused stale grant: %+v", grants)
	}
}

type enterprise174GrantResolver interface {
	ResolveGrants(context.Context, authz.GrantRequest) ([]authz.Grant, error)
}

type enterprise174AlwaysAllowResolver struct{}

func (enterprise174AlwaysAllowResolver) ResolveGrants(_ context.Context, request authz.GrantRequest) ([]authz.Grant, error) {
	out := make([]authz.Grant, 0, len(request.Permissions))
	for _, permission := range request.Permissions {
		out = append(out, authz.Grant{Permission: permission, RoleID: "always-allow", Scope: "all"})
	}
	return out, nil
}

func enterprise174QualifyResolver(ctx context.Context, resolver enterprise174GrantResolver, allowed, denied authz.GrantRequest) error {
	grants, err := resolver.ResolveGrants(ctx, allowed)
	if err != nil {
		return err
	}
	if len(grants) == 0 {
		return errors.New("positive authorization case was denied")
	}
	grants, err = resolver.ResolveGrants(ctx, denied)
	if err != nil {
		return err
	}
	if len(grants) != 0 {
		return errors.New("negative authorization case was allowed")
	}
	return nil
}

func TestEnterprise174QualificationRejectsAlwaysAllowResolver(t *testing.T) {
	allowed := authz.GrantRequest{
		Principal:   identity.Principal{TenantID: "tenant-a", UserID: "user-a", Authenticated: true},
		TenantBound: true,
		Permissions: []authz.PermissionKey{"tenant.member.read"},
	}
	denied := allowed
	denied.Principal.TenantID = "tenant-b"
	if err := enterprise174QualifyResolver(context.Background(), enterprise174AlwaysAllowResolver{}, allowed, denied); err == nil {
		t.Fatal("authorization qualification accepted an always-allow resolver")
	}
}
