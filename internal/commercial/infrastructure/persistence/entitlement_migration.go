package persistence

import (
 "context"
 "embed"
 "errors"
 "gorm.io/gorm"
)

//go:embed migrations/*.sql
var entitlementMigrations embed.FS

// MigrateEntitlements is explicit infrastructure setup; it never runs from a use case.
func MigrateEntitlements(ctx context.Context,db *gorm.DB)error{
 if db==nil{return errors.New("commercial migration: database required")}
 data,err:=entitlementMigrations.ReadFile("migrations/0001_entitlement_sources.sql");if err!=nil{return err}
 return db.WithContext(ctx).Exec(string(data)).Error
}
