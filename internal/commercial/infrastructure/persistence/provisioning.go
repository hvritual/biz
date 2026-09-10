package persistence

import (
 "context"
 "encoding/json"
 "errors"
 "time"
 p "github.com/hvritual/biz/internal/commercial/domain/provisioning"
 "github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
 "github.com/hvritual/biz/internal/commercial/ports"
 "gorm.io/gorm"
 "gorm.io/gorm/clause"
 "yunka.io/framework/requestscope"
)
type provisioningRow struct {
 TaskID string `gorm:"primaryKey"`
 TenantID string
 ChangeID string
 Revision uint64
 State string
 NextAttemptAt time.Time
 LeaseUntil *time.Time
 PayloadSHA256 string `gorm:"column:payload_sha256"`
 Payload string
 CreatedAt time.Time
 UpdatedAt time.Time
}
func(provisioningRow)TableName()string{return "biz_commercial_provisioning_tasks"}
type provisioningAuditRow struct {
 ID uint64 `gorm:"primaryKey;autoIncrement"`
 TaskID string
 Revision uint64
 ActorID string
 Action string
 Reason string
 PayloadSHA256 string `gorm:"column:payload_sha256"`
 Payload string
 CreatedAt time.Time
}
func(provisioningAuditRow)TableName()string{return "biz_commercial_provisioning_audit"}
type provisioningActionRow struct {
 ReceiptKey string `gorm:"primaryKey"`
 ActorID string
 Operation string
 RequestID string
 Fingerprint string
 TaskID string
 PayloadSHA256 string `gorm:"column:payload_sha256"`
 Payload string
}
func(provisioningActionRow)TableName()string{return "biz_commercial_provisioning_actions"}
type provisioningRepository struct{tx *gorm.DB}
func NewProvisioningRepositoryFactory() requestscope.RepositoryFactory[ports.ProvisioningRepositories] {
 return requestscope.GORMRepositories(func(_ context.Context,tx *gorm.DB)(ports.ProvisioningRepositories,error){
  if tx==nil{return ports.ProvisioningRepositories{},errors.New("provisioning: original root transaction required")}
  db:=tx.Session(&gorm.Session{SkipDefaultTransaction:true})
  return ports.ProvisioningRepositories{Tasks:&provisioningRepository{tx:db},Events:&outboxRepository{tx:db}},nil
 })
}
func(r *provisioningRepository)Now(ctx context.Context)(time.Time,error){return consistency.Now(r.tx.WithContext(ctx))}
func decodeTask(row provisioningRow)(*p.Task,error){
 var t p.Task
 if json.Unmarshal([]byte(row.Payload),&t)!=nil||t.Integrity()!=nil||t.ID!=row.TaskID||t.TenantID!=row.TenantID||t.Approval.ChangeID!=row.ChangeID||t.Revision!=row.Revision||t.State!=row.State||t.Hash!=row.PayloadSHA256||!t.NextAttemptAt.Equal(row.NextAttemptAt)||!sameTime(t.LeaseUntil,row.LeaseUntil)||!t.CreatedAt.Equal(row.CreatedAt)||!t.UpdatedAt.Equal(row.UpdatedAt){return nil,p.ErrCorrupt}
 return &t,nil
}
func sameTime(a,b *time.Time)bool{if a==nil||b==nil{return a==nil&&b==nil};return a.Equal(*b)}
func(r *provisioningRepository)Get(ctx context.Context,tenant,id string)(*p.Task,error){return r.get(ctx,tenant,id,false)}
func(r *provisioningRepository)LockTask(ctx context.Context,tenant,id string)(*p.Task,error){
 if _,e:=(&subscriptionChangeRepository{tx:r.tx}).LockTenant(ctx,tenant);e!=nil{return nil,e}
 return r.get(ctx,tenant,id,true)
}
func(r *provisioningRepository)get(ctx context.Context,tenant,id string,lock bool)(*p.Task,error){
 db:=r.tx.WithContext(ctx);if lock{db=db.Clauses(clause.Locking{Strength:"UPDATE"})}
 var row provisioningRow
 e:=db.Where("tenant_id=? AND task_id=?",tenant,id).First(&row).Error
 if errors.Is(e,gorm.ErrRecordNotFound){return nil,p.ErrNotFound};if e!=nil{return nil,e}
 return decodeTask(row)
}
func(r *provisioningRepository)List(ctx context.Context,tenant,after string,limit uint32)([]p.Task,error){
 if limit==0||limit>100{return nil,p.ErrInvalid}
 var rows []provisioningRow
 if e:=r.tx.WithContext(ctx).Where("tenant_id=? AND task_id>?",tenant,after).Order("task_id").Limit(int(limit)).Find(&rows).Error;e!=nil{return nil,e}
 out:=[]p.Task{};for _,row:=range rows{t,e:=decodeTask(row);if e!=nil{return nil,e};out=append(out,*t)};return out,nil
}
func(r *provisioningRepository)Due(ctx context.Context,now time.Time,limit uint32)([]p.Task,error){
 if limit==0||limit>32{return nil,p.ErrInvalid}
 var rows []provisioningRow
 if e:=r.tx.WithContext(ctx).Where("(state IN ? AND next_attempt_at<=?) OR (state=? AND lease_until<=?)",[]string{p.Queued,p.RetryWait,p.Ready},now,p.Running,now).Order("next_attempt_at,task_id").Limit(int(limit)).Find(&rows).Error;e!=nil{return nil,e}
 out:=[]p.Task{};for _,row:=range rows{t,e:=decodeTask(row);if e!=nil{return nil,e};out=append(out,*t)};return out,nil
}
func taskRow(t p.Task)(provisioningRow,error){
 if e:=t.Integrity();e!=nil{return provisioningRow{},e}
 b,e:=json.Marshal(t)
 return provisioningRow{TaskID:t.ID,TenantID:t.TenantID,ChangeID:t.Approval.ChangeID,Revision:t.Revision,State:t.State,NextAttemptAt:t.NextAttemptAt,LeaseUntil:t.LeaseUntil,PayloadSHA256:t.Hash,Payload:string(b),CreatedAt:t.CreatedAt,UpdatedAt:t.UpdatedAt},e
}
func(r *provisioningRepository)Insert(ctx context.Context,t p.Task)error{
 row,e:=taskRow(t);if e!=nil{return e}
 if e=r.tx.WithContext(ctx).Create(&row).Error;e!=nil{return e}
 return r.audit(ctx,t,"CREATED",t.Approval.ActorID,"confirmed preparation intent")
}
func(r *provisioningRepository)Save(ctx context.Context,before,after p.Task,actor,reason string)error{
 if before.ID!=after.ID||before.TenantID!=after.TenantID||after.Revision!=before.Revision+1||p.Digest(before.Approval)!=p.Digest(after.Approval)||!p.Reason(reason)||actor==""{return p.ErrConflict}
 row,e:=taskRow(after);if e!=nil{return e}
 res:=r.tx.WithContext(ctx).Model(&provisioningRow{}).Where("task_id=? AND tenant_id=? AND revision=? AND payload_sha256=?",before.ID,before.TenantID,before.Revision,before.Hash).Updates(map[string]any{"revision":row.Revision,"state":row.State,"next_attempt_at":row.NextAttemptAt,"lease_until":row.LeaseUntil,"payload_sha256":row.PayloadSHA256,"payload":row.Payload,"updated_at":row.UpdatedAt})
 if res.Error!=nil{return res.Error};if res.RowsAffected!=1{return p.ErrConflict}
 return r.audit(ctx,after,after.State,actor,reason)
}
func(r *provisioningRepository)audit(ctx context.Context,t p.Task,action,actor,reason string)error{
 b,e:=json.Marshal(t);if e!=nil{return e}
 return r.tx.WithContext(ctx).Create(&provisioningAuditRow{TaskID:t.ID,Revision:t.Revision,ActorID:actor,Action:action,Reason:reason,PayloadSHA256:t.Hash,Payload:string(b),CreatedAt:t.UpdatedAt}).Error
}
func actionKey(actor,op,key string)string{return p.Digest([]string{actor,op,key})}
func(r *provisioningRepository)ActionReceipt(ctx context.Context,actor,op,key,fp string)(*p.Task,error){
 var row provisioningActionRow
 e:=r.tx.WithContext(ctx).Clauses(clause.Locking{Strength:"SHARE"}).Where("receipt_key=?",actionKey(actor,op,key)).First(&row).Error
 if errors.Is(e,gorm.ErrRecordNotFound){return nil,nil};if e!=nil{return nil,e}
 if row.ActorID!=actor||row.Operation!=op||row.RequestID!=key||row.Fingerprint!=fp{return nil,p.ErrKeyConflict}
 var t p.Task
 if json.Unmarshal([]byte(row.Payload),&t)!=nil||t.Integrity()!=nil||t.Hash!=row.PayloadSHA256||t.ID!=row.TaskID{return nil,p.ErrCorrupt}
 return &t,nil
}
func(r *provisioningRepository)StoreActionReceipt(ctx context.Context,actor,op,key,fp string,t p.Task)error{
 if t.Integrity()!=nil{return p.ErrCorrupt};b,e:=json.Marshal(t);if e!=nil{return e}
 return r.tx.WithContext(ctx).Create(&provisioningActionRow{ReceiptKey:actionKey(actor,op,key),ActorID:actor,Operation:op,RequestID:key,Fingerprint:fp,TaskID:t.ID,PayloadSHA256:t.Hash,Payload:string(b)}).Error
}
