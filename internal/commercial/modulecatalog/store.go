package modulecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

type moduleRow struct { Code string `gorm:"column:module_code;primaryKey;size:96"`; Name string `gorm:"size:160;not null"`; Category string `gorm:"size:96;not null"`; SalesScopeJSON string `gorm:"column:sales_scope_json;type:text;not null"`; TechnicalStatus string `gorm:"size:32;not null"`; SalesStatus string `gorm:"size:32;not null"`; Version uint64 `gorm:"not null"`; CreatedAt time.Time; UpdatedAt time.Time }
func (moduleRow) TableName() string { return "biz_commercial_modules" }
type dependencyRow struct { ModuleCode string `gorm:"primaryKey;size:96"`; DependsOn string `gorm:"primaryKey;size:96;index"` }
func (dependencyRow) TableName() string { return "biz_commercial_module_dependencies" }
type retiredCodeRow struct { ModuleCode string `gorm:"primaryKey;size:96"`; RetiredAt time.Time `gorm:"not null"` }
func (retiredCodeRow) TableName() string { return "biz_commercial_module_retired_codes" }
type auditRow struct { ID uint64 `gorm:"primaryKey;autoIncrement"`; ModuleCode string `gorm:"size:96;index;not null"`; Actor string `gorm:"size:160;not null"`; Action string `gorm:"size:64;not null"`; BeforeJSON string `gorm:"type:text"`; AfterJSON string `gorm:"type:text"`; Reason string `gorm:"size:512;not null"`; RequestID string `gorm:"size:128;index"`; CreatedAt time.Time `gorm:"not null"` }
func (auditRow) TableName() string { return "biz_commercial_module_audit" }
type idempotencyRow struct { RequestID string `gorm:"primaryKey;size:128"`; Operation string `gorm:"size:64;not null"`; ModuleCode string `gorm:"size:96;not null"`; ResponseJSON string `gorm:"type:text;not null"`; CreatedAt time.Time `gorm:"not null"` }
func (idempotencyRow) TableName() string { return "biz_commercial_module_idempotency" }

type Store struct{ db *gorm.DB }
func NewStore(db *gorm.DB) (*Store,error) { if db==nil { return nil,errors.New("module catalog: database required") }; return &Store{db:db},nil }
func (s *Store) Migrate(ctx context.Context) error { return s.db.WithContext(ctx).AutoMigrate(&moduleRow{},&dependencyRow{},&retiredCodeRow{},&auditRow{},&idempotencyRow{}) }
func (s *Store) DB() *gorm.DB { return s.db }

func encode(v any) string { b,_:=json.Marshal(v); return string(b) }
func rowToModule(row moduleRow, def Definition) Module { var scope []string; _=json.Unmarshal([]byte(row.SalesScopeJSON),&scope); return Module{Code:row.Code,Name:row.Name,Category:row.Category,SalesScope:scope,TechnicalStatus:TechnicalStatus(row.TechnicalStatus),SalesStatus:SalesStatus(row.SalesStatus),CapabilityCodes:append([]string(nil),def.CapabilityCodes...),QuotaSchemaKeys:append([]string(nil),def.QuotaSchemaKeys...),FieldPolicySchemaKeys:append([]string(nil),def.FieldPolicySchemaKeys...),Dependencies:append([]string(nil),def.Dependencies...),Version:row.Version,CreatedAt:row.CreatedAt,UpdatedAt:row.UpdatedAt} }
