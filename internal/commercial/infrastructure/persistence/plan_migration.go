package persistence
import("context";"embed";"errors";"gorm.io/gorm")
//go:embed planmigrations/*.sql
var planMigrations embed.FS
// MigratePlans is composition-time schema setup. CE-08 subscription tables are installed immediately after plan tables because they hold RESTRICT plan-version FKs.
func MigratePlans(ctx context.Context,db *gorm.DB)error{if db==nil{return errors.New("plan migration: database required")};xs,e:=planMigrations.ReadDir("planmigrations");if e!=nil{return e};for _,x:=range xs{b,e:=planMigrations.ReadFile("planmigrations/"+x.Name());if e!=nil{return e};if e=db.WithContext(ctx).Exec(string(b)).Error;e!=nil{return e}};return MigrateSubscriptions(ctx,db)}
