//go:build integration

package integration

import (
 "bytes"
 "encoding/json"
 "io"
 "net/http"
 "testing"
 "time"
 commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
 "github.com/hvritual/biz/internal/commercial/domain/entitlement"
 "google.golang.org/protobuf/encoding/protojson"
)

func ce04Post(t *testing.T,e *ce04Environment,path,token string,payload any)(int,[]byte){
 t.Helper();body,err:=json.Marshal(payload);if err!=nil{t.Fatal(err)}
 req,err:=http.NewRequest(http.MethodPost,"http://"+e.runtime.HTTPAddress()+path,bytes.NewReader(body));if err!=nil{t.Fatal(err)}
 req.Header.Set("Authorization","Bearer "+token);req.Header.Set("Content-Type","application/json")
 response,err:=(&http.Client{Timeout:10*time.Second}).Do(req);if err!=nil{t.Fatal(err)}
 defer response.Body.Close();data,err:=io.ReadAll(response.Body);if err!=nil{t.Fatal(err)}
 return response.StatusCode,data
}

func TestCE04MySQLRepeatedRequestBodyRESTAndGRPCParity(t *testing.T){
 e:=ce04NewEnvironment(t,"")
 for _,fieldName:=range []string{"capabilityCodes","capability_codes"}{
  payload:=map[string]any{fieldName:[]string{"unknown.one","unknown.two"}}
  for _,target:=range []struct{path,token string}{{"/v1/tenant/entitlements",e.tokenA},{"/v1/platform/tenants/"+e.tenantA+"/entitlements",e.token}}{
   code,data:=ce04Post(t,e,target.path,target.token,payload);if code!=200{t.Fatalf("REST %d: %s",code,data)}
   var view commercialv1.EntitlementView;if err:=protojson.Unmarshal(data,&view);err!=nil{t.Fatal(err)}
   for _,key:=range []string{"unknown.one","unknown.two"}{if d:=ce04Decision(t,&view,"capability",key,"");d.Allowed||d.Reason!="UNKNOWN_CAPABILITY"{t.Fatal(d)}}
  }
 }
 own,err:=e.client.GetMyEntitlements(ce04Context(e.tokenA,""),&commercialv1.GetMyEntitlementsRequest{CapabilityCodes:[]string{"unknown.one","unknown.two"}});if err!=nil{t.Fatal(err)}
 platform,err:=e.client.ExplainEntitlements(ce04Context(e.token,""),&commercialv1.ExplainEntitlementsRequest{TenantId:e.tenantA,CapabilityCodes:[]string{"unknown.one","unknown.two"}});if err!=nil{t.Fatal(err)}
 for _,v:=range []*commercialv1.EntitlementView{own,platform}{for _,key:=range []string{"unknown.one","unknown.two"}{if ce04Decision(t,v,"capability",key,"").Reason!="UNKNOWN_CAPABILITY"{t.Fatal(v)}}}
 tooMany:=make([]string,129);for i:=range tooMany{tooMany[i]="unknown.one"}
 for _,target:=range []struct{path,token string}{{"/v1/tenant/entitlements",e.tokenA},{"/v1/platform/tenants/"+e.tenantA+"/entitlements",e.token}}{
  if code,_:=ce04Post(t,e,target.path,target.token,map[string]any{"capabilityCodes":tooMany});code>=200&&code<300{t.Fatal("REST bypassed 128-code bound")}
 }
 if _,err:=e.client.GetMyEntitlements(ce04Context(e.tokenA,""),&commercialv1.GetMyEntitlementsRequest{CapabilityCodes:tooMany});err==nil{t.Fatal("gRPC bypassed 128-code bound")}
 if _,err:=e.client.ExplainEntitlements(ce04Context(e.token,""),&commercialv1.ExplainEntitlementsRequest{TenantId:e.tenantA,CapabilityCodes:tooMany});err==nil{t.Fatal("platform gRPC bypassed 128-code bound")}
 if code,_:=ce04Post(t,e,"/v1/tenant/entitlements",e.tokenA,map[string]any{"tenantId":e.tenantB});code>=200&&code<300{t.Fatal("own request accepted client-selected tenant")}
}

func TestCE04MySQLWellShapedOrphanSourceFailsClosed(t *testing.T){
 e:=ce04NewEnvironment(t,"")
 receipt:=e.mustCreate(ce04Request(e.tenantA,"orphan-fixture",0,commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY,"device.lifecycle",commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT))
 var row struct{Payload string}
 if err:=e.db.Table("biz_commercial_entitlement_sources").Where("tenant_id = ? AND source_id = ?",e.tenantA,receipt.Source.Id).Take(&row).Error;err!=nil{t.Fatal(err)}
 var source entitlement.Source;if err:=json.Unmarshal([]byte(row.Payload),&source);err!=nil{t.Fatal(err)};source.Key="device.not_deployed"
 if err:=source.ValidateShape();err!=nil{t.Fatal(err)}
 payload,err:=json.Marshal(source);if err!=nil{t.Fatal(err)}
 if err:=e.db.Table("biz_commercial_entitlement_sources").Where("tenant_id = ? AND source_id = ?",e.tenantA,source.ID).Update("payload",string(payload)).Error;err!=nil{t.Fatal(err)}
 if _,err:=e.client.ExplainEntitlements(ce04Context(e.token,""),&commercialv1.ExplainEntitlementsRequest{TenantId:e.tenantA});err==nil{t.Fatal("well-shaped orphan accepted")}
 if _,err:=e.client.GetMyEntitlements(ce04Context(e.tokenA,""),&commercialv1.GetMyEntitlementsRequest{});err==nil{t.Fatal("tenant orphan accepted")}
}

func TestCE04MySQLReferencedModuleCannotBeHardDeleted(t *testing.T){
 e:=ce04NewEnvironment(t,"")
 source:=e.mustCreate(ce04Request(e.tenantA,"reference",0,commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_MODULE,"device-operations",commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT))
 module,err:=e.catalog.GetModule(ce04Context(e.token,""),&commercialv1.GetModuleRequest{ModuleCode:"device-operations"});if err!=nil{t.Fatal(err)}
 for _,revoked:=range []bool{false,true}{
  if revoked{e.revoke(source.Source.Id,"revoke-reference",1)}
  key:=ce04Random(t)
  if _,err:=e.catalog.DeleteModule(ce04Context(e.token,key),&commercialv1.DeleteModuleRequest{RequestId:key,ModuleCode:module.ModuleCode,Version:module.Version,Reason:"CE04 reference invariant"});err==nil{t.Fatal("referenced module deleted")}
  if _,err:=e.catalog.GetModule(ce04Context(e.token,""),&commercialv1.GetModuleRequest{ModuleCode:module.ModuleCode});err!=nil{t.Fatal("module vanished on failed delete",err)}
  if view:=e.explain(e.tenantA);ce04Decision(t,view,"capability","device.lifecycle","").Allowed==revoked{t.Fatal("invalid decision after rejected delete",view)}
 }
}
