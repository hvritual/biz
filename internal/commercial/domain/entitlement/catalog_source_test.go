package entitlement

import (
 "errors"
 "testing"
)

func TestCE04AllSourcesMustMatchCurrentCatalog(t *testing.T) {
 cases:=[]struct{name string;kind Kind;key string;module string;action string;effect Effect}{
  {"unknown module",Module,"removed","removed","",Grant},
  {"unknown capability",Capability,"devices.missing","devices","",Grant},
  {"unknown quota",Quota,"devices.missing","devices","",QuotaReplace},
  {"unknown field",Field,"device.missing","devices","read",Grant},
 }
 for _,tc:=range cases { t.Run(tc.name,func(t *testing.T){
  s:=source("orphan",tc.kind,tc.key,tc.effect);s.ModuleCode=tc.module;s.Action=tc.action
  if err:=s.ValidateShape();err!=nil{t.Fatal("fixture must have valid shape",err)}
  for _,kind:=range []SourceKind{OverrideSource,PlanSource,AddonSource}{s.SourceKind=kind
   if _,err:=Resolve("tenant-a",1,epoch,catalogFixture(),[]Source{s},nil);!errors.Is(err,ErrCatalog){t.Fatalf("%s: %v",kind,err)}
  }
  at:=epoch;s.RevokedAt=&at
  if _,err:=Resolve("tenant-a",1,epoch,catalogFixture(),[]Source{s},nil);!errors.Is(err,ErrCatalog){t.Fatal("orphan history was silently accepted",err)}
 }) }
}
