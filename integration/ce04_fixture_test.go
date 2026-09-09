//go:build integration

package integration

import (
 "context"
 "crypto/rand"
 "encoding/hex"
 "fmt"
 "testing"
 "time"

 commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
 accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
 "github.com/hvritual/biz/internal/bizruntime"
 "github.com/hvritual/biz/modules/deviceops"
 "google.golang.org/grpc"
 "google.golang.org/grpc/credentials/insecure"
 "google.golang.org/grpc/metadata"
 "gorm.io/gorm"
 "yunka.io/framework/platform"
 "yunka.io/gateway/authz"
 "yunka.io/pkg/logExt"
)

// All identities and credentials below are synthetic, generated inside the
// disposable qualification database. No production credential is used.
type ce04Environment struct {
 t *testing.T
 db *gorm.DB
 runtime *bizruntime.Started
 client commercialv1.EntitlementManagementApplicationClient
 catalog commercialv1.ModuleCatalogApplicationClient
 token, tenantA, tenantB, tokenA, tokenB string
}
func ce04Random(t *testing.T)string{t.Helper();var b [16]byte;if _,err:=rand.Read(b[:]);err!=nil{t.Fatal(err)};return hex.EncodeToString(b[:])}
func ce04Context(token,key string)context.Context{ctx:=metadata.AppendToOutgoingContext(context.Background(),"authorization","Bearer "+token);if key!=""{ctx=metadata.AppendToOutgoingContext(ctx,"idempotency-key",key)};return ctx}
func ce04NewEnvironment(t *testing.T,prefix string)*ce04Environment{
 t.Helper();if prefix==""{prefix=fmt.Sprintf("ce04-%d",time.Now().UnixNano())}
 db:=openDB(t);sqlDB,err:=db.DB();if err!=nil{t.Fatal(err)};t.Cleanup(func(){_ = sqlDB.Close()})
 cfg:=deviceops.DefaultConfig();cfg.HTTPListenAddress="127.0.0.1:0";cfg.GRPCListenAddress="127.0.0.1:0";cfg.AutoMigrate=true
 provider,err:=platform.New(platform.Options{Config:bizruntime.ConfigProvider{DeviceOps:cfg},Logger:logExt.NewBaseLogger(),Databases:map[string]platform.DatabaseFactory{"primary":platform.DatabaseFactoryFunc(func(context.Context,string)(platform.DatabaseResource,error){return platform.BorrowedDatabase(db),nil})}});if err!=nil{t.Fatal(err)}
 token:=ce04Random(t)
 rt,err:=bizruntime.BootstrapWithOptions(context.Background(),provider,bizruntime.Options{DeviceOps:cfg,PlatformBootstrap:bizruntime.PlatformBootstrap{Subject:prefix+"-platform",Token:token,Permissions:[]authz.PermissionKey{"platform.entitlement.manage","platform.entitlement.read","platform.module.manage","platform.module.read","platform.module.technical.manage","platform.tenant.read"}}});if err!=nil{t.Fatal(err)}
 t.Cleanup(func(){ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_ = rt.App.Shutdown(ctx)})
 ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel()
 conn,err:=grpc.DialContext(ctx,rt.GRPCAddress(),grpc.WithTransportCredentials(insecure.NewCredentials()),grpc.WithBlock());if err!=nil{t.Fatal(err)};t.Cleanup(func(){_ = conn.Close()})
 e:=&ce04Environment{t:t,db:db,runtime:rt,client:commercialv1.NewEntitlementManagementApplicationClient(conn),catalog:commercialv1.NewModuleCatalogApplicationClient(conn),token:token,tenantA:prefix+"-a",tenantB:prefix+"-b",tokenA:ce04Random(t),tokenB:ce04Random(t)}
 store,err:=accesspersistence.New(db);if err!=nil{t.Fatal(err)}
 for _,v:=range []struct{tenant,token string}{{e.tenantA,e.tokenA},{e.tenantB,e.tokenB}}{
  if err:=store.Bootstrap(context.Background(),accesspersistence.Bootstrap{TenantID:v.tenant,TenantName:v.tenant,UserID:v.tenant+"-user",Email:v.tenant+"@example.invalid",Token:v.token},[]authz.PermissionKey{"tenant.entitlement.read"});err!=nil{t.Fatal(err)}
 }
 for _,code:=range []string{"access-management","device-operations"}{
  m,err:=e.catalog.GetModule(ce04Context(token,""),&commercialv1.GetModuleRequest{ModuleCode:code})
  if err!=nil{key:=ce04Random(t);m,err=e.catalog.CreateModule(ce04Context(token,key),&commercialv1.CreateModuleRequest{RequestId:key,ModuleCode:code,Name:code,Reason:"CE04 qualification catalog"})}
  if err!=nil{t.Fatal(err)}
  if m.TechnicalStatus!=commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY{key:=ce04Random(t);_,err=e.catalog.SetModuleTechnicalStatus(ce04Context(token,key),&commercialv1.SetModuleTechnicalStatusRequest{RequestId:key,ModuleCode:code,TechnicalStatus:commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY,Version:m.Version,Reason:"CE04 restore qualification fixture"});if err!=nil{t.Fatal(err)}}
 }
 return e
}
func ce04Request(tenant,id string,version uint64,target commercialv1.EntitlementTarget,key string,effect commercialv1.EntitlementEffect)*commercialv1.CreateEntitlementOverrideRequest{return &commercialv1.CreateEntitlementOverrideRequest{TenantId:tenant,RequestId:id,ExpectedVersion:version,ModuleCode:"device-operations",Target:target,Key:key,Effect:effect,Reason:"CE04 synthetic internal reason"}}
func(e *ce04Environment) create(req *commercialv1.CreateEntitlementOverrideRequest)(*commercialv1.EntitlementOverrideReceipt,error){ctx,cancel:=context.WithTimeout(ce04Context(e.token,ce04Random(e.t)),10*time.Second);defer cancel();return e.client.CreateEntitlementOverride(ctx,req)}
func(e *ce04Environment) mustCreate(req *commercialv1.CreateEntitlementOverrideRequest)*commercialv1.EntitlementOverrideReceipt{e.t.Helper();r,err:=e.create(req);if err!=nil{e.t.Fatal(err)};return r}
func(e *ce04Environment) explain(tenant string)*commercialv1.EntitlementView{e.t.Helper();r,err:=e.client.ExplainEntitlements(ce04Context(e.token,""),&commercialv1.ExplainEntitlementsRequest{TenantId:tenant,CapabilityCodes:[]string{"unknown.capability"}});if err!=nil{e.t.Fatal(err)};return r}
func ce04Decision(t *testing.T,r *commercialv1.EntitlementView,kind,key,action string)*commercialv1.EntitlementDecisionDTO{t.Helper();for _,d:=range r.Decisions{if d.Kind==kind&&d.Key==key&&d.FieldAction==action{return d}};t.Fatalf("missing %s/%s/%s",kind,key,action);return nil}
func(e *ce04Environment) revoke(id,key string,version uint64)*commercialv1.EntitlementOverrideReceipt{e.t.Helper();r,err:=e.client.RevokeEntitlementOverride(ce04Context(e.token,ce04Random(e.t)),&commercialv1.RevokeEntitlementOverrideRequest{TenantId:e.tenantA,Id:id,RequestId:key,ExpectedVersion:version,Reason:"CE04 compensate and retain history"});if err!=nil{e.t.Fatal(err)};return r}
