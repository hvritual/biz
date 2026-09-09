// Package consistency owns the commercial lock protocol. It joins the existing
// root transaction; it is not an Executor, authorizer or cross-domain repository.
package consistency
import("context";"errors";"fmt";"time";mysql "github.com/go-sql-driver/mysql";"gorm.io/gorm";"gorm.io/gorm/clause";"yunka.io/framework/execution")
var ErrStateLost=errors.New("ENTITLEMENT_AUTHORITY_STATE_LOST")
var ErrRetryRequired=errors.New("ENTITLEMENT_RETRY_REQUIRED")
type CatalogState struct{ID uint8 `gorm:"column:id;primaryKey"`;Version uint64 `gorm:"column:version"`}
func(CatalogState)TableName()string{return "biz_commercial_catalog_state"}
func DB(ctx context.Context,fallback *gorm.DB)(*gorm.DB,error){
 if f,ok:=execution.Current(ctx);ok&&f.Transaction!=execution.TransactionNone{
 h,err:=execution.TransactionHandleFrom(ctx);if err!=nil{return nil,err};db,ok:=h.(*gorm.DB);if !ok||db==nil{return nil,errors.New("commercial: incompatible root transaction")};return db.WithContext(ctx).Session(&gorm.Session{SkipDefaultTransaction:true}),nil
 };if fallback==nil{return nil,errors.New("commercial: authoritative database required")};return fallback.WithContext(ctx),nil
}
func Transient(err error)bool{var m *mysql.MySQLError;return errors.As(err,&m)&&(m.Number==1213||m.Number==1205)}
// Within never retries a joined root: an InnoDB deadlock can roll back that
// entire transaction. The caller must retry the whole request with the same key.
// Only standalone read-model materialization may retry its own whole transaction.
func Within(ctx context.Context,db *gorm.DB,retries int,fn func(*gorm.DB)error)error{
 if f,ok:=execution.Current(ctx);ok&&f.Transaction!=execution.TransactionNone{
 if f.Transaction!=execution.TransactionLocal{return errors.New("commercial: writable root transaction required")};tx,err:=DB(ctx,nil);if err!=nil{return err};err=fn(tx);if Transient(err){return fmt.Errorf("%w: %w",ErrRetryRequired,err)};return err
 }
 for attempt:=0;;attempt++{err:=db.WithContext(ctx).Transaction(fn);if !Transient(err)||attempt>=retries{return err};timer:=time.NewTimer(time.Duration(5*(attempt+1))*time.Millisecond);select{case <-ctx.Done():timer.Stop();return ctx.Err();case <-timer.C:}}
}
func LockCatalog(tx *gorm.DB,exclusive bool)(uint64,error){
 strength:="SHARE";if exclusive{strength="UPDATE"};var state CatalogState
 err:=tx.Clauses(clause.Locking{Strength:strength}).Where("id = ?",1).First(&state).Error
 if errors.Is(err,gorm.ErrRecordNotFound)||(err==nil&&state.Version==0){return 0,ErrStateLost};return state.Version,err
}
func AdvanceCatalog(tx *gorm.DB)error{
 r:=tx.Model(&CatalogState{}).Where("id=1 AND version < ?",^uint64(0)).Update("version",gorm.Expr("version+1"));if r.Error!=nil{return r.Error};if r.RowsAffected!=1{return ErrStateLost};return nil
}
func Now(tx *gorm.DB)(time.Time,error){var v time.Time;err:=tx.Raw("SELECT UTC_TIMESTAMP(6)").Row().Scan(&v);return v.UTC(),err}
