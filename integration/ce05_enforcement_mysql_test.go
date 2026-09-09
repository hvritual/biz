//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	devicev1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func ce05RPCDenied(t *testing.T, err error, code codes.Code, reason string) {
	t.Helper()
	if status.Code(err) != code {
		t.Fatalf("code=%s expected=%s error=%v", status.Code(err), code, err)
	}
	if reason == "" {
		return
	}
	for _, d := range status.Convert(err).Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			if info.Reason != reason || info.Domain != "biz.commercial" || info.Metadata["correlation_id"] == "" || info.Metadata["operation"] == "" {
				t.Fatalf("bad reason metadata: %+v", info)
			}
			return
		}
	}
	t.Fatalf("missing commercial ErrorInfo: %v", err)
}
func ce05HTTP(t *testing.T, e *ce05Environment, method, path, token string, body []byte) (int, map[string]any) {
	t.Helper()
	r, err := http.NewRequest(method, "http://"+e.runtime.HTTPAddress()+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Idempotency-Key", ce04Random(t))
	r.Header.Set("X-Tenant-ID", e.tenantA)
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	b, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	if out == nil {
		out = map[string]any{"body": string(b)}
	}
	return response.StatusCode, out
}
func (e *ce05Environment) grant(kind commercialv1.EntitlementTarget, key string, version uint64) *commercialv1.EntitlementOverrideReceipt {
	return e.mustCreate(ce04Request(e.tenantA, ce04Random(e.t), version, kind, key, commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT))
}
func (e *ce05Environment) createDevice() *devicev1.DeviceDTO {
	e.t.Helper()
	v, err := e.devices.CreateDevice(ce04Context(e.tokenA, ce04Random(e.t)), &devicev1.CreateDeviceRequest{SiteId: e.siteA, Name: "CE05", Serial: ce04Random(e.t)})
	if err != nil {
		e.t.Fatal(err)
	}
	return v
}
func TestCE05MySQLRESTGRPCGrantAndDeny(t *testing.T) {
	e := ce05New(t)
	_, err := e.devices.ListDevices(ce04Context(e.tokenA, ""), &devicev1.ListDevicesRequest{})
	ce05RPCDenied(t, err, codes.PermissionDenied, "MODULE_NOT_ENTITLED")
	e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, "device.lifecycle", 0)
	item := e.createDevice()
	if s, _ := ce05HTTP(t, e, "GET", "/v1/devices", e.tokenA, nil); s != 200 {
		t.Fatal(s)
	}
	for _, v := range []struct {
		method, path string
		body         []byte
	}{{"GET", "/v1/devices", nil}, {"POST", "/v1/devices", []byte(`{"siteId":"` + e.siteA + `","name":"bad","serial":"bad"}`)}, {"PATCH", "/v1/devices/" + item.Id + "/transfer", []byte(`{"targetSiteId":"` + e.siteB + `","version":"1"}`)}} {
		s, out := ce05HTTP(t, e, v.method, v.path, e.tokenB, v.body)
		if s != 403 || out["code"] != "MODULE_NOT_ENTITLED" || out["correlation_id"] == "" {
			t.Fatal(s, out)
		}
	}
	_, err = e.devices.CreateDevice(ce04Context(e.tokenB, ce04Random(t)), &devicev1.CreateDeviceRequest{SiteId: e.siteA, Name: "B", Serial: ce04Random(t)})
	ce05RPCDenied(t, err, codes.PermissionDenied, "MODULE_NOT_ENTITLED")
	_, err = e.transfers.TransferDevice(ce04Context(e.tokenA, ce04Random(t)), &devicev1.TransferDeviceRequest{Id: item.Id, TargetSiteId: e.siteB, Version: item.Version})
	ce05RPCDenied(t, err, codes.PermissionDenied, "MODULE_NOT_ENTITLED")
	var count int64
	if err := e.db.Table("biz_deviceops_device").Where("tenant_id=?", e.tenantB).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("denied write persisted")
	}
	e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, "device.transfer", 1)
	moved, err := e.transfers.TransferDevice(ce04Context(e.tokenA, ce04Random(t)), &devicev1.TransferDeviceRequest{Id: item.Id, TargetSiteId: e.siteB, Version: item.Version})
	if err != nil || moved.SiteId != e.siteB {
		t.Fatal(moved, err)
	}
	deny := ce04Request(e.tenantA, ce04Random(t), 2, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, "device.lifecycle", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_DENY)
	d := e.mustCreate(deny)
	_, err = e.devices.ListDevices(ce04Context(e.tokenA, ""), &devicev1.ListDevicesRequest{})
	ce05RPCDenied(t, err, codes.PermissionDenied, "CAPABILITY_DISABLED")
	e.revoke(d.Source.Id, ce04Random(t), 3)
	if _, err = e.devices.ListDevices(ce04Context(e.tokenA, ""), &devicev1.ListDevicesRequest{}); err != nil {
		t.Fatal(err)
	}
}
func TestCE05MySQLConditionalChildOnlyWhenMoving(t *testing.T) {
	e := ce05New(t)
	e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, "device.lifecycle", 0)
	d := e.createDevice()
	renamed, err := e.devices.UpdateDevice(ce04Context(e.tokenA, ce04Random(t)), &devicev1.UpdateDeviceRequest{Id: d.Id, Name: "rename without transfer rights", Version: d.Version})
	if err != nil {
		t.Fatal(err)
	}
	same, err := e.devices.UpdateDevice(ce04Context(e.tokenA, ce04Random(t)), &devicev1.UpdateDeviceRequest{Id: d.Id, SiteId: e.siteA, Name: "same site", Version: renamed.Version})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.devices.UpdateDevice(ce04Context(e.tokenA, ce04Random(t)), &devicev1.UpdateDeviceRequest{Id: d.Id, SiteId: e.siteB, Name: "must not persist", Version: same.Version})
	ce05RPCDenied(t, err, codes.PermissionDenied, "MODULE_NOT_ENTITLED")
	unchanged, err := e.devices.GetDevice(ce04Context(e.tokenA, ""), &devicev1.GetDeviceRequest{Id: d.Id})
	if err != nil || unchanged.SiteId != e.siteA || unchanged.Version != same.Version || unchanged.Name != "same site" {
		t.Fatal(unchanged, err)
	}
	e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, "device.transfer", 1)
	s, out := ce05HTTP(t, e, "PATCH", "/v1/devices/"+d.Id, e.tokenA, []byte(`{"siteId":"`+e.siteB+`","version":"`+jsonNumber(same.Version)+`"}`))
	if s != 200 || out["siteId"] != e.siteB {
		t.Fatal(s, out)
	}
	no := ce04Request(e.tenantA, ce04Random(t), 2, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, "device.transfer", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_DENY)
	e.mustCreate(no)
	next, err := e.devices.GetDevice(ce04Context(e.tokenA, ""), &devicev1.GetDeviceRequest{Id: d.Id})
	if err != nil {
		t.Fatal(err)
	}
	s, out = ce05HTTP(t, e, "PATCH", "/v1/devices/"+d.Id, e.tokenA, []byte(`{"siteId":"`+e.siteA+`","version":"`+jsonNumber(next.Version)+`"}`))
	if s != 403 || out["code"] != "CAPABILITY_DISABLED" || out["operation"] != "site.validate_transfer_target" {
		t.Fatal(s, out)
	}
}
func jsonNumber(v uint64) string { b, _ := json.Marshal(v); return string(b) }
func TestCE05MySQLIAMAndDataScopeStillRequired(t *testing.T) {
	e := ce05New(t)
	e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", 0)
	d := e.createDevice()
	token := "limited-" + ce04Random(t)
	u := "u-" + ce04Random(t)
	role := "r-" + ce04Random(t)
	seedReader(t, e.db, e.tenantA, u, token, e.siteB, role, "site-limited", "device.read", "sites")
	_, err := e.devices.CreateDevice(ce04Context(token, ce04Random(t)), &devicev1.CreateDeviceRequest{SiteId: e.siteB, Name: "blocked by IAM", Serial: ce04Random(t)})
	ce05RPCDenied(t, err, codes.PermissionDenied, "")
	listed, err := e.devices.ListDevices(ce04Context(token, ""), &devicev1.ListDevicesRequest{})
	if err != nil || len(listed.Devices) != 0 {
		t.Fatal(listed, err)
	}
	_, err = e.devices.GetDevice(ce04Context(token, ""), &devicev1.GetDeviceRequest{Id: d.Id})
	if err == nil {
		t.Fatal("data scope leaked device")
	}
	for _, perm := range []string{"device.update", "site.read"} {
		if err := e.db.Exec("INSERT INTO biz_permission_grants(tenant_id,role_id,permission,scope) VALUES(?,?,?,'sites')", e.tenantA, role, perm).Error; err != nil {
			t.Fatal(err)
		}
	}
	_, err = e.transfers.TransferDevice(ce04Context(token, ce04Random(t)), &devicev1.TransferDeviceRequest{Id: d.Id, TargetSiteId: e.siteA, Version: d.Version})
	ce05RPCDenied(t, err, codes.PermissionDenied, "")
	s, _ := ce05HTTP(t, e, "GET", "/v1/devices", e.token, nil)
	if s != 403 {
		t.Fatal(s)
	}
	_, err = e.devices.ListDevices(ce04Context(e.token, ""), &devicev1.ListDevicesRequest{})
	ce05RPCDenied(t, err, codes.PermissionDenied, "")
	s, _ = ce05HTTP(t, e, "GET", "/v1/devices", "invalid", nil)
	if s != 401 {
		t.Fatal(s)
	}
	_, err = e.runtime.Applications.DeviceopsDeviceManagement.ListDevices(context.Background(), &devicev1.ListDevicesRequest{})
	if err == nil {
		t.Fatal("raw Application bypass")
	}
	_, err = e.runtime.Applications.DeviceopsSiteManagement.ValidateTransferTarget(context.Background(), &devicev1.ValidateTransferTargetRequest{SiteId: e.siteA})
	if err == nil {
		t.Fatal("raw child bypass")
	}
}
func TestCE05MySQLTechnicalStopExpiryAndSourceFailure(t *testing.T) {
	e := ce05New(t)
	e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "device-operations", 0)
	m, err := e.catalog.GetModule(ce04Context(e.token, ""), &commercialv1.GetModuleRequest{ModuleCode: "device-operations"})
	if err != nil {
		t.Fatal(err)
	}
	key := ce04Random(t)
	m, err = e.catalog.SetModuleSalesStatus(ce04Context(e.token, key), &commercialv1.SetModuleSalesStatusRequest{ModuleCode: m.ModuleCode, Version: m.Version, RequestId: key, Reason: "retire not revoke", SalesStatus: commercialv1.ModuleSalesStatus_MODULE_SALES_STATUS_RETIRED})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.devices.ListDevices(ce04Context(e.tokenA, ""), &devicev1.ListDevicesRequest{}); err != nil {
		t.Fatal(err)
	}
	key = ce04Random(t)
	m, err = e.catalog.SetModuleTechnicalStatus(ce04Context(e.token, key), &commercialv1.SetModuleTechnicalStatusRequest{ModuleCode: m.ModuleCode, Version: m.Version, RequestId: key, Reason: "technical stop", TechnicalStatus: commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_DISABLED})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		key := ce04Random(t)
		_, err := e.catalog.SetModuleTechnicalStatus(ce04Context(e.token, key), &commercialv1.SetModuleTechnicalStatusRequest{ModuleCode: m.ModuleCode, Version: m.Version, RequestId: key, Reason: "restore fixture", TechnicalStatus: commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY})
		if err != nil {
			t.Error(err)
		}
	})
	_, err = e.devices.ListDevices(ce04Context(e.tokenA, ""), &devicev1.ListDevicesRequest{})
	ce05RPCDenied(t, err, codes.PermissionDenied, "TECHNICAL_UNAVAILABLE")
	s, out := ce05HTTP(t, e, "GET", "/v1/devices", e.tokenA, nil)
	if s != 403 || out["code"] != "TECHNICAL_UNAVAILABLE" {
		t.Fatal(s, out)
	}
	callback := "ce05:read-failure"
	if err := e.db.Callback().Row().Before("gorm:row").Register(callback, func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "biz_commercial_entitlement_state") {
			tx.AddError(errors.New("CE05 synthetic authoritative read failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, err = e.devices.ListDevices(ce04Context(e.tokenA, ""), &devicev1.ListDevicesRequest{})
	if err := e.db.Callback().Row().Remove(callback); err != nil {
		t.Fatal(err)
	}
	ce05RPCDenied(t, err, codes.Unavailable, "ENTITLEMENT_SOURCE_UNAVAILABLE")
}
func TestCE05MySQLNoImplicitAccessManagementOrExpiredGrant(t *testing.T) {
	e := ce05New(t)
	req := ce04Request(e.tenantA, ce04Random(t), 0, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, "device.lifecycle", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT)
	req.EffectiveAt = time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
	req.ExpiresAt = time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
	e.mustCreate(req)
	_, err := e.devices.ListDevices(ce04Context(e.tokenA, ""), &devicev1.ListDevicesRequest{})
	ce05RPCDenied(t, err, codes.PermissionDenied, "MODULE_NOT_ENTITLED")
	req = ce04Request(e.tenantA, ce04Random(t), 1, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, "device.lifecycle", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT)
	req.EffectiveAt = time.Now().Add(time.Hour).UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
	e.mustCreate(req)
	_, err = e.devices.ListDevices(ce04Context(e.tokenA, ""), &devicev1.ListDevicesRequest{})
	ce05RPCDenied(t, err, codes.PermissionDenied, "MODULE_NOT_ENTITLED")
	if _, err = e.client.GetMyEntitlements(ce04Context(e.tokenA, ""), &commercialv1.GetMyEntitlementsRequest{}); err != nil {
		t.Fatal("recovery explanation unavailable", err)
	}
	if err := e.db.Exec("INSERT IGNORE INTO biz_permission_grants(tenant_id,role_id,permission,scope) SELECT tenant_id,role_id,'tenant.member.read','all' FROM biz_member_roles WHERE tenant_id=?", e.tenantA).Error; err != nil {
		t.Fatal(err)
	}
	conn, err := grpc.DialContext(context.Background(), e.runtime.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	members := accessv1.NewTenantMemberLifecycleApplicationClient(conn)
	_, err = members.ListTenantMembers(ce04Context(e.tokenA, ""), &accessv1.ListTenantMembersRequest{})
	ce05RPCDenied(t, err, codes.PermissionDenied, "MODULE_NOT_ENTITLED")
	req = ce04Request(e.tenantA, ce04Random(t), 2, commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE, "access-management", commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT)
	req.ModuleCode = "access-management"
	e.mustCreate(req)
	if _, err = members.ListTenantMembers(ce04Context(e.tokenA, ""), &accessv1.ListTenantMembersRequest{}); err != nil {
		t.Fatal(err)
	}
}
