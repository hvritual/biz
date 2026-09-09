package modulecatalog
import("context";"github.com/hvritual/biz/internal/commercial/domain/entitlement";"github.com/hvritual/biz/internal/commercial/infrastructure/consistency";"gorm.io/gorm";"gorm.io/gorm/clause")
// ReadSnapshotCatalog is an owner-defined immutable projection on the supplied
// commercial transaction. Callers never access catalog tables/repositories.
func(s *Service)ReadSnapshotCatalog(ctx context.Context,tx *gorm.DB)(entitlement.Catalog,error){
 var rows []moduleRow
 if err:=tx.WithContext(ctx).Clauses(clause.Locking{Strength:"SHARE"}).Order("module_code").Find(&rows).Error;err!=nil{return nil,err}
 out:=make(entitlement.Catalog,0,len(rows))
 for _,row:=range rows{
 def,ok:=s.registry.Definition(row.Code);if !ok{return nil,ErrUnknownDefinition};status:=row.TechnicalStatus;if !def.ImplementationReady{status=string(TechnicalNotReady)}
 out=append(out,entitlement.ModuleDefinition{Code:row.Code,TechnicalStatus:status,SalesStatus:row.SalesStatus,Version:row.Version,Capabilities:append([]string(nil),def.CapabilityCodes...),QuotaKeys:append([]string(nil),def.QuotaSchemaKeys...),FieldKeys:append([]string(nil),def.FieldPolicySchemaKeys...),Dependencies:append([]string(nil),def.Dependencies...)})
 };return out,out.Validate()
}
func(s *Service)transact(ctx context.Context,fn func(*gorm.DB)error)error{return consistency.Within(ctx,s.store.db,0,func(tx *gorm.DB)error{if _,err:=consistency.LockCatalog(tx,true);err!=nil{return err};return fn(tx)})}
