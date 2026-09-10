// Package provisioning models durable preparation independently of effective
// subscription state. It does not execute networks, SQL, clocks or scripts.
package provisioning

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "regexp"
 "strings"
 "time"
)

const (
 Queued = "QUEUED"
 Running = "RUNNING"
 RetryWait = "RETRY_WAIT"
 Ready = "READY"
 Applied = "APPLIED"
 Failed = "FAILED"
 ReconcileRequired = "RECONCILIATION_REQUIRED"
 Cancelled = "CANCELLED"
 WaitingStep = "WAITING"
 RunningStep = "RUNNING"
 ReadyStep = "READY"
 Absent = "ABSENT"
 Unknown = "UNKNOWN"
 PrepareStage = "PREPARE"
 ReconcileStage = "RECONCILE"
 ActivateStage = "ACTIVATE"
 Retryable = "RETRYABLE"
 Permanent = "PERMANENT"
)

var (
 ErrInvalid = errors.New("PROVISIONING_INVALID_REQUEST")
 ErrScope = errors.New("PROVISIONING_PLATFORM_CONTEXT_REQUIRED")
 ErrNotFound = errors.New("PROVISIONING_TASK_NOT_FOUND")
 ErrConflict = errors.New("PROVISIONING_VERSION_CONFLICT")
 ErrKeyConflict = errors.New("PROVISIONING_REQUEST_CONFLICT")
 ErrLease = errors.New("PROVISIONING_LEASE_LOST")
 ErrCorrupt = errors.New("PROVISIONING_AUTHORITY_CORRUPT")
 ErrCancellation = errors.New("PROVISIONING_COMPENSATION_OR_RECONCILIATION_REQUIRED")
 ErrRetry = errors.New("PROVISIONING_RETRY_NOT_ALLOWED")
 ErrStale = errors.New("PROVISIONING_APPROVAL_STALE")
 ErrUnavailable = errors.New("PROVISIONING_ADAPTER_UNAVAILABLE")
)
var key = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
func Key(s string) bool { return key.MatchString(s) }
func Tenant(s string) bool { return len(s) <= 64 && Key(s) }
func Reason(s string) bool { return strings.TrimSpace(s) != "" && len(s) <= 512 }
func Digest(v any) string {
 b, _ := json.Marshal(v)
 h := sha256.Sum256(b)
 return hex.EncodeToString(h[:])
}
func TaskID(changeID string) string { return "job-" + Digest(changeID)[:48] }

