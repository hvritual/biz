// Package subscriptionchange owns deterministic plan-change decisions. It has
// no transport, database, identity resolver, wall clock, payment or scheduler.
package subscriptionchange

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "regexp"
 "strings"
 "time"

 "github.com/hvritual/biz/internal/commercial/domain/entitlement"
 "github.com/hvritual/biz/internal/commercial/domain/plan"
 "github.com/hvritual/biz/internal/commercial/domain/subscription"
)

const (
 Switch = "SWITCH"
 Renew = "RENEW"
 StopRenewal = "STOP_RENEWAL"
 Immediate = "IMMEDIATE"
 Scheduled = "SCHEDULED"
 Applied = "APPLIED"
)
var (
 ErrInvalid = errors.New("SUBSCRIPTION_CHANGE_INVALID_REQUEST")
 ErrConflict = errors.New("SUBSCRIPTION_CHANGE_VERSION_CONFLICT")
 ErrRequestConflict = errors.New("SUBSCRIPTION_CHANGE_REQUEST_CONFLICT")
 ErrExpired = errors.New("SUBSCRIPTION_CHANGE_PREVIEW_EXPIRED")
 ErrNotFound = errors.New("SUBSCRIPTION_CHANGE_NOT_FOUND")
 ErrTarget = errors.New("SUBSCRIPTION_CHANGE_TARGET_NOT_ELIGIBLE")
 ErrScope = errors.New("SUBSCRIPTION_CHANGE_PLATFORM_APPROVAL_REQUIRED")
 ErrPending = errors.New("SUBSCRIPTION_CHANGE_ALREADY_PENDING")
 ErrPeriod = errors.New("SUBSCRIPTION_CHANGE_NEXT_PERIOD_UNDEFINED")
 ErrQuota = errors.New("SUBSCRIPTION_CHANGE_QUOTA_VALIDATION_REQUIRED")
 ErrCorrupt = errors.New("SUBSCRIPTION_CHANGE_AUTHORITY_CORRUPT")
)
var keyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
func Key(s string) bool { return keyPattern.MatchString(s) }
func Tenant(s string) bool { return len(s)<=64 && Key(s) }
func Reason(s string) bool { return strings.TrimSpace(s)!="" && len(s)<=512 }
func Digest(v any) string { b,_:=json.Marshal(v); h:=sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func ID(actor,tenant,key string) string { return "chg-"+Digest([]string{actor,tenant,key})[:48] }

type Input struct {
 TenantID string `json:"tenant_id"`
 RequestID string `json:"request_id"`
 Action string `json:"action"`
 TargetPlanCode string `json:"target_plan_code"`
 TargetPlanVersion uint64 `json:"target_plan_version"`
 EffectiveAt *time.Time `json:"effective_at,omitempty"`
 Reason string `json:"reason"`
}
func (i Input) Validate() error {
 if !Tenant(i.TenantID)||!Key(i.RequestID)||!Reason(i.Reason) {return ErrInvalid}
 switch i.Action {
 case Switch,Renew:
  if !plan.Code(i.TargetPlanCode)||i.TargetPlanVersion==0{return ErrInvalid}
 case StopRenewal:
  if i.TargetPlanCode!=""||i.TargetPlanVersion!=0||i.EffectiveAt!=nil{return ErrInvalid}
 default:return ErrInvalid
 }
 return nil
}
type Dependency struct { ModuleCode string `json:"module_code"`; Requires []string `json:"requires"` }
type QuotaImpact struct {
 ModuleCode string `json:"module_code"`
 Key string `json:"key"`
 Before entitlement.Limit `json:"before"`
 After entitlement.Limit `json:"after"`
 UsageKnown bool `json:"usage_known"`
 Used uint64 `json:"used"`
 OverLimit bool `json:"over_limit"`
 Policy string `json:"policy"`
 Evidence string `json:"evidence"`
}
type Preview struct {
 ChangeID string `json:"change_id"`
 ActorID string `json:"actor_id"`
 Input Input `json:"input"`
 Fingerprint string `json:"fingerprint"`
 Hash string `json:"hash"`
 Before subscription.Subscription `json:"before"`
 Target plan.Version `json:"target"`
 Current entitlement.Result `json:"current"`
 Projected entitlement.Result `json:"projected"`
 Classification string `json:"classification"`
 Mode string `json:"mode"`
 CreatedAt time.Time `json:"created_at"`
 ExpiresAt time.Time `json:"expires_at"`
 EffectiveAt time.Time `json:"effective_at"`
 EntitlementExpiresAt *time.Time `json:"entitlement_expires_at,omitempty"`
 Dependencies []Dependency `json:"dependencies"`
 Quotas []QuotaImpact `json:"quotas"`
 Impacts []string `json:"impacts"`
 QuotaValidationRequired bool `json:"quota_validation_required"`
 PricingBasis string `json:"pricing_basis"`
}
func (p Preview) Seal() Preview {p.Hash="";p.Hash=Digest(p);return p}
func (p Preview) Integrity() error {
 if p.Input.Validate()!=nil||p.ActorID==""||p.ChangeID!=ID(p.ActorID,p.Input.TenantID,p.Input.RequestID)||p.Fingerprint!=Digest(p.Input)||!p.CreatedAt.Before(p.ExpiresAt)||p.Hash!=p.Seal().Hash||p.Before.TenantID!=p.Input.TenantID||p.Before.Revision==0||p.Current.SourceVersion==0||p.Current.EntitlementVersion==0||p.Target.Integrity()!=nil {return ErrCorrupt}
 return nil
}
type Receipt struct {
 ChangeID string `json:"change_id"`
 TenantID string `json:"tenant_id"`
 ActorID string `json:"actor_id"`
 RequestID string `json:"request_id"`
 Fingerprint string `json:"fingerprint"`
 PreviewHash string `json:"preview_hash"`
 Hash string `json:"hash"`
 Action string `json:"action"`
 Status string `json:"status"`
 Mode string `json:"mode"`
 ConfirmedAt time.Time `json:"confirmed_at"`
 EffectiveAt time.Time `json:"effective_at"`
 EntitlementExpiresAt *time.Time `json:"entitlement_expires_at,omitempty"`
 Reason string `json:"reason"`
 Before subscription.Subscription `json:"before"`
 After subscription.Subscription `json:"after"`
 BeforeSourceVersion uint64 `json:"before_source_version"`
 AfterSourceVersion uint64 `json:"after_source_version"`
 BeforeEntitlementVersion uint64 `json:"before_entitlement_version"`
 AfterEntitlementVersion uint64 `json:"after_entitlement_version"`
 QuotaValidationRequired bool `json:"quota_validation_required"`
 PricingAuthority string `json:"pricing_authority"`
 Quotas []QuotaImpact `json:"quotas"`
}
func (r Receipt) Seal() Receipt {r.Hash="";r.Hash=Digest(r);return r}
func (r Receipt) Integrity() error {
 if !Tenant(r.TenantID)||!Key(r.RequestID)||!Key(r.ChangeID)||r.ActorID==""||r.Hash!=r.Seal().Hash||r.Before.TenantID!=r.TenantID||r.After.TenantID!=r.TenantID||r.After.Revision!=r.Before.Revision+1||r.After.Revision==0||r.ConfirmedAt.IsZero()||len(r.PreviewHash)!=64||(r.Status!=Applied&&r.Status!=Scheduled) {return ErrCorrupt}
 return nil
}

// Normalize supplies a version/period for the append-only CE-08 payload format.
// It never rewrites historical receipts and never invents a billing month.
func Normalize(s subscription.Subscription, p plan.Version) (subscription.Subscription,error) {
 if s.PlanCode!=p.PlanCode||s.PlanVersion!=p.Number||s.Validate()!=nil {return s,ErrCorrupt}
 if s.Revision==0 {
  s.Revision=1;s.PeriodStart=s.CreatedAt
  if p.Terms.ValidityMode=="fixed_days" {end:=s.CreatedAt.AddDate(0,0,int(p.Terms.ValidityDays));s.PeriodEnd=&end}
 }
 if s.SourceNamespace=="" {s.SourceNamespace=s.ID}
 if s.PeriodStart.IsZero()||(s.PeriodEnd!=nil&&!s.PeriodEnd.After(s.PeriodStart)) {return s,ErrCorrupt}
 return s,nil
}
func Period(i Input,before subscription.Subscription,classification string,now time.Time,target plan.Terms) (string,time.Time,*time.Time,error) {
 mode:=Immediate;at:=now
 if i.EffectiveAt!=nil {
  if !i.EffectiveAt.After(now)||i.EffectiveAt.After(now.AddDate(100,0,0)) {return "",at,nil,ErrInvalid}
  mode=Scheduled;at=i.EffectiveAt.UTC()
 } else if i.Action==Switch&&classification=="DOWNGRADE" {
  if before.PeriodEnd==nil {return "",at,nil,ErrPeriod}
  if before.PeriodEnd.After(now) {mode=Scheduled;at=*before.PeriodEnd}
 }
 if i.Action==StopRenewal{return Immediate,now,before.PeriodEnd,nil}
 var end *time.Time
 if i.Action==Renew {
  if target.ValidityMode!="fixed_days"||before.PeriodEnd==nil {return "",at,nil,ErrPeriod}
  base:=at;if before.PeriodEnd.After(base) {base=*before.PeriodEnd}
  v:=base.AddDate(0,0,int(target.ValidityDays));end=&v
 } else if target.ValidityMode=="fixed_days" {v:=at.AddDate(0,0,int(target.ValidityDays));end=&v}
 return mode,at,end,nil
}

// Classify compares authoritative terms, not a client tier label or price.
// Mixed additions and removals are conservatively a downgrade.
func Classify(a,b plan.Terms) string {
 features:=func(t plan.Terms) map[string]int {
  out:=map[string]int{}
  for _,m:=range t.Modules {out["m/"+m.Code]=1;for _,c:=range m.Capabilities {out["c/"+m.Code+"/"+c]=1};for _,f:=range m.Fields {rank:=map[string]int{"deny":0,"masked":1,"allow":2}[f.Mode];out["f/"+m.Code+"/"+f.Key+"/"+f.Action]=rank}}
  return out
 }
 old,next:=features(a),features(b);up,down:=false,false
 for k,v:=range old {if next[k]<v{down=true};if next[k]>v{up=true}}
 for k,v:=range next {if v>old[k]{up=true}}
 for _,q:=range QuotaChanges(a,b) {if Less(q.After,q.Before){down=true}else{up=true}}
 if a.ValidityMode=="unlimited"&&b.ValidityMode!="unlimited" {down=true}
 if a.ValidityMode!="unlimited"&&b.ValidityMode=="unlimited" {up=true}
 if a.ValidityMode=="fixed_days"&&b.ValidityMode=="fixed_days" {if b.ValidityDays<a.ValidityDays {down=true};if b.ValidityDays>a.ValidityDays {up=true}}
 if down{return "DOWNGRADE"};if up{return "UPGRADE"};return "SAME_TIER"
}
func Less(a,b entitlement.Limit) bool {return !a.Unlimited&&(b.Unlimited||a.Value<b.Value)}
func QuotaChanges(a,b plan.Terms) []QuotaImpact {
 old,next:=map[string]entitlement.Limit{},map[string]entitlement.Limit{};keys:=map[string]QuotaImpact{}
 add:=func(t plan.Terms,values map[string]entitlement.Limit){for _,m:=range t.Modules {for _,q:=range m.Quotas {k:=m.Code+"/"+q.Key;values[k]=entitlement.Limit{Unlimited:q.Unlimited,Value:q.Value};keys[k]=QuotaImpact{ModuleCode:m.Code,Key:q.Key}}}}
 add(a,old);add(b,next);out:=[]QuotaImpact{}
 for _,k:=range sortedKeys(keys) {q:=keys[k];q.Before=old[k];q.After=next[k];if q.Before!=q.After {out=append(out,q)}}
 return out
}
