package architecture_test
import("encoding/json";"os";"path/filepath";"strings";"testing";"github.com/hvritual/biz/internal/commercial/capabilitymap")
func TestCE06AllBusinessWritesJoinInvocationBarrier(t *testing.T){
 root:=filepath.Join("..","..");doc,err:=capabilitymap.BuildRepository(root);if err!=nil{t.Fatal(err)}
 b,err:=os.ReadFile(filepath.Join(root,"contracts/generated/operation-plans.json"));if err!=nil{t.Fatal(err)}
 var plans struct{Operations []struct{ID string `json:"operationId"`;Execution struct{Transaction string `json:"transaction"`} `json:"execution"`} `json:"operations"`}
 if err:=json.Unmarshal(b,&plans);err!=nil{t.Fatal(err)};writes:=map[string]bool{};for _,o:=range plans.Operations{writes[o.ID]=o.Execution.Transaction=="local"}
 var joined string;for _,f:=range []string{"device_entitlements.go","member_entitlements.go","role_entitlements.go"}{b,err:=os.ReadFile(filepath.Join(root,"internal/bizruntime",f));if err!=nil{t.Fatal(err)};joined+=string(b)}
 count:=0;for _,o:=range doc.Operations{if o.Classification==capabilitymap.TenantBusiness&&writes[o.OperationID]{count++;if !strings.Contains(joined,`enforcement.RequireExecuted(ctx, "`+o.OperationID+`")`){t.Fatalf("unfenced business write %s",o.OperationID)}}};if count==0{t.Fatal("no business writes tested")}
}
