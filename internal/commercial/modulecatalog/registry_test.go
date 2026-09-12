package modulecatalog

import (
	"errors"
	"testing"
)

func TestCE02DefinitionGraphValidation(t *testing.T){
	if _,err:=NewRegistry([]Definition{{Code:"a",Dependencies:[]string{"missing"}}});!errors.Is(err,ErrDependencyMissing){t.Fatalf("missing dependency err=%v",err)}
	if _,err:=NewRegistry([]Definition{{Code:"a",Dependencies:[]string{"b"}},{Code:"b",Dependencies:[]string{"a"}}});!errors.Is(err,ErrDependencyCycle){t.Fatalf("cycle err=%v",err)}
	r:=ProductionRegistry();if err:=r.Validate();err!=nil{t.Fatal(err)}
	if d,ok:=r.Definition("access-management");!ok||len(d.CapabilityCodes)==0||len(d.QuotaSchemaKeys)==0||len(d.FieldPolicySchemaKeys)==0||!d.ImplementationReady{t.Fatalf("production definition incomplete: %#v",d)}
}
