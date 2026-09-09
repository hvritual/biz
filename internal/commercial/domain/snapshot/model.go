// Package snapshot describes immutable derived commercial views. Sources and
// catalogue revisions, not cache entries, own authority.
package snapshot

import (
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "github.com/hvritual/biz/internal/commercial/domain/entitlement"
 "time"
)

var ErrStale = errors.New("ENTITLEMENT_SNAPSHOT_STALE")
var ErrInvalid = errors.New("ENTITLEMENT_SNAPSHOT_INVALID")
func Digest(b []byte) string { v:=sha256.Sum256(b); return hex.EncodeToString(v[:]) }
func InputHash(c entitlement.Catalog, sources []entitlement.Source)(string,error){
 b,err:=json.Marshal(struct{Catalog entitlement.Catalog;Sources []entitlement.Source}{c,sources});return Digest(b),err
}
func ValidAt(r entitlement.Result,at time.Time)bool{return !at.Before(r.EvaluatedAt)&&(r.ValidUntil==nil||at.Before(*r.ValidUntil))}
func Decode(b []byte,tenant string,version,source,catalog uint64,digest string,at time.Time)(entitlement.Result,error){
 if Digest(b)!=digest{return entitlement.Result{},ErrInvalid}
 var r entitlement.Result
 if err:=json.Unmarshal(b,&r);err!=nil{return r,ErrInvalid}
 if r.TenantID!=tenant||r.EntitlementVersion!=version||version==0||r.SourceVersion!=source||r.CatalogRevision!=catalog||r.ResolverVersion!=1||r.PermissionVersion!=""||r.PermissionSubject!=""{return entitlement.Result{},ErrInvalid}
 if !ValidAt(r,at){return entitlement.Result{},ErrStale};return r,nil
}
// The persisted view always contains every implemented capability. Unknown
// requested keys are appended as explicit denials, never new grants.
func Requested(r entitlement.Result,requested []string)entitlement.Result{
 seen:=map[string]bool{};for _,d:=range r.Decisions{if d.Kind==entitlement.Capability{seen[d.Key]=true}}
 for _,key:=range requested{if !seen[key]{r.Decisions=append(r.Decisions,entitlement.Decision{Kind:entitlement.Capability,Key:key,Allowed:false,Reason:"UNKNOWN_CAPABILITY"});seen[key]=true}};return r
}
