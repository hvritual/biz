// Package provisioningworker selects local preparation contracts, not scripts.
package provisioningworker

import (
 "context"
 "fmt"
 "sort"
 "github.com/hvritual/biz/internal/commercial/domain/plan"
 p "github.com/hvritual/biz/internal/commercial/domain/provisioning"
)
type ModulePreparation struct {ModuleCode string; Steps []p.Requirement}
type ModulePolicy struct {Modules []ModulePreparation}
func(m ModulePolicy)Requirements(_ context.Context,v plan.Version)([]p.Requirement,error){
 selected:=map[string]bool{}
 for _,module:=range v.Terms.Modules {selected[module.Code]=true}
 modules:=append([]ModulePreparation(nil),m.Modules...)
 sort.Slice(modules,func(i,j int)bool{return modules[i].ModuleCode<modules[j].ModuleCode})
 out:=[]p.Requirement{};seen:=map[string]bool{}
 for _,module:=range modules {
  if !plan.Code(module.ModuleCode)||seen[module.ModuleCode]{return nil,fmt.Errorf("provisioning: duplicate or invalid module policy")}
  seen[module.ModuleCode]=true
  if p.ValidateRequirements(module.Steps)!=nil{return nil,p.ErrInvalid}
  if selected[module.ModuleCode]{out=append(out,module.Steps...)}
 }
 if p.ValidateRequirements(out)!=nil{return nil,p.ErrInvalid}
 if len(out)==0{return nil,nil}
 return out,nil
}
