package architecture

import (
 "encoding/json"
 "os"
 "path/filepath"
 "reflect"
 "strings"
 "testing"
 v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
 "google.golang.org/protobuf/reflect/protoreflect"
)
func TestCE09DeclaredChangeSecurityAndTransportAreExact(t *testing.T){
 raw,err:=os.ReadFile(filepath.Join("..","..","contracts","generated","operation-plans.json"));if err!=nil{t.Fatal(err)}
 var manifest struct{Operations []struct{
  ID string `json:"operationId"`
  Security struct{Permissions []string `json:"permissions"`;Mode string `json:"permissionMode"`} `json:"security"`
  Execution struct{Transaction string `json:"transaction"`;Idempotency string `json:"idempotency"`} `json:"execution"`
  Bindings struct{HTTP []struct{Method,Path string}} `json:"bindings"`
  Composition struct{Boundary string `json:"boundary"`;Requires []string `json:"requiresOperations"`} `json:"composition"`
 }}
 if err=json.Unmarshal(raw,&manifest);err!=nil{t.Fatal(err)}
 found:=map[string]bool{}
 for _,op:=range manifest.Operations{
  if !strings.HasPrefix(op.ID,"commercial.subscription.change."){continue};found[op.ID]=true
  if len(op.Bindings.HTTP)!=1||!strings.HasPrefix(op.Bindings.HTTP[0].Path,"/v1/platform/tenants/"){t.Fatalf("unscoped change route %v",op)}
  if op.Security.Mode!="all"{t.Fatal("change permission closure relaxed")}
  write:=op.ID=="commercial.subscription.change.preview"||op.ID=="commercial.subscription.change.confirm"
  if write{
   permission:="platform.subscription.manage";if strings.HasSuffix(op.ID,"confirm"){permission="platform.subscription.confirm"}
   want:=[]string{"commercial.catalog.read","platform.plan.read",permission,"platform.tenant.read"}
   if !reflect.DeepEqual(op.Security.Permissions,want)||op.Execution.Transaction!="local"||op.Execution.Idempotency!="required"||op.Composition.Boundary!="local"||len(op.Composition.Requires)!=3{t.Fatalf("wrong mutation boundary %v",op)}
  } else if op.Execution.Transaction!="read-only"||op.Execution.Idempotency!="none" {t.Fatalf("readback is not read-only %v",op)}
 }
 if len(found)!=4{t.Fatalf("change operations=%d",len(found))}
 for _,message:=range []protoreflect.MessageDescriptor{(&v1.ConfirmSubscriptionChangeRequest{}).ProtoReflect().Descriptor(),(&v1.PreviewSubscriptionChangeRequest{}).ProtoReflect().Descriptor()}{
  for _,name:=range []protoreflect.Name{"paid","payment_status","used","entitlement_version","source_version","effective_entitlements"}{if message.Fields().ByName(name)!=nil{t.Fatalf("client authority field %s",name)}}
 }
}
