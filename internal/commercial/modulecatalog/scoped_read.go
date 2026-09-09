package modulecatalog

import (
 "context"
 "gorm.io/gorm"
 "yunka.io/framework/requestscope"
)

type catalogReadRepositories struct { db *gorm.DB }

// ListInScope is owned by ModuleCatalog and joins, rather than opens, the root
// UoW. EntitlementManagement consumes it only through a generated child port.
func(s *Service) ListInScope(ctx context.Context)([]Module,error){
 factory:=requestscope.GORMRepositories(func(_ context.Context,tx *gorm.DB)(catalogReadRepositories,error){return catalogReadRepositories{db:tx},nil})
 return requestscope.JoinValue(ctx,factory,func(scope *requestscope.View[catalogReadRepositories])([]Module,error){
  var rows []moduleRow
  if err:=scope.Repositories().db.WithContext(scope.Context()).Order("module_code ASC").Find(&rows).Error;err!=nil{return nil,err}
  result:=make([]Module,0,len(rows))
  for _,row:=range rows{def,ok:=s.registry.Definition(row.Code);if !ok{return nil,ErrUnknownDefinition};m:=rowToModule(row,def);if !def.ImplementationReady{m.TechnicalStatus=TechnicalNotReady};result=append(result,m)}
  return result,nil
 })
}
