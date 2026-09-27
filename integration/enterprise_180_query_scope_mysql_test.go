//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	devicev1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/bizruntime"
	"github.com/hvritual/biz/modules/deviceops"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"yunka.io/framework/platform"
	"yunka.io/pkg/logExt"
)

// The discovery endpoint is a fixture; cookies, authentication, authorization,
// application calls and MySQL persistence are the production implementations.
func startEffectiveScopeRuntime(t *testing.T, db *gorm.DB) (*bizruntime.Started, string) {
	t.Helper()
	var issuer *httptest.Server
	issuer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": issuer.URL, "authorization_endpoint": issuer.URL + "/authorize", "token_endpoint": issuer.URL + "/token", "jwks_uri": issuer.URL + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
	}))
	t.Cleanup(issuer.Close)
	config := deviceops.DefaultConfig()
	config.HTTPListenAddress, config.GRPCListenAddress, config.AutoMigrate = "127.0.0.1:0", "127.0.0.1:0", true
	provider, err := platform.New(platform.Options{Config: bizruntime.ConfigProvider{DeviceOps: config}, Logger: logExt.NewBaseLogger(), Databases: map[string]platform.DatabaseFactory{"primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
		return platform.BorrowedDatabase(db), nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started, err := bizruntime.BootstrapWithOptions(ctx, provider, bizruntime.Options{DeviceOps: config, WebAuth: bizruntime.WebAuthConfig{IssuerURL: issuer.URL, ClientID: "effective-scope-test", RedirectURL: "http://127.0.0.1/auth/callback", Scopes: []string{"openid"}, SessionTTL: time.Hour, FlowTTL: time.Minute}})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = started.App.Shutdown(shutdown)
	})
	return started, issuer.URL
}