// Requirement names a compiled server-side adapter and immutable contract
// version. There are no URLs, commands, credentials or client-defined scripts.
type Requirement struct {
 Code string `json:"code"`
 Adapter string `json:"adapter"`
 Version string `json:"version"`
 MaxAttempts uint32 `json:"max_attempts"`
}
func (r Requirement) Validate() error {
 if !Key(r.Code) || !Key(r.Adapter) || !Key(r.Version) || r.MaxAttempts < 1 || r.MaxAttempts > 10 { return ErrInvalid }
 return nil
}
func ValidateRequirements(rs []Requirement) error {
 if len(rs) > 16 { return ErrInvalid }
 seen := map[string]bool{}
 for _, r := range rs {
  if r.Validate() != nil || seen[r.Code] { return ErrInvalid }
  seen[r.Code] = true
 }
 return nil
}
type Step struct {
 Requirement Requirement `json:"requirement"`
 IdempotencyKey string `json:"idempotency_key"`
 State string `json:"state"`
 Effect string `json:"effect"`
 Attempts uint32 `json:"attempts"`
 CycleAttempts uint32 `json:"cycle_attempts"`
 Reconciliations uint32 `json:"reconciliations"`
 Evidence string `json:"evidence,omitempty"`
 FailureCode string `json:"failure_code,omitempty"`
}
type Approval struct {
 ChangeID string `json:"change_id"`
 ActorID string `json:"actor_id"`
 PreviewHash string `json:"preview_hash"`
 TargetHash string `json:"target_hash"`
 TargetPlanCode string `json:"target_plan_code"`
 TargetPlanVersion uint64 `json:"target_plan_version"`
 SubscriptionRevision uint64 `json:"subscription_revision"`
 SourceVersion uint64 `json:"source_version"`
 EntitlementVersion uint64 `json:"entitlement_version"`
 CatalogRevision uint64 `json:"catalog_revision"`
}
type Completion struct {
 ChangeID string `json:"change_id"`
 SubscriptionRevision uint64 `json:"subscription_revision"`
 SourceVersion uint64 `json:"source_version"`
 EntitlementVersion uint64 `json:"entitlement_version"`
 AppliedAt time.Time `json:"applied_at"`
}
type Task struct {
 ID string `json:"task_id"`
 TenantID string `json:"tenant_id"`
 Approval Approval `json:"approval"`
 Revision uint64 `json:"revision"`
 State string `json:"state"`
 Steps []Step `json:"steps"`
 StepIndex int `json:"step_index"`
 Stage string `json:"stage"`
 NextAttemptAt time.Time `json:"next_attempt_at"`
 CreatedAt time.Time `json:"created_at"`
 UpdatedAt time.Time `json:"updated_at"`
 Deadline time.Time `json:"deadline"`
 LeaseOwner string `json:"lease_owner,omitempty"`
 LeaseToken uint64 `json:"lease_token"`
 LeaseUntil *time.Time `json:"lease_until,omitempty"`
 FailureCode string `json:"failure_code,omitempty"`
 RetryAllowed bool `json:"retry_allowed"`
 RetryCycles uint32 `json:"retry_cycles"`
 ActivationAttempts uint32 `json:"activation_attempts"`
 Completion *Completion `json:"completion,omitempty"`
 Hash string `json:"hash"`
}
func New(tenant string, a Approval, rs []Requirement, now time.Time) (Task, error) {
 t := Task{ID: TaskID(a.ChangeID), TenantID: tenant, Approval: a, Revision: 1, State: Queued, Stage: PrepareStage, CreatedAt: now, UpdatedAt: now, NextAttemptAt: now, Deadline: now.Add(24*time.Hour)}
 if len(rs) == 0 || ValidateRequirements(rs) != nil { return t, ErrInvalid }
 for i, r := range rs { t.Steps = append(t.Steps, Step{Requirement:r, IdempotencyKey:"prep-"+Digest([]any{t.ID,i,r})[:48], State:WaitingStep, Effect:Absent}) }
 t = t.Seal()
 return t, t.Integrity()
}
func (t Task) Seal() Task { t.Hash = ""; t.Hash = Digest(t); return t }
func (t Task) Integrity() error {
 if !Tenant(t.TenantID) || t.ID != TaskID(t.Approval.ChangeID) || !Key(t.Approval.ChangeID) || t.Revision == 0 || t.Approval.ActorID == "" || len(t.Approval.PreviewHash) != 64 || len(t.Approval.TargetHash) != 64 || !Key(t.Approval.TargetPlanCode) || t.Approval.TargetPlanVersion == 0 || t.Approval.SubscriptionRevision == 0 || t.Approval.SourceVersion == 0 || t.Approval.EntitlementVersion == 0 || t.Approval.CatalogRevision == 0 || len(t.Steps) < 1 || len(t.Steps) > 16 || t.StepIndex < 0 || t.StepIndex > len(t.Steps) || t.CreatedAt.IsZero() || !t.Deadline.After(t.CreatedAt) || t.Hash != t.Seal().Hash { return ErrCorrupt }
 switch t.State { case Queued,Running,RetryWait,Ready,Applied,Failed,ReconcileRequired,Cancelled: default: return ErrCorrupt }
 for i,s := range t.Steps {
  if s.Requirement.Validate()!=nil || s.IdempotencyKey!="prep-"+Digest([]any{t.ID,i,s.Requirement})[:48] || s.CycleAttempts>s.Attempts || s.CycleAttempts>s.Requirement.MaxAttempts || len(s.Evidence)>1024 || len(s.FailureCode)>128 { return ErrCorrupt }
  switch s.State { case WaitingStep,RunningStep,ReadyStep: default: return ErrCorrupt }
  switch s.Effect { case Absent,Unknown,ReadyStep: default: return ErrCorrupt }
  if i<t.StepIndex && (s.State!=ReadyStep || s.Effect!=ReadyStep || s.Evidence=="") { return ErrCorrupt }
 }
 if t.State==Running && (t.LeaseOwner=="" || t.LeaseUntil==nil || t.LeaseToken==0) { return ErrCorrupt }
 if t.State!=Running && (t.LeaseOwner!="" || t.LeaseUntil!=nil) { return ErrCorrupt }
 if t.State==Applied && (t.Completion==nil || t.Completion.ChangeID!=t.Approval.ChangeID || t.Completion.EntitlementVersion==0 || t.Completion.SourceVersion==0 || t.Completion.SubscriptionRevision!=t.Approval.SubscriptionRevision+1) { return ErrCorrupt }
 return nil
}
func (t Task) Terminal() bool { return t.State==Applied || t.State==Cancelled }
func (t Task) Due(now time.Time) bool {
 if t.State==Running { return t.LeaseUntil!=nil && !now.Before(*t.LeaseUntil) }
 return (t.State==Queued || t.State==RetryWait || t.State==Ready) && !now.Before(t.NextAttemptAt)
}
func (t *Task) release() { t.LeaseOwner=""; t.LeaseUntil=nil }
func (t *Task) finishRevision(now time.Time) { t.Revision++; t.UpdatedAt=now; *t=t.Seal() }
func (t *Task) Claim(owner string, now time.Time, lease time.Duration) error {
 if !Key(owner) || lease<5*time.Second || lease>5*time.Minute || !t.Due(now) || t.Revision==^uint64(0) || t.LeaseToken==^uint64(0) { return ErrConflict }
 if !now.Before(t.Deadline) {
  t.State=ReconcileRequired; t.FailureCode="APPROVAL_DEADLINE_EXCEEDED"; t.RetryAllowed=false; t.release(); t.finishRevision(now); return nil
 }
 expired:=t.State==Running
 if t.StepIndex==len(t.Steps) {
  t.Stage=ActivateStage; t.ActivationAttempts++
  if t.ActivationAttempts>3 { t.State=Failed; t.FailureCode="ACTIVATION_RETRIES_EXHAUSTED"; t.RetryAllowed=true; t.release(); t.finishRevision(now); return nil }
 } else {
  s:=&t.Steps[t.StepIndex]
  if expired || s.Effect==Unknown {
   if s.Reconciliations>=32 { t.State=ReconcileRequired; t.FailureCode="RECONCILIATION_LIMIT_REACHED"; t.RetryAllowed=false; t.release(); t.finishRevision(now); return nil }
   t.Stage=ReconcileStage; s.Reconciliations++
  } else {
   if s.CycleAttempts>=s.Requirement.MaxAttempts { t.State=Failed; t.FailureCode="PREPARATION_RETRIES_EXHAUSTED"; t.RetryAllowed=true; t.release(); t.finishRevision(now); return nil }
   t.Stage=PrepareStage; s.Attempts++; s.CycleAttempts++
  }
  s.State=RunningStep; s.Effect=Unknown
 }
 t.State=Running; t.LeaseOwner=owner; t.LeaseToken++
 until:=now.Add(lease); t.LeaseUntil=&until; t.finishRevision(now)
 return nil
}
func (t Task) Owns(owner string, token uint64, now time.Time) bool { return t.State==Running && t.LeaseOwner==owner && t.LeaseToken==token && t.LeaseUntil!=nil && now.Before(*t.LeaseUntil) }

