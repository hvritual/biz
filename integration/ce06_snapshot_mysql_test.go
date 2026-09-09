//go:build integration

package integration
import(
 "context";"encoding/json";"errors";"strings";"testing";"time"
 commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
 devicev1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
 accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
 "github.com/hvritual/biz/internal/commercial/domain/entitlement"
 "github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
 commercialpersistence "github.com/hvritual/biz/internal/commercial/infrastructure/persistence"
 "github.com/hvritual/biz/internal/commercial/modulecatalog"
 "github.com/hvritual/biz/internal/commercial/ports"
 "google.golang.org/grpc/codes";"gorm.io/gorm"
 "yunka.io/framework/core/identity";"yunka.io/framework/execution";"yunka.io/framework/requestscope"
)
func ce06My(t *testing.T,e *ce05Environment,token string)*commercialv1.EntitlementView{
 t.Helper();v,err:=e.client.GetMyEntitlements(ce04Context(token,""),&commercialv1.GetMyEntitlementsRequest{});if err!=nil{t.Fatal(err)};return v
}
func ce06Store(t *testing.T,e *ce05Environment,cache ports.SnapshotCache)(*commercialpersistence.SnapshotStore,context.Context){
 t.Helper();store,err:=modulecatalog.NewStore(e.db);if err!=nil{t.Fatal(err)};catalog,err:=modulecatalog.NewService(store,modulecatalog.ProductionRegistry());if err!=nil{t.Fatal(err)}
 snapshots,err:=commercialpersistence.NewSnapshotStore(e.db,catalog,cache);if err!=nil{t.Fatal(err)};access,err:=accesspersistence.New(e.db);if err!=nil{t.Fatal(err)};p,err:=access.Authenticate(context.Background(),e.tokenA);if err!=nil{t.Fatal(err)}
 return snapshots,identity.WithPrincipal(context.Background(),p)
}
func TestCE06MySQLSnapshotsAndPermissionAssociation(t *testing.T){
 e:=ce05New(t);zero:=ce06My(t,e,e.tokenA)
 if zero.EntitlementVersion==0||zero.CatalogRevision==0||!strings.HasPrefix(zero.PermissionVersion,"sha256:")||zero.PermissionSubject==""{t.Fatal(zero)}
 g:=e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY,"device.lifecycle",0)
 first:=ce06My(t,e,e.tokenA);again:=ce06My(t,e,e.tokenA)
 if first.EntitlementVersion<=zero.EntitlementVersion||first.EntitlementVersion!=again.EntitlementVersion||first.SourceVersion!=1{t.Fatal(first,again)}
 if !ce04Decision(t,first,"capability","device.lifecycle","").Allowed{t.Fatal(first)}
 if err:=e.db.Exec("UPDATE biz_permission_grants SET scope='self' WHERE tenant_id=? AND permission='device.read'",e.tenantA).Error;err!=nil{t.Fatal(err)}
 changed:=ce06My(t,e,e.tokenA);if changed.PermissionVersion==first.PermissionVersion||changed.EntitlementVersion!=first.EntitlementVersion{t.Fatal("IAM/commercial versions conflated",changed)}
 other:=ce06My(t,e,e.tokenB);if other.PermissionVersion==changed.PermissionVersion||other.PermissionSubject==changed.PermissionSubject{t.Fatal("principal view leaked")}
 platform:=e.explain(e.tenantA);if platform.PermissionVersion!=""||platform.PermissionSubject!=""{t.Fatal("platform explanation forged member version")}
 e.revoke(g.Source.Id,ce04Random(t),1);denied:=ce06My(t,e,e.tokenA)
 if denied.EntitlementVersion<=first.EntitlementVersion||denied.SourceVersion!=2||ce04Decision(t,denied,"capability","device.lifecycle","").Allowed{t.Fatal(denied)}
 var n int64;if err:=e.db.Table("biz_commercial_entitlement_snapshots").Where("tenant_id=?",e.tenantA).Count(&n).Error;err!=nil||n<3{t.Fatal(n,err)}
 // Existing membership revocation is still evaluated on the next real request.
 if err:=e.db.Exec("UPDATE biz_memberships SET status='suspended',version=version+1 WHERE tenant_id=?",e.tenantA).Error;err!=nil{t.Fatal(err)}
 _,err:=e.devices.ListDevices(ce04Context(e.tokenA,""),&devicev1.ListDevicesRequest{});if err==nil{t.Fatal("membership revocation bypass")}
}
type ce06BadCache struct{b []byte;broken bool}
func(c *ce06BadCache)Get(context.Context,string)([]byte,error){if c.broken{return nil,errors.New("cache down")};return c.b,nil}
func(c *ce06BadCache)Put(context.Context,string,[]byte)error{return errors.New("cache write down")}
func TestCE06MySQLStaleCacheAndCrossInstance(t *testing.T){
 e:=ce05New(t);other:=ce05New(t);g:=e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE,"device-operations",0);first:=ce06My(t,e,e.tokenA)
 cache:=&ce06BadCache{};store,ctx:=ce06Store(t,e,cache);old,err:=store.ReadSnapshot(ctx,e.tenantA,nil);if err!=nil{t.Fatal(err)};cache.b,_=json.Marshal(old)
 // This runtime has its own cache and no notification channel.
 _,err=other.client.RevokeEntitlementOverride(ce04Context(other.token,ce04Random(t)),&commercialv1.RevokeEntitlementOverrideRequest{TenantId:e.tenantA,Id:g.Source.Id,ExpectedVersion:1,RequestId:ce04Random(t),Reason:"CE06 independent instance revoke"});if err!=nil{t.Fatal(err)}
 _,err=e.devices.ListDevices(ce04Context(e.tokenA,""),&devicev1.ListDevicesRequest{});ce05RPCDenied(t,err,codes.PermissionDenied,"MODULE_NOT_ENTITLED")
 for _,broken:=range []bool{false,true}{cache.broken=broken;r,err:=store.ReadSnapshot(ctx,e.tenantA,nil);if err!=nil{t.Fatal(err)};if r.EntitlementVersion<=first.EntitlementVersion{t.Fatal(r)};for _,d:=range r.Decisions{if d.Kind==entitlement.Capability&&d.Key=="device.lifecycle"&&d.Allowed{t.Fatal("stale cache grant")}}}
 direct,_:=ce06Store(t,e,nil);if _,err:=direct.ReadSnapshot(ctx,e.tenantA,nil);err!=nil{t.Fatal("cache-disabled recovery",err)}
 // Fault injection affects only this disposable tenant, not production data.
 if err:=e.db.Exec("DELETE FROM biz_commercial_entitlement_state WHERE tenant_id=?",e.tenantA).Error;err!=nil{t.Fatal(err)}
 _,err=store.ReadSnapshot(ctx,e.tenantA,nil);if !errors.Is(err,consistency.ErrStateLost){t.Fatal("lost version did not fail closed",err)}
 status,out:=ce05HTTP(t,e,"GET","/v1/devices",e.tokenA,nil);if status!=503||out["code"]!="ENTITLEMENT_AUTHORITY_STATE_LOST"{t.Fatal(status,out)}
}
func ce06WaitDB(t *testing.T,db *gorm.DB,target time.Time){
 t.Helper();deadline:=time.Now().Add(5*time.Second);for time.Now().Before(deadline){at,err:=consistency.Now(db);if err!=nil{t.Fatal(err)};if !at.Before(target){return};time.Sleep(10*time.Millisecond)};t.Fatal("database boundary not reached")
}
func TestCE06MySQLTimeTransitionsWithoutScheduler(t *testing.T){
 e:=ce05New(t);at,err:=consistency.Now(e.db);if err!=nil{t.Fatal(err)};start:=at.Add(400*time.Millisecond);end:=at.Add(1400*time.Millisecond)
 req:=ce04Request(e.tenantA,ce04Random(t),0,commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE,"device-operations",commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT)
 req.EffectiveAt=start.Format(time.RFC3339Nano);req.ExpiresAt=end.Format(time.RFC3339Nano);e.mustCreate(req)
 scheduled:=ce06My(t,e,e.tokenA);if ce04Decision(t,scheduled,"capability","device.lifecycle","").Allowed||scheduled.NextTransitionAt!=start.Format(time.RFC3339Nano){t.Fatal(scheduled)}
 ce06WaitDB(t,e.db,start);active:=ce06My(t,e,e.tokenA)
 if active.EntitlementVersion<=scheduled.EntitlementVersion||!ce04Decision(t,active,"capability","device.lifecycle","").Allowed||active.ValidUntil!=end.Format(time.RFC3339Nano){t.Fatal(active)}
 ce06WaitDB(t,e.db,end);expired:=ce06My(t,e,e.tokenA)
 if expired.EntitlementVersion<=active.EntitlementVersion||expired.SourceVersion!=active.SourceVersion||ce04Decision(t,expired,"capability","device.lifecycle","").Allowed{t.Fatal(expired)}
 _,err=e.devices.CreateDevice(ce04Context(e.tokenA,ce04Random(t)),&devicev1.CreateDeviceRequest{SiteId:e.siteA,Name:"expired",Serial:ce04Random(t)})
 ce05RPCDenied(t,err,codes.PermissionDenied,"MODULE_NOT_ENTITLED")
}
func TestCE06MySQLAtomicInvalidationAndSnapshotRollback(t *testing.T){
 e:=ce05New(t);g:=e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE,"device-operations",0);first:=ce06My(t,e,e.tokenA)
 name:="ce06:abort-audit";if err:=e.db.Callback().Create().Before("gorm:create").Register(name,func(tx *gorm.DB){if tx.Statement.Table=="biz_commercial_entitlement_audit"{tx.AddError(errors.New("CE06 injected write failure"))}});err!=nil{t.Fatal(err)}
 _,err:=e.client.RevokeEntitlementOverride(ce04Context(e.token,ce04Random(t)),&commercialv1.RevokeEntitlementOverrideRequest{TenantId:e.tenantA,Id:g.Source.Id,ExpectedVersion:1,RequestId:ce04Random(t),Reason:"rollback probe"})
 _=e.db.Callback().Create().Remove(name);if err==nil{t.Fatal("write failure ignored")}
 same:=ce06My(t,e,e.tokenA);if same.SourceVersion!=1||same.EntitlementVersion!=first.EntitlementVersion{t.Fatal("partial invalidation",same)}
 e.revoke(g.Source.Id,ce04Random(t),1);snapshots,ctx:=ce06Store(t,e,nil)
 factory,err:=requestscope.NewGORMExecutionFactory(e.db);if err!=nil{t.Fatal(err)}
 rootCtx,root,err:=execution.BeginRoot(ctx,"ce06.snapshot.rollback",execution.TransactionLocal,nil,factory);if err!=nil{t.Fatal(err)}
 built,err:=snapshots.ReadSnapshot(rootCtx,e.tenantA,nil);if err!=nil{_ = root.Rollback(rootCtx);t.Fatal(err)};if err:=root.Rollback(rootCtx);err!=nil{t.Fatal(err)}
 var version uint64;if err:=e.db.Table("biz_commercial_entitlement_snapshot_heads").Select("version").Where("tenant_id=?",e.tenantA).Scan(&version).Error;err!=nil{t.Fatal(err)}
 if version!=first.EntitlementVersion{t.Fatal("snapshot pointer escaped rollback",version)}
 refreshed:=ce06My(t,e,e.tokenA);if refreshed.EntitlementVersion!=built.EntitlementVersion||refreshed.SourceVersion!=2{t.Fatal(refreshed)}
 // Pointer loss with intact immutable history must not reset the version.
 if err:=e.db.Exec("DELETE FROM biz_commercial_entitlement_snapshot_heads WHERE tenant_id=?",e.tenantA).Error;err!=nil{t.Fatal(err)}
 _,err=snapshots.ReadSnapshot(ctx,e.tenantA,nil);if !errors.Is(err,consistency.ErrStateLost){t.Fatal(err)}
}
func TestCE06MySQLConcurrentReadsPreserveVersion(t *testing.T){
 e:=ce05New(t);e.grant(commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE,"device-operations",0);first:=ce06My(t,e,e.tokenA)
 type sample struct{duration time.Duration;err error;version uint64};results:=make(chan sample,16)
 for i:=0;i<16;i++{go func(){start:=time.Now();ctx,cancel:=context.WithTimeout(ce04Context(e.tokenA,""),5*time.Second);defer cancel();v,err:=e.client.GetMyEntitlements(ctx,&commercialv1.GetMyEntitlementsRequest{});results<-sample{time.Since(start),err,v.GetEntitlementVersion()}}()}
 var max,total time.Duration;for i:=0;i<16;i++{s:=<-results;if s.err!=nil||s.version!=first.EntitlementVersion{t.Fatal(s)};total+=s.duration;if s.duration>max{max=s.duration}}
 t.Logf("CE06_LOCK_COST sample=16 concurrent_same_tenant mean=%s max=%s; qualification-run sample, not production SLO",total/16,max)
}
