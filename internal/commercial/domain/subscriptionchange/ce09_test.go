package subscriptionchange

import (
 "errors"
 "testing"
 "time"
 "github.com/hvritual/biz/internal/commercial/domain/entitlement"
 "github.com/hvritual/biz/internal/commercial/domain/plan"
 "github.com/hvritual/biz/internal/commercial/domain/subscription"
)
func ce09Terms(value uint64,unlimited bool)plan.Terms{return plan.Terms{Modules:[]plan.Module{{Code:"device-operations",Capabilities:[]string{"device.lifecycle"},Quotas:[]plan.Quota{{Key:"tenant.devices",Value:value,Unlimited:unlimited}},Fields:[]plan.Field{{Key:"device.identity",Action:"read",Mode:"allow"}}}},SalesScope:[]string{"ce09"},ValidityMode:"fixed_days",ValidityDays:30}}
func ce09Version(code string,terms plan.Terms,at time.Time)plan.Version{return plan.Version{PlanCode:code,Number:1,Revision:1,PlanRevision:1,State:plan.Published,Name:code,Terms:terms,ContentSHA256:plan.Hash(code,terms),CreatedAt:at,PublishedAt:&at,ActorID:"platform",Reason:"qualification"}}
func TestCE09ClassificationIsConservativeAndZeroIsNotUnlimited(t *testing.T){
 a:=ce09Terms(10,false)
 for _,tc:=range []struct{name string;b plan.Terms;want string}{
  {"quota-up",ce09Terms(20,false),"UPGRADE"},{"zero",ce09Terms(0,false),"DOWNGRADE"},{"unlimited",ce09Terms(0,true),"UPGRADE"},{"same",ce09Terms(10,false),"SAME_TIER"},
 }{t.Run(tc.name,func(t *testing.T){if got:=Classify(a,tc.b);got!=tc.want{t.Fatalf("%s want %s",got,tc.want)}})}
 mixed:=ce09Terms(20,false);mixed.Modules[0].Fields[0].Mode="masked";if Classify(a,mixed)!="DOWNGRADE"{t.Fatal("mixed reduction misclassified")}
 if !Less(entitlement.Limit{},entitlement.Limit{Unlimited:true})||Less(entitlement.Limit{Unlimited:true},entitlement.Limit{}){t.Fatal("zero/unlimited order lost")}
}
func TestCE09PeriodsRequireRealCycleAndRespectExplicitTime(t *testing.T){
 now:=time.Date(2026,9,10,12,0,0,0,time.UTC);end:=now.AddDate(0,0,8);sub:=subscription.Subscription{PeriodEnd:&end};terms:=ce09Terms(5,false)
 mode,at,expires,err:=Period(Input{Action:Switch},sub,"DOWNGRADE",now,terms);if err!=nil||mode!=Scheduled||!at.Equal(end)||!expires.Equal(end.AddDate(0,0,30)){t.Fatalf("default cycle %s %s %v",mode,at,err)}
 _,_,_,err=Period(Input{Action:Switch},subscription.Subscription{},"DOWNGRADE",now,terms);if !errors.Is(err,ErrPeriod){t.Fatal("invented billing period")}
 future:=now.Add(time.Hour);mode,at,_,err=Period(Input{Action:Switch,EffectiveAt:&future},subscription.Subscription{},"DOWNGRADE",now,terms);if err!=nil||mode!=Scheduled||!at.Equal(future){t.Fatal("explicit schedule rejected")}
 _,_,_,err=Period(Input{Action:Switch,EffectiveAt:&now},sub,"UPGRADE",now,terms);if !errors.Is(err,ErrInvalid){t.Fatal("past/equal effective time accepted")}
 _,at,expires,err=Period(Input{Action:Renew},sub,Renew,now,terms);if err!=nil||!at.Equal(now)||!expires.Equal(end.AddDate(0,0,30)){t.Fatal("renewal shortened purchased duration")}
 _,_,expires,err=Period(Input{Action:StopRenewal},sub,StopRenewal,now,terms);if err!=nil||!expires.Equal(end){t.Fatal("stop shortened period")}
}
func TestCE09QuotaUnknownNeverBecomesZeroOrImmediateApproval(t *testing.T){
 requested:=QuotaChanges(ce09Terms(10,false),ce09Terms(2,false));report:=DefaultQuotaImpacts(requested)
 if report[0].UsageKnown||report[0].Used!=0||report[0].Evidence==""{t.Fatal("unknown usage not explicit")}
 deferred,err:=CheckQuotaReport(requested,report,Scheduled);if err!=nil||!deferred{t.Fatal("scheduled intent missing recheck")}
 _,err=CheckQuotaReport(requested,report,Immediate);if !errors.Is(err,ErrQuota){t.Fatal("unknown usage passed immediate reduction")}
 report[0].UsageKnown=true;report[0].Used=1;report[0].Evidence="trusted-local-count@version-4";report[0].Policy="CHECKED"
 if deferred,err=CheckQuotaReport(requested,report,Immediate);err!=nil||deferred{t.Fatal("trusted within-limit usage rejected")}
 report[0].Used=3;report[0].OverLimit=true;_,err=CheckQuotaReport(requested,report,Immediate);if !errors.Is(err,ErrQuota){t.Fatal("over-limit immediate write allowed")}
 report[0].After.Value=5;_,err=CheckQuotaReport(requested,report,Scheduled);if !errors.Is(err,ErrCorrupt){t.Fatal("adapter changed requested limit")}
}
func TestCE09ProjectionPreservesOverridesAndRevokedGenerations(t *testing.T){
 start:=time.Date(2026,9,1,0,0,0,0,time.UTC);end:=start.AddDate(0,0,30);old:=ce09Version("old",ce09Terms(10,false),start)
 sub:=subscription.Subscription{ID:"sub-1",TenantID:"tenant-1",PlanCode:"old",PlanVersion:1,Revision:1,SourceNamespace:"sub-1",PeriodStart:start,PeriodEnd:&end}
 sources:=subscription.Sources(sub.TenantID,sub.ID,start,old.Terms)
 override:=entitlement.Source{ID:"override-1",TenantID:sub.TenantID,SourceKind:entitlement.OverrideSource,ModuleCode:"device-operations",Kind:entitlement.Field,Key:"device.identity",Action:"read",Effect:entitlement.SafetyMask,EffectiveAt:start,Reason:"safety floor",ActorID:"platform",Version:1}
 sources=append(sources,override);original:=Digest(sources);target:=ce09Version("new",ce09Terms(20,false),start);at:=start.Add(time.Hour);newEnd:=at.AddDate(0,0,30)
 out,err:=ProjectSources(sub,old,target,sources,"chg-1",at,&newEnd);if err!=nil{t.Fatal(err)};if Digest(sources)!=original{t.Fatal("projection mutated caller sources")}
 var live,revoked int;found:=false
 for _,s:=range out{if s.ID==override.ID{found=Digest(s)==Digest(override)};if s.SourceKind==entitlement.PlanSource{if s.RevokedAt!=nil{revoked++;if s.Version!=2||!s.RevokedAt.Equal(at){t.Fatal("wrong revocation")}}else{live++}}}
 if !found||live!=4||revoked!=4{t.Fatalf("projection live=%d revoked=%d override=%v",live,revoked,found)}
 sub.PlanCode="new";sub.PeriodStart=at;sub.PeriodEnd=&newEnd;sub.SourceNamespace="chg-1"
 if err=ValidateCurrentSources(sub,target,out);err!=nil{t.Fatal(err)}
 for i:=range out{if out[i].SourceKind==entitlement.PlanSource&&out[i].RevokedAt==nil{out[i].Effect=entitlement.Deny;break}}
 if err=ValidateCurrentSources(sub,target,out);!errors.Is(err,ErrCorrupt){t.Fatal("altered PLAN authority accepted")}
}
func TestCE09LegacySubscriptionNormalizationKeepsHistoricalIdentity(t *testing.T){
 at:=time.Date(2026,8,1,0,0,0,0,time.UTC);v:=ce09Version("old",ce09Terms(10,false),at)
 old:=subscription.Subscription{ID:"sub-old",TenantID:"tenant-old",Kind:subscription.KindBase,State:subscription.StateActive,PlanCode:"old",PlanVersion:1,RuleID:"rule-old",RuleVersion:1,SalesScope:"ce09",EntitlementSourceVersion:1,CreatedAt:at,MatchExplanation:"old immutable result"}
 before:=Digest(old);normalized,err:=Normalize(old,v);if err!=nil{t.Fatal(err)}
 if before!=Digest(old)||normalized.ID!=old.ID||normalized.Revision!=1||normalized.SourceNamespace!=old.ID||!normalized.PeriodStart.Equal(at)||!normalized.PeriodEnd.Equal(at.AddDate(0,0,30)){t.Fatal("normalization rewrote history or invented period")}
}
func TestCE09RequestAndPreviewIdentityDoNotTrustClientState(t *testing.T){
 if InputError:= (Input{TenantID:"tenant",RequestID:"key",Action:StopRenewal,TargetPlanCode:"paid-plan",TargetPlanVersion:1,Reason:"x"}).Validate();!errors.Is(InputError,ErrInvalid){t.Fatal("stop accepted target")}
 if ID("a","t","k")==ID("b","t","k")||ID("a","t","k")==ID("a","u","k"){t.Fatal("preview identity lacks owner scope")}
 if (Input{TenantID:"tenant",RequestID:"key",Action:Switch,TargetPlanCode:"valid-plan",TargetPlanVersion:1,Reason:"manual"}).Validate()!=nil{t.Fatal("valid request rejected")}
}
