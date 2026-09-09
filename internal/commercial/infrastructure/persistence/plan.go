package persistence

import (
 "context"
 "encoding/json"
 "errors"
 "time"
 "github.com/hvritual/biz/internal/commercial/domain/plan"
 "github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
 "github.com/hvritual/biz/internal/commercial/ports"
 "gorm.io/gorm"
 "gorm.io/gorm/clause"
 "yunka.io/framework/requestscope"
)

type planRow struct{PlanCode string `gorm:"column:plan_code;primaryKey"`;Revision uint64;LatestVersion uint64}
func(planRow)TableName()string{return "biz_commercial_plans"}
type planVersionRow struct{PlanCode string `gorm:"column:plan_code;primaryKey"`;Version uint64 `gorm:"primaryKey"`;Revision uint64;State string;ContentSHA256 string `gorm:"column:content_sha256"`;Payload string}
func(planVersionRow)TableName()string{return "biz_commercial_plan_versions"}
type planRefRow struct{PlanCode string `gorm:"column:plan_code;primaryKey"`;Version uint64 `gorm:"primaryKey"`;ModuleCode string `gorm:"column:module_code;primaryKey"`}
func(planRefRow)TableName()string{return "biz_commercial_plan_module_refs"}
type planReceiptRow struct{PlanCode string `gorm:"column:plan_code;primaryKey"`;RequestID string `gorm:"column:request_id;primaryKey"`;Fingerprint string;Payload string}
func(planReceiptRow)TableName()string{return "biz_commercial_plan_receipts"}
type planAuditRow struct{ID uint64 `gorm:"primaryKey;autoIncrement"`;PlanCode string;Version uint64;RequestID string;Action string;ActorID string;Reason string;BeforeJSON string `gorm:"column:before_json"`;AfterJSON string `gorm:"column:after_json"`;CreatedAt time.Time}
func(planAuditRow)TableName()string{return "biz_commercial_plan_audit"}
type planRepository struct{tx *gorm.DB}
func NewPlanRepositoryFactory()requestscope.RepositoryFactory[ports.PlanRepositories]{
 return requestscope.GORMRepositories(func(_ context.Context,tx *gorm.DB)(ports.PlanRepositories,error){
  if tx==nil{return ports.PlanRepositories{},errors.New("plans: root transaction required")}
  return ports.PlanRepositories{Plans:&planRepository{tx:tx.Session(&gorm.Session{SkipDefaultTransaction:true})}},nil
 })
}
func(r *planRepository)Now(ctx context.Context)(time.Time,error){return consistency.Now(r.tx.WithContext(ctx))}
func(r *planRepository)Lock(ctx context.Context,code string)(ports.PlanHead,error){
 db:=r.tx.WithContext(ctx)
 if err:=db.Clauses(clause.OnConflict{DoNothing:true}).Create(&planRow{PlanCode:code}).Error;err!=nil{return ports.PlanHead{},err}
 var row planRow
 if err:=db.Clauses(clause.Locking{Strength:"UPDATE"}).Where("plan_code=?",code).First(&row).Error;err!=nil{return ports.PlanHead{},err}
 return ports.PlanHead{Code:code,Revision:row.Revision,Latest:row.LatestVersion},nil
}
func decodePlan(row planVersionRow)(plan.Version,error){
 var v plan.Version;if err:=json.Unmarshal([]byte(row.Payload),&v);err!=nil{return v,plan.ErrCorrupt}
 if v.PlanCode!=row.PlanCode||v.Number!=row.Version||v.Revision!=row.Revision||v.State!=row.State||v.ContentSHA256!=row.ContentSHA256{return v,plan.ErrCorrupt}
 return v,v.Integrity()
}
func(r *planRepository)Get(ctx context.Context,code string,number uint64,current bool)(plan.Version,error){
 db:=r.tx.WithContext(ctx);if current{db=db.Clauses(clause.Locking{Strength:"SHARE"})};var row planVersionRow
 if err:=db.Where("plan_code=? AND version=?",code,number).First(&row).Error;err!=nil{if errors.Is(err,gorm.ErrRecordNotFound){err=plan.ErrNotFound};return plan.Version{},err};return decodePlan(row)
}
func(r *planRepository)List(ctx context.Context,code string,after uint64,limit int)([]plan.Version,error){
 var rows []planVersionRow;if err:=r.tx.WithContext(ctx).Where("plan_code=? AND version>?",code,after).Order("version ASC").Limit(limit).Find(&rows).Error;err!=nil{return nil,err}
 out:=make([]plan.Version,0,len(rows));for _,row:=range rows{v,err:=decodePlan(row);if err!=nil{return nil,err};out=append(out,v)};return out,nil
}
func(r *planRepository)Receipt(ctx context.Context,code,key,hash string)(*plan.Version,error){
 var row planReceiptRow;err:=r.tx.WithContext(ctx).Clauses(clause.Locking{Strength:"UPDATE"}).Where("plan_code=? AND request_id=?",code,key).First(&row).Error
 if errors.Is(err,gorm.ErrRecordNotFound){return nil,nil};if err!=nil{return nil,err};if row.Fingerprint!=hash{return nil,plan.ErrRequestConflict}
 var out plan.Version;if err:=json.Unmarshal([]byte(row.Payload),&out);err!=nil{return nil,plan.ErrCorrupt};if out.PlanCode!=code{return nil,plan.ErrCorrupt};if err:=out.Integrity();err!=nil{return nil,err};return &out,nil
}
func(r *planRepository)Save(ctx context.Context,head ports.PlanHead,before *plan.Version,v plan.Version)error{
 if err:=v.Integrity();err!=nil{return err};if head.Code!=v.PlanCode||head.Revision==^uint64(0)||v.PlanRevision!=head.Revision+1{return plan.ErrConflict}
 if before!=nil {
  if before.Revision==^uint64(0)||v.Revision!=before.Revision+1{return plan.ErrConflict}
  if before.State!=plan.Draft && !(before.State==plan.Published&&v.State==plan.Retired&&before.ContentSHA256==v.ContentSHA256){return plan.ErrImmutable}
 }else if head.Latest==^uint64(0)||v.Number!=head.Latest+1||v.Revision!=1||v.State!=plan.Draft{return plan.ErrConflict}
 b,err:=json.Marshal(v);if err!=nil{return err};db:=r.tx.WithContext(ctx)
 row:=planVersionRow{PlanCode:v.PlanCode,Version:v.Number,Revision:v.Revision,State:v.State,ContentSHA256:v.ContentSHA256,Payload:string(b)}
 if before==nil{err=db.Create(&row).Error}else{
  res:=db.Model(&planVersionRow{}).Where("plan_code=? AND version=? AND revision=? AND state=?",v.PlanCode,v.Number,before.Revision,before.State).Updates(map[string]any{"revision":v.Revision,"state":v.State,"content_sha256":v.ContentSHA256,"payload":string(b)})
  err=res.Error;if err==nil&&res.RowsAffected!=1{err=plan.ErrConflict}
 };if err!=nil{return err}
 latest:=head.Latest;if v.Number>latest{latest=v.Number}
 res:=db.Model(&planRow{}).Where("plan_code=? AND revision=?",head.Code,head.Revision).Updates(map[string]any{"revision":head.Revision+1,"latest_version":latest});if res.Error!=nil{return res.Error};if res.RowsAffected!=1{return plan.ErrConflict}
 // Only draft authoring may replace references. Published/retired refs are retained.
 if before==nil||before.State==plan.Draft {
  if err:=db.Where("plan_code=? AND version=?",v.PlanCode,v.Number).Delete(&planRefRow{}).Error;err!=nil{return err}
  for _,m:=range v.Terms.Modules{if err:=db.Create(&planRefRow{PlanCode:v.PlanCode,Version:v.Number,ModuleCode:m.Code}).Error;err!=nil{return err}}
 }
 return nil
}
func(r *planRepository)Complete(ctx context.Context,operation,key,hash string,before *plan.Version,after plan.Version,at time.Time)error{
 a,err:=json.Marshal(before);if err!=nil{return err};b,err:=json.Marshal(after);if err!=nil{return err};db:=r.tx.WithContext(ctx)
 if err:=db.Create(&planAuditRow{PlanCode:after.PlanCode,Version:after.Number,RequestID:key,Action:operation,ActorID:after.ActorID,Reason:after.Reason,BeforeJSON:string(a),AfterJSON:string(b),CreatedAt:at}).Error;err!=nil{return err}
 return db.Create(&planReceiptRow{PlanCode:after.PlanCode,RequestID:key,Fingerprint:hash,Payload:string(b)}).Error
}
