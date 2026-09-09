//go:build integration

package integration

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/hvritual/biz/internal/commercial/modulecatalog"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
)

func ce02DB(t *testing.T)*gorm.DB{t.Helper();dsn:=os.Getenv("YUNKA_TEST_MYSQL_DSN");if dsn==""{t.Skip("YUNKA_TEST_MYSQL_DSN required")};db,err:=gorm.Open(mysql.Open(dsn),&gorm.Config{});if err!=nil{t.Fatal(err)};if err:=db.Migrator().DropTable("biz_commercial_module_idempotency","biz_commercial_module_audit","biz_commercial_module_dependencies","biz_commercial_module_retired_codes","biz_commercial_modules");err!=nil{t.Fatal(err)};store,err:=modulecatalog.NewStore(db);if err!=nil{t.Fatal(err)};if err:=store.Migrate(context.Background());err!=nil{t.Fatal(err)};return db}
func ce02Platform()context.Context{return identity.WithPrincipal(context.Background(),identity.Principal{Subject:"platform-admin:ce02",Roles:[]string{"platform-admin"},AuthMethod:identity.AuthMethodAPIKey,Authenticated:true})}
func ce02Tenant()context.Context{return identity.WithPrincipal(context.Background(),identity.Principal{Subject:"tenant-owner:ce02",TenantID:"tenant-1",UserID:"u1",Roles:[]string{"owner"},AuthMethod:identity.AuthMethodAPIKey,Authenticated:true})}

func TestCE02MySQLCreateUpdateRetireAndIdempotency(t *testing.T){db:=ce02DB(t);store,_:=modulecatalog.NewStore(db);svc,err:=modulecatalog.NewService(store,modulecatalog.ProductionRegistry());if err!=nil{t.Fatal(err)};ctx:=ce02Platform()
	m,err:=svc.Create(ctx,modulecatalog.CreateCommand{RequestID:"ce02-create-access",Code:"access-management",Name:"Access",Category:"platform",SalesScope:[]string{"global","global"},Reason:"initial catalog"});if err!=nil{t.Fatal(err)};if m.Version!=1||m.TechnicalStatus!=modulecatalog.TechnicalReady||m.SalesStatus!=modulecatalog.SalesSellable{t.Fatalf("unexpected create: %#v",m)}
	dup,err:=svc.Create(ctx,modulecatalog.CreateCommand{RequestID:"ce02-create-access",Code:"access-management",Name:"IGNORED",Category:"x",Reason:"same request"});if err!=nil{t.Fatal(err)};if dup.Name!="Access"||dup.Version!=1{t.Fatalf("idempotent response changed: %#v",dup)}
	m,err=svc.Update(ctx,modulecatalog.UpdateCommand{RequestID:"ce02-update-access",Code:m.Code,Name:"Access Management",Category:"core",SalesScope:[]string{"enterprise"},Version:m.Version,Reason:"rename display metadata"});if err!=nil{t.Fatal(err)};if m.Version!=2||m.Name!="Access Management"{t.Fatalf("update=%#v",m)}
	if _,err:=svc.Update(ctx,modulecatalog.UpdateCommand{RequestID:"ce02-stale",Code:m.Code,Name:"stale",Version:1,Reason:"stale"});!errors.Is(err,modulecatalog.ErrConflict){t.Fatalf("stale err=%v",err)}
	m,err=svc.SetSalesStatus(ctx,modulecatalog.StatusCommand{RequestID:"ce02-retire",Code:m.Code,Version:m.Version,Sales:modulecatalog.SalesRetired,Reason:"stop new sales"});if err!=nil{t.Fatal(err)};if m.SalesStatus!=modulecatalog.SalesRetired||m.TechnicalStatus!=modulecatalog.TechnicalReady{t.Fatalf("sales retirement changed technical status: %#v",m)}
}

func TestCE02MySQLRejectsTenantUnknownImplementationReferenceAndCodeReuse(t *testing.T){db:=ce02DB(t);store,_:=modulecatalog.NewStore(db);reg,err:=modulecatalog.NewRegistry([]modulecatalog.Definition{{Code:"core",ImplementationReady:true},{Code:"child",Dependencies:[]string{"core"},ImplementationReady:true},{Code:"future",ImplementationReady:false}});if err!=nil{t.Fatal(err)};svc,_:=modulecatalog.NewService(store,reg)
	if _,err:=svc.Create(ce02Tenant(),modulecatalog.CreateCommand{RequestID:"tenant",Code:"core",Name:"Core",Reason:"forbidden"});!errors.Is(err,modulecatalog.ErrPlatformPrincipalRequired){t.Fatalf("tenant err=%v",err)}
	if _,err:=svc.Create(ce02Platform(),modulecatalog.CreateCommand{RequestID:"unknown",Code:"undeclared",Name:"Unknown",Reason:"invalid"});!errors.Is(err,modulecatalog.ErrUnknownDefinition){t.Fatalf("unknown err=%v",err)}
	if _,err:=svc.Create(ce02Platform(),modulecatalog.CreateCommand{RequestID:"child-first",Code:"child",Name:"Child",Reason:"missing dep"});!errors.Is(err,modulecatalog.ErrDependencyMissing){t.Fatalf("missing dep err=%v",err)}
	core,err:=svc.Create(ce02Platform(),modulecatalog.CreateCommand{RequestID:"core",Code:"core",Name:"Core",Reason:"create"});if err!=nil{t.Fatal(err)}
	child,err:=svc.Create(ce02Platform(),modulecatalog.CreateCommand{RequestID:"child",Code:"child",Name:"Child",Reason:"create"});if err!=nil{t.Fatal(err)}
	if err:=svc.Delete(ce02Platform(),modulecatalog.DeleteCommand{RequestID:"delete-core",Code:"core",Version:core.Version,Reason:"referenced"});!errors.Is(err,modulecatalog.ErrReferenced){t.Fatalf("referenced delete err=%v",err)}
	future,err:=svc.Create(ce02Platform(),modulecatalog.CreateCommand{RequestID:"future",Code:"future",Name:"Future",Reason:"registered but unavailable"});if err!=nil{t.Fatal(err)};if future.TechnicalStatus!=modulecatalog.TechnicalNotReady{t.Fatalf("future status=%v",future.TechnicalStatus)};if _,err:=svc.SetTechnicalStatus(ce02Platform(),modulecatalog.StatusCommand{RequestID:"future-ready",Code:"future",Version:future.Version,Technical:modulecatalog.TechnicalReady,Reason:"invalid ready"});!errors.Is(err,modulecatalog.ErrImplementationUnavailable){t.Fatalf("ready err=%v",err)}
	if err:=svc.Delete(ce02Platform(),modulecatalog.DeleteCommand{RequestID:"delete-child",Code:"child",Version:child.Version,Reason:"remove"});err!=nil{t.Fatal(err)};if _,err:=svc.Create(ce02Platform(),modulecatalog.CreateCommand{RequestID:"reuse-child",Code:"child",Name:"Child2",Reason:"reuse"});!errors.Is(err,modulecatalog.ErrCodeRetired){t.Fatalf("reuse err=%v",err)}
}
