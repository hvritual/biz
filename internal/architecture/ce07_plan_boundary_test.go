package architecture_test

import (
 "os"
 "path/filepath"
 "strings"
 "testing"
 "github.com/hvritual/biz/internal/commercial/capabilitymap"
)
func TestCE07PlanOwnerAndNoHardDelete(t *testing.T){
 root:=filepath.Join("..","..");b,err:=os.ReadFile(filepath.Join(root,"contracts/proto/commercial/v1/plan.proto"));if err!=nil{t.Fatal(err)};if strings.Contains(string(b),"rpc Delete")||strings.Contains(string(b),"tenant_required: true"){t.Fatal("plan authoring leaks tenant/delete surface")}
 err=filepath.WalkDir(filepath.Join(root,"internal/commercial/application/planmanagement"),func(path string,d os.DirEntry,err error)error{if err!=nil{return err};if d.IsDir()||!strings.HasSuffix(path,".go"){return nil};b,err:=os.ReadFile(path);if err!=nil{return err};for _,forbidden:=range []string{"gorm.io","internal/access/infrastructure","internal/commercial/modulecatalog","operation.NewExecutor"}{if strings.Contains(string(b),forbidden){t.Errorf("owner boundary %s contains %s",path,forbidden)}};return nil});if err!=nil{t.Fatal(err)}
 doc,err:=capabilitymap.BuildRepository(root);if err!=nil{t.Fatal(err)};count:=0;for _,op:=range doc.Operations{if strings.HasPrefix(op.OperationID,"commercial.plan."){count++;if op.Classification!=capabilitymap.PlatformManagement{t.Errorf("unprotected classification %s",op.OperationID)}}};if count!=8{t.Fatalf("expected eight declared plan operations, got %d",count)}
}
