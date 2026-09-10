package persistence

import (
 "context"
 "embed"
 "errors"
 "gorm.io/gorm"
)

//go:embed provisioningmigrations/*.sql
var provisioningMigrations embed.FS

// MigrateProvisioning follows subscription change tables. Production applies
// additive migrations explicitly before enabling the new binary.
func MigrateProvisioning(ctx context.Context,db *gorm.DB)error{
 if db==nil{return errors.New("provisioning migration: database required")}
 entries,err:=provisioningMigrations.ReadDir("provisioningmigrations");if err!=nil{return err}
 for _,e:=range entries {
  b,err:=provisioningMigrations.ReadFile("provisioningmigrations/"+e.Name());if err!=nil{return err}
  if err=db.WithContext(ctx).Exec(string(b)).Error;err!=nil{return err}
 }
 return nil
}
