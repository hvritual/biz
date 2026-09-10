package provisioning

import "time"

// Event is an internal immutable subscription fact, not a client command.
type Event struct {
 ID string `json:"event_id"`
 TenantID string `json:"tenant_id"`
 AggregateID string `json:"aggregate_id"`
 AggregateVersion uint64 `json:"aggregate_version"`
 ChangeID string `json:"change_id"`
 TaskID string `json:"task_id,omitempty"`
 Status string `json:"status"`
 SourceVersion uint64 `json:"source_version"`
 EntitlementVersion uint64 `json:"entitlement_version"`
 OccurredAt time.Time `json:"occurred_at"`
 Hash string `json:"hash"`
}
func (e Event) Seal() Event {
 e.ID="evt-"+Digest([]any{e.TenantID,e.AggregateID,e.AggregateVersion})[:48]
 e.Hash=""; e.Hash=Digest(e); return e
}
func (e Event) Integrity() error {
 sealed:=e.Seal()
 if !Tenant(e.TenantID)||!Key(e.AggregateID)||e.AggregateVersion==0||!Key(e.ChangeID)||e.OccurredAt.IsZero()||e.SourceVersion==0||e.EntitlementVersion==0||sealed.ID!=e.ID||sealed.Hash!=e.Hash {return ErrCorrupt}
 switch e.Status {case Applied,"SCHEDULED","PROVISIONING",Cancelled: default:return ErrCorrupt}
 return nil
}
type Delivery struct {
 ID string
 Event Event
 Attempts uint32
 State string
 FailureCode string
 NextAttemptAt time.Time
 LeaseOwner string
 LeaseToken uint64
 LeaseUntil *time.Time
}
type DeliveryReceipt struct {
 EventID string
 Outcome string
 AggregateVersion uint64
 EntitlementVersion uint64
}
// Exact duplicates are harmless; a reused ID with different facts is corrupt.
// Older revisions are recorded STALE and never replace newer projections.
func ClassifyDelivery(e Event,inboxHash string,lastVersion uint64,lastHash string)(string,error){
 if e.Integrity()!=nil{return "",ErrCorrupt}
 if inboxHash!="" {if inboxHash!=e.Hash{return "",ErrCorrupt};return "DUPLICATE",nil}
 if e.AggregateVersion<lastVersion{return "STALE",nil}
 if e.AggregateVersion==lastVersion {if e.Hash!=lastHash{return "",ErrCorrupt};return "DUPLICATE",nil}
 return "APPLIED",nil
}
