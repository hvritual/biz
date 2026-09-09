package persistence

import (
 "context"
 "database/sql"
 "errors"
 "github.com/hvritual/biz/internal/commercial/domain/entitlement"
 "github.com/hvritual/biz/internal/commercial/ports"
 "gorm.io/gorm"
 "yunka.io/framework/execution"
 "yunka.io/framework/requestscope"
)

type currentEntitlements struct { db *gorm.DB }

func NewEntitlementStateReader(db *gorm.DB) (ports.EntitlementStateReader, error) {
 if db == nil { return nil, errors.New("commercial: authoritative database required") }
 return &currentEntitlements{db:db},nil
}

// One SELECT sees the aggregate version and every source in one statement
// snapshot. No cache, replica, write, or independently created root transaction.
// Within a child invocation, reads join its root UoW. This is not the CE-06
// read/write revocation barrier.
func (r *currentEntitlements) ReadCurrent(ctx context.Context, tenant string) (ports.EntitlementState, error) {
 if tenant == "" { return ports.EntitlementState{}, entitlement.ErrScope }
 if _,ok:=execution.Current(ctx);ok {
  return requestscope.JoinValue(ctx,NewEntitlementRepositoryFactory(),func(scope *requestscope.View[ports.EntitlementRepositories])(ports.EntitlementState,error){ return scope.Repositories().Entitlements.Read(scope.Context(),tenant) })
 }
 var rows []struct {
  StateVersion uint64
  ModuleCode sql.NullString
  SourceID sql.NullString
  Version sql.NullInt64
  Payload sql.NullString
 }
 err := r.db.WithContext(ctx).Raw(`SELECT s.version AS state_version, o.module_code, o.source_id, o.version, o.payload
 FROM biz_commercial_entitlement_state AS s LEFT JOIN biz_commercial_entitlement_sources AS o
 ON o.tenant_id=s.tenant_id WHERE s.tenant_id=? ORDER BY o.source_id`,tenant).Scan(&rows).Error
 if err != nil { return ports.EntitlementState{}, err }
 version:=uint64(0); decoded:=[]overrideRow{}
 for _,row:=range rows {
  version=row.StateVersion
  if !row.SourceID.Valid { continue }
  if !row.ModuleCode.Valid || !row.Version.Valid || row.Version.Int64<1 || !row.Payload.Valid { return ports.EntitlementState{},entitlement.ErrInvalid }
  decoded=append(decoded,overrideRow{TenantID:tenant,ID:row.SourceID.String,ModuleCode:row.ModuleCode.String,Version:uint64(row.Version.Int64),Payload:row.Payload.String})
 }
 return decodeSources(tenant,version,decoded)
}
