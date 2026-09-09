package entitlement

import (
 "encoding/json"
 "errors"
 "math"
 "testing"
 "time"
)
var epoch=time.Date(2026,9,9,0,0,0,0,time.UTC)
func catalogFixture()Catalog{return Catalog{{Code:"devices",Version:1,TechnicalStatus:"ready",SalesStatus:"sellable",Capabilities:[]string{"devices.read","devices.write"},QuotaKeys:[]string{"devices.count"},FieldKeys:[]string{"device.identity"}}}}
func source(id string,k Kind,key string,e Effect)Source{return Source{ID:id,TenantID:"tenant-a",SourceKind:OverrideSource,ModuleCode:"devices",Kind:k,Key:key,Effect:e,EffectiveAt:epoch.Add(-time.Hour),Reason:"qualification source",ActorID:"platform-actor",Version:1}}
func grantModule()Source{return source("module-grant",Module,"devices",Grant)}
func findDecision(t *testing.T,r Result,k Kind,key,action string)Decision{t.Helper();for _,d:=range r.Decisions{if d.Kind==k&&d.Key==key&&d.Action==action{return d}};t.Fatalf("missing decision %s %s %s",k,key,action);return Decision{}}
func resolveFixture(t *testing.T,at time.Time,c Catalog,s []Source)Result{t.Helper();r,err:=Resolve("tenant-a",7,at,c,s,[]string{"unknown.capability"});if err!=nil{t.Fatal(err)};return r}
func TestCE04CapabilityPriorityAndTenantIsolation(t *testing.T){
 cases:=[]struct{name string;sources []Source;status string;allowed bool;reason string}{
  {"no implicit free access",nil,"ready",false,"MODULE_NOT_ENTITLED"},
  {"module grant",[]Source{grantModule()},"ready",true,"ALLOWED"},
  {"specific capability",[]Source{source("grant",Capability,"devices.read",Grant)},"ready",true,"ALLOWED"},
  {"module deny wins",[]Source{grantModule(),source("deny",Module,"devices",Deny)},"ready",false,"CAPABILITY_DISABLED"},
  {"capability deny wins",[]Source{grantModule(),source("deny",Capability,"devices.read",Deny)},"ready",false,"CAPABILITY_DISABLED"},
  {"safety wins ordinary grant",[]Source{grantModule(),source("deny",Module,"devices",SafetyDeny)},"ready",false,"SECURITY_DISABLED"},
  {"technical disabled",[]Source{grantModule()},"disabled",false,"TECHNICAL_UNAVAILABLE"},
  {"implementation not ready",[]Source{grantModule()},"not_ready",false,"TECHNICAL_UNAVAILABLE"},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){c:=catalogFixture();c[0].TechnicalStatus=tc.status;r:=resolveFixture(t,epoch,c,tc.sources);d:=findDecision(t,r,Capability,"devices.read","");if d.Allowed!=tc.allowed||d.Reason!=tc.reason{t.Fatalf("%+v",d)};u:=findDecision(t,r,Capability,"unknown.capability","");if u.Allowed||u.Reason!="UNKNOWN_CAPABILITY"{t.Fatal(u)}})}
 t.Run("B cannot consume A sources",func(t *testing.T){if _,err:=Resolve("tenant-b",0,epoch,catalogFixture(),[]Source{grantModule()},nil);!errors.Is(err,ErrScope){t.Fatal(err)}})
 t.Run("sales retirement preserves grants",func(t *testing.T){c:=catalogFixture();c[0].SalesStatus="retired";r:=resolveFixture(t,epoch,c,[]Source{grantModule()});if !findDecision(t,r,Capability,"devices.read","").Allowed{t.Fatal(r)}})
 t.Run("specific capability is not wildcard",func(t *testing.T){r:=resolveFixture(t,epoch,catalogFixture(),[]Source{source("only-read",Capability,"devices.read",Grant)});if findDecision(t,r,Capability,"devices.write","").Allowed{t.Fatal(r)}})
}
func TestCE04HalfOpenTimeAndRevoke(t *testing.T){
 end:=epoch.Add(time.Hour);s:=grantModule();s.EffectiveAt=epoch;s.ExpiresAt=&end
 for _,tc:=range []struct{name string;at time.Time;allowed bool;state string}{{"before",epoch.Add(-time.Microsecond),false,"scheduled"},{"at start",epoch,true,"active"},{"before end",end.Add(-time.Microsecond),true,"active"},{"at end",end,false,"expired"}}{t.Run(tc.name,func(t *testing.T){r:=resolveFixture(t,tc.at,catalogFixture(),[]Source{s});d:=findDecision(t,r,Capability,"devices.read","");if d.Allowed!=tc.allowed||d.Sources[0].State!=tc.state{t.Fatal(d)};if tc.at.Before(end)&&r.NextTransitionAt==nil{t.Fatal("lost transition")}})}
 t.Run("revoked before future start",func(t *testing.T){revoked:=epoch.Add(-time.Hour);s.RevokedAt=&revoked;r:=resolveFixture(t,epoch,catalogFixture(),[]Source{s});if findDecision(t,r,Capability,"devices.read","").Allowed||r.NextTransitionAt!=nil{t.Fatal(r)}})
 t.Run("next future denial bounds grant",func(t *testing.T){deny:=source("future-deny",Module,"devices",Deny);deny.EffectiveAt=epoch.Add(time.Minute);r:=resolveFixture(t,epoch,catalogFixture(),[]Source{grantModule(),deny});if r.ValidUntil==nil||!r.ValidUntil.Equal(deny.EffectiveAt){t.Fatal(r)}})
}
func TestCE04DependenciesAndInvalidCatalog(t *testing.T){
 c:=catalogFixture();c[0].Dependencies=[]string{"core"};c=append(c,ModuleDefinition{Code:"core",Version:1,TechnicalStatus:"ready",Capabilities:[]string{"core.use"}})
 core:=source("core",Module,"core",Grant);core.ModuleCode="core"
 t.Run("missing dependent grant",func(t *testing.T){r:=resolveFixture(t,epoch,c,[]Source{grantModule()});if d:=findDecision(t,r,Capability,"devices.read","");d.Allowed||d.Reason!="DEPENDENCY_UNAVAILABLE"{t.Fatal(d)}})
 t.Run("dependency granted",func(t *testing.T){r:=resolveFixture(t,epoch,c,[]Source{grantModule(),core});if !findDecision(t,r,Capability,"devices.read","").Allowed{t.Fatal(r)}})
 t.Run("missing persisted dependency",func(t *testing.T){r:=resolveFixture(t,epoch,c[:1],[]Source{grantModule()});if d:=findDecision(t,r,Capability,"devices.read","");d.Allowed||d.Reason!="DEPENDENCY_UNAVAILABLE"{t.Fatal(d)}})
 t.Run("cycle rejected",func(t *testing.T){c[1].Dependencies=[]string{"devices"};if _,err:=Resolve("tenant-a",1,epoch,c,nil,nil);!errors.Is(err,ErrCatalog){t.Fatal(err)}})
 t.Run("unknown technical state rejected",func(t *testing.T){c:=catalogFixture();c[0].TechnicalStatus="broken";if _,err:=Resolve("tenant-a",1,epoch,c,nil,nil);!errors.Is(err,ErrCatalog){t.Fatal(err)}})
}
func TestCE04QuotaReplacementAndRestoration(t *testing.T){
 add:=source("add",Quota,"devices.count",QuotaAdd);add.Limit.Value=7
 replacement:=source("replacement",Quota,"devices.count",QuotaReplace);replacement.Limit.Value=20
 r:=resolveFixture(t,epoch,catalogFixture(),[]Source{grantModule(),add,replacement});d:=findDecision(t,r,Quota,"devices.count","")
 if d.Limit.Value!=20||!d.Allowed{t.Fatal(d)}
 shadowed:=false;for _,s:=range d.Sources{if s.ID=="add"{shadowed=s.Disposition=="overridden_by_replacement"}};if !shadowed{t.Fatal(d)}
 t.Run("revocation restores increment",func(t *testing.T){at:=epoch;replacement.RevokedAt=&at;r:=resolveFixture(t,epoch,catalogFixture(),[]Source{grantModule(),add,replacement});if d:=findDecision(t,r,Quota,"devices.count","");d.Limit.Value!=7{t.Fatal(d)}})
 t.Run("zero is finite",func(t *testing.T){z:=source("zero",Quota,"devices.count",QuotaReplace);r:=resolveFixture(t,epoch,catalogFixture(),[]Source{grantModule(),z});d:=findDecision(t,r,Quota,"devices.count","");if d.Limit.Unlimited||d.Limit.Value!=0||d.Reason!="ZERO_QUOTA"{t.Fatal(d)}})
 t.Run("unlimited is explicit",func(t *testing.T){u:=source("unlimited",Quota,"devices.count",QuotaReplace);u.Limit.Unlimited=true;r:=resolveFixture(t,epoch,catalogFixture(),[]Source{grantModule(),u});if !findDecision(t,r,Quota,"devices.count","").Limit.Unlimited{t.Fatal(r)}})
 t.Run("duplicate replacement fail closed",func(t *testing.T){a:=source("a",Quota,"devices.count",QuotaReplace);b:=a;b.ID="b";r:=resolveFixture(t,epoch,catalogFixture(),[]Source{grantModule(),a,b});if d:=findDecision(t,r,Quota,"devices.count","");d.Allowed||d.Reason!="QUOTA_CONFIGURATION_CONFLICT"{t.Fatal(d)}})
 t.Run("overflow fail closed",func(t *testing.T){a:=add;a.Limit.Value=math.MaxInt64;b:=add;b.ID="b";r:=resolveFixture(t,epoch,catalogFixture(),[]Source{grantModule(),a,b});if d:=findDecision(t,r,Quota,"devices.count","");d.Allowed||d.Reason!="QUOTA_OVERFLOW"{t.Fatal(d)}})
 t.Run("future replacement overlap rejected",func(t *testing.T){a:=source("a",Quota,"devices.count",QuotaReplace);a.EffectiveAt=epoch.Add(time.Hour);b:=a;b.ID="b";b.EffectiveAt=epoch.Add(2*time.Hour);if !errors.Is(ValidateAppend([]Source{a},b,catalogFixture()),ErrQuotaConflict){t.Fatal("overlap accepted")};end:=b.EffectiveAt;a.ExpiresAt=&end;if err:=ValidateAppend([]Source{a},b,catalogFixture());err!=nil{t.Fatal(err)}})
 t.Run("typed real future sources have separate semantics",func(t *testing.T){base:=source("plan-version",Quota,"devices.count",QuotaReplace);base.SourceKind=PlanSource;base.Limit.Value=10;addon:=add;addon.SourceKind=AddonSource;r:=resolveFixture(t,epoch,catalogFixture(),[]Source{grantModule(),base,addon});if findDecision(t,r,Quota,"devices.count","").Limit.Value!=17{t.Fatal(r)}})
}
func TestCE04FieldActionsAndSafetyMask(t *testing.T){
 read:=source("read",Field,"device.identity",Grant);read.Action="read"
 mask:=source("mask",Field,"device.identity",SafetyMask);mask.Action="read"
 r:=resolveFixture(t,epoch,catalogFixture(),[]Source{grantModule(),read,mask})
 if d:=findDecision(t,r,Field,"device.identity","read");!d.Allowed||!d.Masked{t.Fatal(d)}
 if d:=findDecision(t,r,Field,"device.identity","export");d.Allowed{t.Fatal(d)}
 if d:=findDecision(t,r,Field,"device.identity","write");d.Allowed{t.Fatal(d)}
 deny:=read;deny.ID="deny";deny.Effect=Deny;r=resolveFixture(t,epoch,catalogFixture(),[]Source{grantModule(),read,mask,deny});if d:=findDecision(t,r,Field,"device.identity","read");d.Allowed||!d.Masked{t.Fatal(d)}
}
func TestCE04DeterminismAndInvalidSources(t *testing.T){
 a:=grantModule();b:=source("read",Field,"device.identity",Grant);b.Action="read"
 x:=resolveFixture(t,epoch,catalogFixture(),[]Source{a,b});y:=resolveFixture(t,epoch,catalogFixture(),[]Source{b,a});j,_:=json.Marshal(x);k,_:=json.Marshal(y);if string(j)!=string(k){t.Fatal("source order changed result")}
 for _,tc:=range []struct{name string;mutate func(*Source)}{
  {"unknown effect",func(s *Source){s.Effect="allow_everything"}},
  {"invalid unlimited",func(s *Source){s.Kind=Quota;s.Key="devices.count";s.Effect=QuotaReplace;s.Limit=Limit{true,1}}},
  {"unlimited addition",func(s *Source){s.Kind=Quota;s.Key="devices.count";s.Effect=QuotaAdd;s.Limit.Unlimited=true}},
  {"empty reason",func(s *Source){s.Reason=" "}},
  {"precision would be truncated",func(s *Source){s.EffectiveAt=s.EffectiveAt.Add(time.Nanosecond)}},
  {"empty interval",func(s *Source){s.ExpiresAt=&s.EffectiveAt}},
  {"zero source version",func(s *Source){s.Version=0}},
  {"mask write forbidden",func(s *Source){s.Kind=Field;s.Key="device.identity";s.Action="write";s.Effect=SafetyMask}},
 }{t.Run(tc.name,func(t *testing.T){s:=a;tc.mutate(&s);if err:=s.Validate(catalogFixture());err==nil{t.Fatal("invalid source accepted")}})}
 t.Run("unknown capability cannot be persisted",func(t *testing.T){s:=source("unknown",Capability,"unknown.capability",Grant);if !errors.Is(s.Validate(catalogFixture()),ErrCatalog){t.Fatal("unknown accepted")}})
 t.Run("duplicate source cannot double quota",func(t *testing.T){s:=source("same",Quota,"devices.count",QuotaAdd);s.Limit.Value=1;if _,err:=Resolve("tenant-a",1,epoch,catalogFixture(),[]Source{s,s},nil);err==nil{t.Fatal("duplicate accepted")}})
}
