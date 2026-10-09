//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/commercial/modulecatalog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"yunka.io/gateway/authz"
)

// ce293AdmittedAccessFixture is an isolated, pre-admitted module prerequisite,
// just like seedB122DefaultSubscription. The injected verifier record is NOT
// evidence that this test has run, a sales qualification, or a release approval.
// Tenant entitlements and all journey mutations below still use real APIs.
func ce293AdmittedAccessFixture(t *testing.T, db *gorm.DB, catalog commercialv1.ModuleCatalogApplicationClient, token string) {
	t.Helper()
	const code = "access-management"
	ctx := func() context.Context { return ce04Context(token, ce04Random(t)) }
	module, err := catalog.GetModule(ctx(), &commercialv1.GetModuleRequest{ModuleCode: code})
	if status.Code(err) == codes.NotFound {
		module, err = catalog.CreateModule(ctx(), &commercialv1.CreateModuleRequest{RequestId: ce04Random(t), ModuleCode: code, Name: "Access acceptance fixture", Reason: "isolated pre-admitted module fixture"})
	}
	if err != nil {
		t.Fatal(err)
	}
	if module.TechnicalStatus != commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY {
		module, err = catalog.SetModuleTechnicalStatus(ctx(), &commercialv1.SetModuleTechnicalStatusRequest{RequestId: ce04Random(t), ModuleCode: code, Version: module.Version, TechnicalStatus: commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY, Reason: "isolated fixture prerequisite"})
		if err != nil {
			t.Fatal(err)
		}
	}
	store, err := modulecatalog.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	service, err := modulecatalog.NewService(store, modulecatalog.ProductionRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if err = service.RecordRuntimeVerification(ce04RuntimeVerifier(), modulecatalog.RuntimeVerificationCommand{ModuleCode: code, ModuleVersion: module.Version, EvidenceDigest: strings.Repeat("a", 64), SourceTree: strings.Repeat("b", 64)}); err != nil {
		t.Fatal(err)
	}
	if module.SalesStatus != commercialv1.ModuleSalesStatus_MODULE_SALES_STATUS_SELLABLE {
		_, err = catalog.SetModuleSalesStatus(ctx(), &commercialv1.SetModuleSalesStatusRequest{RequestId: ce04Random(t), ModuleCode: code, Version: module.Version, SalesStatus: commercialv1.ModuleSalesStatus_MODULE_SALES_STATUS_SELLABLE, Reason: "isolated fixture, not production admission"})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func ce293IAMState(t *testing.T, db *gorm.DB, tenant string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, table := range []string{"biz_memberships", "biz_roles", "biz_member_roles", "biz_permission_grants"} {
		var rows []map[string]any
		if err := db.Table(table).Where("tenant_id=?", tenant).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		values := []string{}
		for _, row := range rows {
			value, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			values = append(values, string(value))
		}
		sortStrings(values)
		value, err := json.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		out[table] = string(value)
	}
	return out
}

func TestCE293AccessModuleFirstSubscriptionRoleJourney(t *testing.T) {
	e := ce09OnDB(t, ce08FreshFixtureDB(t), "ce293-platform-"+ce04Random(t))
	ce293AdmittedAccessFixture(t, e.db, e.catalog, e.token)
	store, err := accesspersistence.New(e.db)
	if err != nil {
		t.Fatal(err)
	}
	tenantA := ce340NoSubscriptionTenant(t, e)
	tenantB := ce340NoSubscriptionTenant(t, e)
	adminA, adminB, memberToken := ce04Random(t), ce04Random(t), ce04Random(t)
	memberID := "ce293-member-" + ce04Random(t)
	for _, actor := range []struct{ tenant, token string }{{tenantA, adminA}, {tenantB, adminB}} {
		if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{TenantID: actor.tenant, TenantName: "Access sample", UserID: actor.tenant + "-admin", Email: actor.tenant + "@example.invalid", Token: actor.token}, []authz.PermissionKey{"tenant.member.read", "tenant.member.manage", "tenant.role.read", "tenant.role.manage", "tenant.entitlement.read"}); err != nil {
			t.Fatal(err)
		}
	}
	seedB124PlainMember(t, e.db, tenantA, memberID, memberID+"@example.invalid", memberToken)
	connection, err := grpc.DialContext(context.Background(), e.grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	roles := accessv1.NewTenantRolePermissionApplicationClient(connection)
	countRole := func(tenant, name string) int64 {
		t.Helper()
		var n int64
		if err := e.db.Table("biz_roles").Where("tenant_id=? AND name=?", tenant, name).Count(&n).Error; err != nil {
			t.Fatal(err)
		}
		return n
	}
	denyCreate := func(token, reason string) {
		t.Helper()
		before := ce293IAMState(t, e.db, tenantA)
		name := "denied-" + ce04Random(t)
		_, err := roles.CreateTenantRole(ce04Context(token, ce04Random(t)), &accessv1.CreateTenantRoleRequest{Name: name})
		ce05RPCDenied(t, err, codes.PermissionDenied, reason)
		ce09EqualState(t, before, ce293IAMState(t, e.db, tenantA))
		if countRole(tenantA, name) != 0 {
			t.Fatal("denied role persisted")
		}
	}
	// A2: neither right, then IAM only. Both must refuse the real write.
	denyCreate(memberToken, "")
	denyCreate(adminA, "MODULE_NOT_ENTITLED")
	if got := memberListStatusB124(t, e.base, adminA); got != http.StatusForbidden {
		t.Fatalf("unentitled HTTP member read=%d", got)
	}
	for _, tenant := range []string{tenantA, tenantB} {
		_, err := e.subscriptions.GetTenantSubscription(e.ctx(), &commercialv1.GetTenantSubscriptionRequest{TenantId: tenant})
		ce09Code(t, err, codes.NotFound)
	}
	terms := &commercialv1.PlanTerms{Modules: []*commercialv1.PlanModule{{ModuleCode: "access-management", CapabilityCodes: []string{"tenant.member.lifecycle", "tenant.role.permission"}, Quotas: []*commercialv1.PlanQuota{{Key: "tenant.members", Value: 100}}, Fields: []*commercialv1.PlanField{{Key: "member.profile", Action: "read", Mode: "allow"}}}}, SalesScope: []string{"ce09"}, ValidityMode: "fixed_days", ValidityDays: 30}
	target := e.plan(terms)
	var firstReceipt *commercialv1.SubscriptionChangeReceiptDTO
	for _, tenant := range []string{tenantA, tenantB} {
		e.tenant = tenant
		before := ce293IAMState(t, e.db, tenant)
		preview := ce340InitialPreview(t, e, target)
		command := ce340InitialConfirm(preview, "ce293-confirm-"+ce04Random(t))
		receipt, err := e.confirm(command)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.Status != "APPLIED" || receipt.After == nil {
			t.Fatalf("first activation receipt=%+v", receipt)
		}
		recovered, err := e.confirm(command)
		if err != nil || !proto.Equal(receipt, recovered) {
			t.Fatalf("same confirmation did not recover original receipt: %v", err)
		}
		ce09EqualState(t, before, ce293IAMState(t, e.db, tenant))
		subscription := e.getSubscription(tenant)
		if subscription.PlanCode != target.PlanCode || subscription.PlanVersion != target.Version || subscription.SubscriptionId != receipt.After.SubscriptionId {
			t.Fatal("exact subscription readback mismatch")
		}
		var n int64
		if err := e.db.Table("biz_commercial_subscriptions").Where("tenant_id=?", tenant).Count(&n).Error; err != nil || n != 1 {
			t.Fatalf("subscription count=%d error=%v", n, err)
		}
		for _, key := range []string{"tenant.member.lifecycle", "tenant.role.permission"} {
			if !ce04Decision(t, e.view(), "capability", key, "").Allowed {
				t.Fatalf("missing Access capability %s", key)
			}
		}
		if tenant == tenantA {
			firstReceipt = receipt
		}
	}
	// A2: entitlement only still cannot grant IAM to the plain member.
	denyCreate(memberToken, "")
	role, err := roles.CreateTenantRole(ce04Context(adminA, ce04Random(t)), &accessv1.CreateTenantRoleRequest{Name: "operator-" + ce04Random(t)})
	if err != nil {
		t.Fatal(err)
	}
	role, err = roles.SetTenantRolePermissions(ce04Context(adminA, ce04Random(t)), &accessv1.SetTenantRolePermissionsRequest{RoleId: role.Id, Version: role.Version, Permissions: []*accessv1.PermissionGrantInput{{Permission: "tenant.member.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL}, {Permission: "tenant.role.read", Scope: accessv1.DataScope_DATA_SCOPE_ALL}, {Permission: "tenant.role.manage", Scope: accessv1.DataScope_DATA_SCOPE_ALL}}})
	if err != nil {
		t.Fatal(err)
	}
	role, err = roles.AssignTenantRoleMember(ce04Context(adminA, ce04Random(t)), &accessv1.AssignTenantRoleMemberRequest{RoleId: role.Id, UserId: memberID})
	if err != nil {
		t.Fatal(err)
	}
	if got := memberListStatusB124(t, e.base, memberToken); got != http.StatusOK {
		t.Fatalf("authorized HTTP read=%d", got)
	}
	// A3/A5: the newly-authorized member writes; replay returns the same object.
	key := "ce293-role-recovery-" + ce04Random(t)
	command := &accessv1.CreateTenantRoleRequest{Name: "created-by-member-" + ce04Random(t)}
	created, err := roles.CreateTenantRole(ce04Context(memberToken, key), command)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := roles.CreateTenantRole(ce04Context(memberToken, key), command)
	if err != nil || !proto.Equal(created, replayed) || countRole(tenantA, command.Name) != 1 {
		t.Fatalf("role replay duplicated or changed result: %v", err)
	}
	// A4: even an entitled, authorized B admin cannot address A's role.
	_, err = roles.GetTenantRole(ce04Context(adminB, ""), &accessv1.GetTenantRoleRequest{RoleId: created.Id})
	ce09Code(t, err, codes.NotFound)
	beforeA, beforeB := ce293IAMState(t, e.db, tenantA), ce293IAMState(t, e.db, tenantB)
	_, err = roles.UpdateTenantRole(ce04Context(adminB, ce04Random(t)), &accessv1.UpdateTenantRoleRequest{RoleId: created.Id, Version: created.Version, Name: "cross-tenant-forbidden"})
	ce09Code(t, err, codes.NotFound)
	ce09EqualState(t, beforeA, ce293IAMState(t, e.db, tenantA))
	ce09EqualState(t, beforeB, ce293IAMState(t, e.db, tenantB))
	// The existing object invariant must reject disabling a member-bound role.
	_, err = roles.DisableTenantRole(ce04Context(adminA, ce04Random(t)), &accessv1.DisableTenantRoleRequest{RoleId: role.Id, Version: role.Version})
	if err == nil {
		t.Fatal("member-bound role disabled")
	}
	ce09EqualState(t, beforeA, ce293IAMState(t, e.db, tenantA))
	_, err = roles.RevokeTenantRoleMember(ce04Context(adminA, ce04Random(t)), &accessv1.RevokeTenantRoleMemberRequest{RoleId: role.Id, UserId: memberID})
	if err != nil {
		t.Fatal(err)
	}
	if got := memberListStatusB124(t, e.base, memberToken); got != http.StatusForbidden {
		t.Fatalf("old token after revoke status=%d", got)
	}
	denyCreate(memberToken, "")
	if e.getSubscription(tenantA).SubscriptionId != firstReceipt.After.SubscriptionId {
		t.Fatal("IAM revocation changed commercial subscription")
	}
	t.Log("CE293_ACCESS_JOURNEY: exact INITIAL; no implicit IAM; 2x2 write matrix; actual role grant/read/write; idempotent recovery; cross-tenant and object-state rejection; old-token revocation PASS")
}

// The two entries are separate retry fixtures: retrying a browser assertion
// must never treat an already-activated tenant as the initial clean state.
type ce293AccessBrowserFixture struct {
	TenantA        string `json:"tenant_a"`
	TenantB        string `json:"tenant_b"`
	AdminEmail     string `json:"admin_email"`
	AdminPassword  string `json:"admin_password"`
	MemberID       string `json:"member_id"`
	MemberEmail    string `json:"member_email"`
	MemberPassword string `json:"member_password"`
}

func ce293SeedAccessBrowserFixtures(t *testing.T, db *gorm.DB, store *accesspersistence.Store, address, token string) []ce293AccessBrowserFixture {
	t.Helper()
	connection, err := grpc.DialContext(context.Background(), address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	ce293AdmittedAccessFixture(t, db, commercialv1.NewModuleCatalogApplicationClient(connection), token)
	fixtures := []ce293AccessBrowserFixture{}
	for attempt := 0; attempt < 2; attempt++ {
		f := ce293AccessBrowserFixture{TenantA: seedCE13NoSubscriptionTenant(t, db), TenantB: seedCE13NoSubscriptionTenant(t, db), AdminEmail: "ce293-admin-" + ce04Random(t) + "@example.invalid", AdminPassword: "CE293-Admin-Fixture-2026!", MemberID: "ce293-member-" + ce04Random(t), MemberPassword: "CE293-Member-Fixture-2026!"}
		f.MemberEmail = f.MemberID + "@example.invalid"
		adminID := "ce293-admin-" + ce04Random(t)
		for _, tenant := range []string{f.TenantA, f.TenantB} {
			if err := store.Bootstrap(context.Background(), accesspersistence.Bootstrap{TenantID: tenant, TenantName: "Access isolated sample", UserID: adminID, Email: f.AdminEmail, Token: "setup-" + tenant}, []authz.PermissionKey{"tenant.member.read", "tenant.member.manage", "tenant.role.read", "tenant.role.manage", "tenant.entitlement.read"}); err != nil {
				t.Fatal(err)
			}
			for _, table := range []string{"biz_commercial_subscriptions", "biz_commercial_entitlement_sources"} {
				var count int64
				if err := db.Table(table).Where("tenant_id=?", tenant).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("browser fixture unexpectedly entitled: %s count=%d error=%v", table, count, err)
				}
			}
		}
		if err := store.SetUserPassword(context.Background(), adminID, f.AdminPassword); err != nil {
			t.Fatal(err)
		}
		if err := store.BootstrapGlobalUser(context.Background(), accesspersistence.GlobalUserBootstrap{ID: f.MemberID, Email: f.MemberEmail}); err != nil {
			t.Fatal(err)
		}
		// Membership is an explicit initial fixture, with no role or grants.
		if err := db.Exec("INSERT INTO biz_memberships (tenant_id,user_id,status,version,created_at,updated_at) VALUES (?,?,?,1,NOW(3),NOW(3))", f.TenantA, f.MemberID, "active").Error; err != nil {
			t.Fatal(err)
		}
		if err := store.SetUserPassword(context.Background(), f.MemberID, f.MemberPassword); err != nil {
			t.Fatal(err)
		}
		fixtures = append(fixtures, f)
	}
	return fixtures
}
