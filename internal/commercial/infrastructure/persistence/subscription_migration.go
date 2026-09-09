package persistence
import("context";"embed";"errors";"gorm.io/gorm")
//go:embed subscriptionmigrations/*.sql
var subscriptionMigrations embed.FS
func MigrateSubscriptions(ctx context.Context,db *gorm.DB)error{if db==nil{return errors.New("subscription migration: database required")};xs,e:=subscriptionMigrations.ReadDir("subscriptionmigrations");if e!=nil{return e};for _,x:=range xs{b,e:=subscriptionMigrations.ReadFile("subscriptionmigrations/"+x.Name());if e!=nil{return e};if e=db.WithContext(ctx).Exec(string(b)).Error;e!=nil{return e}};return nil}