// RETRYABLE/PERMANENT require positive absence evidence. UNKNOWN stops
// automatic attempts. READY requires a durable provider reference.
type Observation struct { Outcome string; Evidence string; FailureCode string }
func (o Observation) Validate() error {
 if len(o.Evidence)>1024 || strings.TrimSpace(o.Evidence)=="" || len(o.FailureCode)>128 { return ErrInvalid }
 switch o.Outcome { case ReadyStep,Absent,Retryable,Permanent,Unknown: default:return ErrInvalid }
 if o.Outcome!=ReadyStep && o.Outcome!=Absent && !Key(o.FailureCode) { return ErrInvalid }
 return nil
}
func Backoff(n uint32) time.Duration { if n>6 {n=6}; return time.Duration(1<<n)*time.Second }
func (t *Task) Observe(owner string, token uint64, o Observation, now time.Time) error {
 if !t.Owns(owner,token,now) { return ErrLease }
 if o.Validate()!=nil { return ErrInvalid }
 t.FailureCode=o.FailureCode; t.RetryAllowed=false
 if t.Stage==ActivateStage {
  if o.Outcome==Retryable { t.State=RetryWait; t.NextAttemptAt=now.Add(Backoff(t.ActivationAttempts)); t.RetryAllowed=true } else { t.State=ReconcileRequired }
  t.release(); t.finishRevision(now); return nil
 }
 s:=&t.Steps[t.StepIndex]; s.Evidence=o.Evidence; s.FailureCode=o.FailureCode
 switch o.Outcome {
 case ReadyStep:
  s.State=ReadyStep; s.Effect=ReadyStep; t.StepIndex++; t.State=Queued; t.NextAttemptAt=now
  if t.StepIndex==len(t.Steps) { t.State=Ready; t.Stage=ActivateStage }
 case Absent,Retryable:
  s.State=WaitingStep; s.Effect=Absent; t.RetryAllowed=true
  if s.CycleAttempts>=s.Requirement.MaxAttempts {t.State=Failed; t.FailureCode="PREPARATION_RETRIES_EXHAUSTED"} else {t.State=RetryWait; t.NextAttemptAt=now.Add(Backoff(s.CycleAttempts))}
 case Permanent: s.State=WaitingStep; s.Effect=Absent; t.State=Failed
 case Unknown: s.State=WaitingStep; s.Effect=Unknown; t.State=ReconcileRequired; t.RetryAllowed=true
 }
 t.release(); t.finishRevision(now); return nil
}
func (t Task) Cancellable() bool {
 if t.State==Running || t.Terminal() { return false }
 for _,s:=range t.Steps { if s.Effect!=Absent || s.State==ReadyStep {return false} }
 return true
}
func (t *Task) Cancel(now time.Time) error {
 if !t.Cancellable() { return ErrCancellation }
 t.State=Cancelled; t.RetryAllowed=false; t.release(); t.finishRevision(now); return nil
}
func (t *Task) Retry(now time.Time) error {
 if (t.State!=Failed && t.State!=ReconcileRequired) || !t.RetryAllowed || !now.Before(t.Deadline) || t.RetryCycles>=10 {return ErrRetry}
 t.RetryCycles++; t.ActivationAttempts=0
 if t.StepIndex<len(t.Steps) {t.Steps[t.StepIndex].CycleAttempts=0; t.Steps[t.StepIndex].State=WaitingStep}
 t.State=Queued; t.NextAttemptAt=now; t.RetryAllowed=false; t.release(); t.finishRevision(now); return nil
}
func (t *Task) Complete(owner string, token uint64, c Completion, now time.Time) error {
 if !t.Owns(owner,token,now) || t.Stage!=ActivateStage || t.StepIndex!=len(t.Steps) {return ErrLease}
 if c.ChangeID!=t.Approval.ChangeID || c.SubscriptionRevision!=t.Approval.SubscriptionRevision+1 || c.SourceVersion<=t.Approval.SourceVersion || c.EntitlementVersion<=t.Approval.EntitlementVersion || c.AppliedAt.IsZero() {return ErrCorrupt}
 t.State=Applied; t.Completion=&c; t.RetryAllowed=false; t.FailureCode=""; t.release(); t.finishRevision(now); return t.Integrity()
}
func (t Task) Clone() Task { t.Steps=append([]Step(nil),t.Steps...); if t.Completion!=nil {c:=*t.Completion; t.Completion=&c}; return t }
