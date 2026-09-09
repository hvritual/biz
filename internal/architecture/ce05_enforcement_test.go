package architecture_test

import (
 "bytes"
 "os"
 "path/filepath"
 "slices"
 "strings"
 "testing"
 assets "github.com/hvritual/biz/contracts/commercial"
 "github.com/hvritual/biz/internal/commercial/capabilitymap"
)
func TestCE05EmbeddedPolicyAndRealConditionalChild(t *testing.T) {
 root:=filepath.Join("..","..");d,err:=capabilitymap.BuildRepository(root);if err!=nil{t.Fatal(err)}
 if err:=capabilitymap.SyncArtifacts(root,d,false);err!=nil{t.Fatal(err)}
 data,err:=os.ReadFile(filepath.Join(root,capabilitymap.JSONPath));if err!=nil{t.Fatal(err)}
 if !bytes.Equal(data,assets.Catalog()){t.Fatal("embedded policy differs from generated inventory")}
 for _,o:=range d.Operations {if o.OperationID=="device.update" {
  if len(o.Children)!=1||o.Children[0].Mode!="conditional"||o.Children[0].OperationID!="site.validate_transfer_target"||!slices.Equal(o.AlwaysRequiredCapabilityCodes,[]string{"device.lifecycle"}){t.Fatal(o)}
  return
 }}
 t.Fatal("actual device.update conditional edge missing")
}
func TestCE05EnforcementDoesNotCreateExecutorsOrReadRepositories(t *testing.T) {
 root:=filepath.Join("..","..");files,err:=filepath.Glob(filepath.Join(root,"internal/commercial/enforcement/*.go"));if err!=nil{t.Fatal(err)}
 if len(files)==0{t.Fatal("empty enforcement")}
 for _,f:=range files {if strings.HasSuffix(f,"_test.go"){continue};b,err:=os.ReadFile(f);if err!=nil{t.Fatal(err)}
  for _,bad:=range []string{"gorm.io/gorm","infrastructure/persistence","NewExecutor(","NewExecutorWithOptions(",".Begin(",".Transaction(",".Commit("}{if strings.Contains(string(b),bad){t.Fatalf("%s contains %s",f,bad)}}
 }
}
// A new commercial child must have an Application-side check, not only a root guard.
func TestCE05EveryBusinessChildHasInvocationCheck(t *testing.T) {
 root:=filepath.Join("..","..")
 d,err:=capabilitymap.BuildRepository(root);if err!=nil {t.Fatal(err)}
 b,err:=os.ReadFile(filepath.Join(root,"internal/bizruntime/device_entitlements.go"));if err!=nil{t.Fatal(err)}
 byID:=map[string]capabilitymap.CompiledOperation{};for _,o:=range d.Operations{byID[o.OperationID]=o}
 for _,o:=range d.Operations { for _,c:=range o.Children {if byID[c.OperationID].Classification==capabilitymap.TenantBusiness {
  if !strings.Contains(string(b),"enforcement.RequireExecuted(ctx, \""+c.OperationID+"\")") {t.Fatalf("CE05-UNCHECKED-CHILD %s -> %s",o.OperationID,c.OperationID)}
 }}}
}
