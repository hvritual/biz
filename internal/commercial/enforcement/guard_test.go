package enforcement

import (
 "context"
 "encoding/json"
 "errors"
 "net/http"
 "net/http/httptest"
 "testing"
 "github.com/hvritual/biz/internal/commercial/capabilitymap"
 "github.com/hvritual/biz/internal/commercial/domain/entitlement"
 "yunka.io/framework/core/identity"
 "yunka.io/gateway/authz"
)
type testReader struct { result entitlement.Result; err error; requested []string; calls int }
func (r *testReader) Decide(_ context.Context,codes []string)(entitlement.Result,error){r.calls++;r.requested=append([]string(nil),codes...);return r.result,r.err}
type auditRecorder struct { events []AuditEvent }
func (a *auditRecorder) Record(_ context.Context,e AuditEvent){a.events=append(a.events,e)}
func testPrincipal()identity.Principal{return identity.Principal{Subject:"u",UserID:"u",TenantID:"a",Authenticated:true,AuthMethod:"api_key"}}
func authorized(id string,p identity.Principal)authz.AuthorizedOperation{return authz.AuthorizedOperation{Principal:p,Policy:authz.Policy{Operation:authz.OperationID(id)},Decision:authz.Decision{Allowed:true,Operation:authz.OperationID(id)}}}
func allowedReader() *testReader {return &testReader{result:entitlement.Result{TenantID:"a",SourceVersion:7,Decisions:[]entitlement.Decision{{Kind:entitlement.Capability,Key:"device.lifecycle",Allowed:true,Reason:"ALLOWED"}}}}}
func TestCE05GuardPolicyAndContext(t *testing.T) {
 for _,v:=range []struct{name,operation,reason string}{
  {"unknown root","device.unknown","OPERATION_NOT_CLASSIFIED"},
  {"private root","site.validate_transfer_target","INTERNAL_OPERATION_ROOT_DENIED"},
  {"wrong authority","commercial.module.list","PLATFORM_CONTEXT_REQUIRED"},
 } {t.Run(v.name,func(t *testing.T){r:=allowedReader();a:=&auditRecorder{};g,err:=New(r,a);if err!=nil{t.Fatal(err)};p:=testPrincipal();ctx:=identity.WithPrincipal(ensureOutcome(context.Background()),p)
  _,err=g.Prepare(ctx,authorized(v.operation,p),nil);var f *Failure;if !errors.As(err,&f)||f.Code!=v.reason{t.Fatal(err)}
  if r.calls!=0 || len(a.events)!=1 || a.events[0].Allowed {t.Fatal("invalid entry reached resolver")}
 })}
 t.Run("conditional excludes unused capability",func(t *testing.T){r:=allowedReader();a:=&auditRecorder{};g,err:=New(r,a);if err!=nil{t.Fatal(err)};p:=testPrincipal();ctx:=identity.WithPrincipal(context.Background(),p)
  _,err=g.Prepare(ctx,authorized("device.update",p),nil);if err!=nil{t.Fatal(err)}
  if len(r.requested)!=1||r.requested[0]!="device.lifecycle"{t.Fatal(r.requested)}
  if a.events[0].SourceVersion!=7||a.events[0].CorrelationID==""{t.Fatal(a.events)}
 })
 t.Run("mandatory closure includes child",func(t *testing.T){r:=allowedReader();a:=&auditRecorder{};g,_:=New(r,a);p:=testPrincipal();_,err:=g.Prepare(identity.WithPrincipal(context.Background(),p),authorized("device.transfer",p),nil)
  if err==nil||len(r.requested)!=2{t.Fatal(err,r.requested)}
 })
 t.Run("missing scope cannot reuse token",func(t *testing.T){r:=allowedReader();g,_:=New(r,&auditRecorder{});p:=testPrincipal();ctx,err:=g.Prepare(identity.WithPrincipal(context.Background(),p),authorized("device.list",p),nil);if err!=nil{t.Fatal(err)}
  if RequireExecuted(ctx,"device.list")==nil{t.Fatal("raw call accepted")}
  changed:=p;changed.TenantID="b";if RequireExecuted(identity.WithPrincipal(ctx,changed),"device.create")==nil{t.Fatal("cross-tenant token accepted")}
  if RequireExecuted(context.Background(),"device.list")==nil{t.Fatal("missing frame accepted")}
 })
 t.Run("explicit recovery avoids business reader",func(t *testing.T){r:=allowedReader();g,_:=New(r,&auditRecorder{});p:=testPrincipal();_,err:=g.Prepare(identity.WithPrincipal(context.Background(),p),authorized("commercial.entitlement.get_my",p),nil);if err!=nil||r.calls!=0{t.Fatal(err,r.calls)}})
 t.Run("source error fails closed",func(t *testing.T){r:=allowedReader();r.err=errors.New("synthetic DB failure");g,_:=New(r,&auditRecorder{});p:=testPrincipal();_,err:=g.Prepare(identity.WithPrincipal(context.Background(),p),authorized("device.list",p),nil);var f *Failure;if !errors.As(err,&f)||!f.Unavailable||f.Code!="ENTITLEMENT_SOURCE_UNAVAILABLE"{t.Fatal(err)}})
 t.Run("reader tenant mismatch fails closed",func(t *testing.T){r:=allowedReader();r.result.TenantID="b";g,_:=New(r,&auditRecorder{});p:=testPrincipal();_,err:=g.Prepare(identity.WithPrincipal(context.Background(),p),authorized("device.list",p),nil);if err==nil{t.Fatal("cross tenant decision accepted")}})
}
func TestCE05GuardRejectsInvalidPolicy(t *testing.T) {
 cases:=[]capabilitymap.Document{{},{MappingVersion:"x"},{MappingVersion:"x",Operations:[]capabilitymap.CompiledOperation{{Mapping:capabilitymap.Mapping{OperationID:"x",Classification:capabilitymap.TenantBusiness}}}}}
 for i,d:=range cases {if _,err:=newGuard(d,allowedReader(),&auditRecorder{});err==nil{t.Fatal(i)}}
 if _,err:=New(nil,&auditRecorder{});err==nil{t.Fatal("nil reader")}
 if _,err:=New(allowedReader(),nil);err==nil{t.Fatal("nil audit")}
}
func TestCE05HTTPErrorProjectionDoesNotWeakenIAM(t *testing.T) {
 for _,unavailable:=range []bool{false,true}{r:=httptest.NewRequest("GET","/",nil);w:=httptest.NewRecorder()
  HTTP(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){remember(r.Context(),&Failure{Code:"TEST_DENIED",Operation:"device.list",CorrelationID:correlation(r.Context()),Unavailable:unavailable});http.Error(w,"Forbidden",403)})).ServeHTTP(w,r)
  expected:=403;if unavailable{expected=503};if w.Code!=expected{t.Fatal(w.Code)}
  var f Failure;if err:=json.Unmarshal(w.Body.Bytes(),&f);err!=nil||f.CorrelationID==""||f.Code!="TEST_DENIED"{t.Fatal(f,err)}
 }
 w:=httptest.NewRecorder();HTTP(http.HandlerFunc(func(w http.ResponseWriter,_ *http.Request){http.Error(w,"Unauthorized",401)})).ServeHTTP(w,httptest.NewRequest("GET","/",nil))
 if w.Code!=401||w.Body.String()!="Unauthorized\n"{t.Fatal(w)}
}