func TestEnterprise180EffectiveScopeQueryEnforcement(t *testing.T) {
	db := openDB(t)
	started, issuer := startEffectiveScopeRuntime(t, db)
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenant, foreign := "e180-query-a-"+stamp, "e180-query-b-"+stamp
	admin, foreignAdmin := "e180-admin-"+stamp, "e180-admin-b-"+stamp
	token, foreignToken := "e180-admin-token-"+stamp, "e180-admin-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenant, admin, admin+"@example.invalid", token)
	seedB123TenantAdmin(t, db, foreign, foreignAdmin, foreignAdmin+"@example.invalid", foreignToken)
	seedB124RoleAdminPermissions(t, db, tenant)
	seedB124RoleAdminPermissions(t, db, foreign)
	ce05LegacyFixtureGrants(t, db, tenant, []string{"device-operations"})
	ce05LegacyFixtureGrants(t, db, foreign, []string{"device-operations"})
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	exec("INSERT INTO biz_permission_grants(tenant_id,role_id,permission,scope) VALUES(?,?,?,?)", tenant, tenant+":member-admin", "tenant.organization.manage", "all")
	reader, readerToken := "e180-reader-"+stamp, "e180-reader-token-"+stamp
	roleID := tenant + ":reader"
	seedReader(t, db, tenant, reader, readerToken, "", roleID, "policy-reader", "device.read", "all")
	siteA, siteB, siteC, foreignSite := "e180-a-"+stamp, "e180-b-"+stamp, "e180-c-"+stamp, "e180-f-"+stamp
	for _, site := range []struct{ tenant, id string }{{tenant, siteA}, {tenant, siteB}, {tenant, siteC}, {foreign, foreignSite}} {
		exec("INSERT INTO biz_deviceops_site(id,tenant_id,name,version,created_at,updated_at) VALUES(?,?,?,1,NOW(3),NOW(3))", site.id, site.tenant, site.id)
		exec("INSERT INTO biz_deviceops_device(id,tenant_id,site_id,name,serial,created_by,version,created_at,updated_at) VALUES(?,?,?,?,?,?,1,NOW(3),NOW(3))", "device-"+site.id, site.tenant, site.id, site.id, site.id, reader)
	}
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	roles := accessv1.NewTenantRolePermissionApplicationClient(conn)
	scopes := accessv1.NewTenantMemberBusinessScopeApplicationClient(conn)
	members := accessv1.NewTenantMemberLifecycleApplicationClient(conn)
	departments := accessv1.NewTenantDepartmentManagementApplicationClient(conn)
	devices := devicev1.NewDeviceApplicationClient(conn)
	auth := func(value string) context.Context {
		return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+value)
	}
	sequence := 0
	adminCtx := func(value string) context.Context {
		sequence++
		return metadata.AppendToOutgoingContext(auth(value), "idempotency-key", fmt.Sprintf("e180-query-%s-%d", stamp, sequence))
	}
	createPolicy := func(value, name string, ids ...string) *accessv1.TenantDataPolicyDTO {
		t.Helper()
		p, e := roles.CreateTenantDataPolicy(adminCtx(value), &accessv1.CreateTenantDataPolicyRequest{Name: name, SiteIds: ids})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	bind := func(roleID string, p *accessv1.TenantDataPolicyDTO) {
		t.Helper()
		r, e := roles.GetTenantRole(auth(token), &accessv1.GetTenantRoleRequest{RoleId: roleID})
		if e != nil {
			t.Fatal(e)
		}
		_, e = roles.SetTenantRoleDataPolicy(adminCtx(token), &accessv1.SetTenantRoleDataPolicyRequest{RoleId: roleID, Version: r.GetVersion(), PolicyId: p.GetId(), PolicyVersion: p.GetVersion()})
		if e != nil {
			t.Fatal(e)
		}
	}
	setMember := func(ids ...string) {
		t.Helper()
		m, e := scopes.GetTenantMemberBusinessScope(auth(token), &accessv1.GetTenantMemberBusinessScopeRequest{UserId: reader})
		if e != nil {
			t.Fatal(e)
		}
		_, e = scopes.SetTenantMemberBusinessScope(adminCtx(token), &accessv1.SetTenantMemberBusinessScopeRequest{UserId: reader, Version: m.GetVersion(), SiteIds: ids})
		if e != nil {
			t.Fatal(e)
		}
	}
	changePolicy := func(p *accessv1.TenantDataPolicyDTO, expires string, ids ...string) *accessv1.TenantDataPolicyDTO {
		t.Helper()
		v, e := roles.UpdateTenantDataPolicy(adminCtx(token), &accessv1.UpdateTenantDataPolicyRequest{PolicyId: p.GetId(), Version: p.GetVersion(), Name: p.GetName(), SiteIds: ids, ExpiresAt: expires})
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	policyA := createPolicy(token, "Query A", siteA, siteB)
	bind(roleID, policyA)
	setMember(siteA, siteB)
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := store.ResolveOrBindOIDCIdentity(context.Background(), issuer, reader, reader+"@example.invalid", true)
	if err != nil {
		t.Fatal(err)
	}
	cookie, session, err := store.CreateWebSession(context.Background(), linked, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if session.Session.ActiveTenantID != tenant {
		session, err = store.SwitchWebSessionTenant(context.Background(), cookie, tenant)
		if err != nil {
			t.Fatal(err)
		}
	}
	sessionContext, _ := json.Marshal(map[string]any{"actor_kind": string(session.Session.ActorKind), "platform_subject": "", "user_id": reader, "active_tenant_id": tenant, "context_version": session.Session.ContextVersion})
	// This exact cookie/context and gRPC credential are retained for every request.
	getHTTP := func(path string) (int, []byte) {
		t.Helper()
		req, e := http.NewRequest(http.MethodGet, "http://"+started.HTTPAddress()+path, nil)
		if e != nil {
			t.Fatal(e)
		}
		req.AddCookie(&http.Cookie{Name: "biz_session", Value: cookie})
		req.Header.Set("X-Biz-Session-Context", string(sessionContext))
		res, e := (&http.Client{Timeout: 5 * time.Second}).Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		body, e := io.ReadAll(res.Body)
		if e != nil {
			t.Fatal(e)
		}
		return res.StatusCode, body
	}
	assertList := func(want ...string) {
		t.Helper()
		sort.Strings(want)
		code, body := getHTTP("/v1/devices")
		if code != http.StatusOK {
			t.Fatalf("cookie list status=%d body=%s", code, body)
		}
		var httpResult struct {
			Devices []struct {
				SiteID string `json:"siteId"`
			}
		}
		if err := json.Unmarshal(body, &httpResult); err != nil {
			t.Fatal(err)
		}
		httpIDs := []string{}
		for _, d := range httpResult.Devices {
			httpIDs = append(httpIDs, d.SiteID)
		}
		sort.Strings(httpIDs)
		if len(want) == 0 {
			want = []string{}
		}
		if !reflect.DeepEqual(httpIDs, want) {
			t.Fatalf("cookie sites=%v want=%v body=%s", httpIDs, want, body)
		}
		rpc, e := devices.ListDevices(auth(readerToken), &devicev1.ListDevicesRequest{})
		if e != nil {
			t.Fatal(e)
		}
		rpcIDs := []string{}
		for _, d := range rpc.GetDevices() {
			rpcIDs = append(rpcIDs, d.GetSiteId())
		}
		sort.Strings(rpcIDs)
		if !reflect.DeepEqual(rpcIDs, want) {
			t.Fatalf("gRPC sites=%v want=%v", rpcIDs, want)
		}
	}
	assertDetailDenied := func(site string) {
		t.Helper()
		code, body := getHTTP("/v1/devices/device-" + site)
		if code != 400 && code != 403 && code != 404 {
			t.Fatalf("out-of-scope detail status=%d body=%s", code, body)
		}
		_, e := devices.GetDevice(auth(readerToken), &devicev1.GetDeviceRequest{Id: "device-" + site})
		c := status.Code(e)
		if c != codes.InvalidArgument && c != codes.PermissionDenied && c != codes.NotFound {
			t.Fatalf("out-of-scope gRPC detail code=%s err=%v", c, e)
		}
	}
	assertInvalidPolicyDenied := func() {
		t.Helper()
		code, body := getHTTP("/v1/devices")
		if code != 403 {
			t.Fatalf("invalid policy cookie status=%d body=%s", code, body)
		}
		_, e := devices.ListDevices(auth(readerToken), &devicev1.ListDevicesRequest{})
		if status.Code(e) != codes.PermissionDenied {
			t.Fatalf("invalid policy gRPC err=%v", e)
		}
	}

	assertList(siteA, siteB)
	assertDetailDenied(siteC)
	assertDetailDenied(foreignSite)
	// A wider/unreferenced ALL role cannot bypass an existing applicable policy.
	broadRole := tenant + ":broad"
	seedReader(t, db, tenant, reader, readerToken, "", broadRole, "broad-reader", "device.read", "all")
	assertList(siteA, siteB)
	policyB := createPolicy(token, "Query B", siteB, siteC)
	bind(broadRole, policyB)
	assertList(siteB) // policies must intersect, not union.
	assertDetailDenied(siteA)

	deptA, e := departments.CreateTenantDepartment(adminCtx(token), &accessv1.CreateTenantDepartmentRequest{Name: "Scope-neutral department A", LeaderUserId: reader})
	if e != nil {
		t.Fatal(e)
	}
	member, e := members.GetTenantMember(auth(token), &accessv1.GetTenantMemberRequest{UserId: reader})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = members.UpdateTenantMemberProfile(adminCtx(token), &accessv1.UpdateTenantMemberProfileRequest{UserId: reader, Version: member.GetVersion(), DepartmentId: deptA.GetDepartmentId(), Name: "Moved reader"}); e != nil {
		t.Fatal(e)
	}
	assertList(siteB) // joining/leading a department creates no resource grant.

	deptB, e := departments.CreateTenantDepartment(adminCtx(token), &accessv1.CreateTenantDepartmentRequest{Name: "Scope-neutral department B", LeaderUserId: admin})
	if e != nil {
		t.Fatal(e)
	}
	member, e = members.GetTenantMember(auth(token), &accessv1.GetTenantMemberRequest{UserId: reader})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = members.UpdateTenantMemberProfile(adminCtx(token), &accessv1.UpdateTenantMemberProfileRequest{UserId: reader, Version: member.GetVersion(), DepartmentId: deptB.GetDepartmentId(), Name: "Cross-department reader"}); e != nil {
		t.Fatal(e)
	}
	assertList(siteB) // moving between departments must not expand effective scope.

	deptB, e = departments.DisableTenantDepartment(adminCtx(token), &accessv1.DisableTenantDepartmentRequest{DepartmentId: deptB.GetDepartmentId(), Version: deptB.GetVersion()})
	if e != nil {
		t.Fatal(e)
	}
	if deptB.GetStatus() != accessv1.TenantDepartmentStatus_TENANT_DEPARTMENT_STATUS_DISABLED {
		t.Fatalf("disabled department=%+v", deptB)
	}
	assertList(siteB) // department disablement is organization state, not business-data authority.

	policyA = changePolicy(policyA, "", siteA)
	assertList() // first request after committed policy contraction, old cookie.
	assertDetailDenied(siteB)
	policyA = changePolicy(policyA, "", siteA, siteB)
	policyB = changePolicy(policyB, "", siteA, siteB, siteC)
	assertList(siteA, siteB)
	setMember(siteB)
	assertList(siteB)
	assertDetailDenied(siteA)

	// SELF adds the ownership predicate inside, not outside, the policy ceiling.
	if _, e = roles.RevokeTenantRoleMember(adminCtx(token), &accessv1.RevokeTenantRoleMemberRequest{RoleId: broadRole, UserId: reader}); e != nil {
		t.Fatal(e)
	}
	role, e := roles.GetTenantRole(auth(token), &accessv1.GetTenantRoleRequest{RoleId: roleID})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = roles.SetTenantRolePermissions(adminCtx(token), &accessv1.SetTenantRolePermissionsRequest{RoleId: roleID, Version: role.GetVersion(), Permissions: []*accessv1.PermissionGrantInput{{Permission: "device.read", Scope: accessv1.DataScope_DATA_SCOPE_SELF}}}); e != nil {
		t.Fatal(e)
	}
	assertList(siteB)
	assertDetailDenied(siteA) // reader created A but is not allowed its site.

	policyA = changePolicy(policyA, time.Now().Add(-time.Minute).UTC().Format(time.RFC3339), siteA, siteB)
	assertInvalidPolicyDenied()
	policyA = changePolicy(policyA, "", siteA, siteB)
	assertList(siteB)
	foreignPolicy := createPolicy(foreignToken, "Foreign policy", foreignSite)
	for _, invalid := range []string{"missing-" + stamp, foreignPolicy.GetId()} {
		// Adversarial persisted references must not be rescued by stale sessions.
		exec("UPDATE biz_roles SET data_policy_id=? WHERE tenant_id=? AND id=?", invalid, tenant, roleID)
		assertInvalidPolicyDenied()
	}
	exec("UPDATE biz_roles SET data_policy_id=? WHERE tenant_id=? AND id=?", policyA.GetId(), tenant, roleID)
	assertList(siteB)
	exec("DELETE FROM biz_deviceops_site WHERE tenant_id=? AND id=?", tenant, siteB)
	assertList() // stale member/policy site links are not current assignability.
	assertDetailDenied(siteB)
	if _, e = roles.RevokeTenantDataPolicy(adminCtx(token), &accessv1.RevokeTenantDataPolicyRequest{PolicyId: policyA.GetId(), Version: policyA.GetVersion()}); e != nil {
		t.Fatal(e)
	}
	assertInvalidPolicyDenied()
}
