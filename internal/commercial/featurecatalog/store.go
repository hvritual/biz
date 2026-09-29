package featurecatalog

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store is the single persistence adapter for CommercialFeature lifecycle
// facts. Plan, add-on, subscription and entitlement references remain owned by
// their existing repositories and are supplied as ReferenceImpact at Retire.
type Store struct{ db *gorm.DB }

type featureRow struct {
	Code            string    `gorm:"column:feature_code;primaryKey;size:96"`
	Name            string    `gorm:"size:160;not null"`
	Product         string    `gorm:"column:product_state;size:32;not null"`
	Sales           string    `gorm:"column:sales_state;size:32;not null"`
	Runtime         string    `gorm:"column:runtime_state;size:32;not null"`
	Migration       string    `gorm:"column:migration_state;size:32;not null"`
	ReplacementCode string    `gorm:"column:replacement_feature_code;size:96;not null"`
	Version         uint64    `gorm:"not null"`
	CreatedAt       time.Time `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null"`
}

func (featureRow) TableName() string { return "biz_commercial_features" }

type featureModuleRow struct {
	FeatureCode     string `gorm:"column:feature_code;primaryKey;size:96"`
	ModuleCode      string `gorm:"column:module_code;primaryKey;size:96"`
	CapabilityCodes string `gorm:"column:capability_codes_json;type:text;not null"`
}

func (featureModuleRow) TableName() string { return "biz_commercial_feature_modules" }

type featureAuditRow struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	Feature   string    `gorm:"column:feature_code;size:96;not null"`
	Actor     string    `gorm:"size:160;not null"`
	Action    string    `gorm:"size:64;not null"`
	Before    string    `gorm:"column:before_json;type:text"`
	After     string    `gorm:"column:after_json;type:text"`
	Reason    string    `gorm:"size:512;not null"`
	RequestID string    `gorm:"column:request_id;size:128;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (featureAuditRow) TableName() string { return "biz_commercial_feature_audit" }

type featureIdempotencyRow struct {
	RequestID string `gorm:"column:request_id;primaryKey;size:128"`
	Operation string `gorm:"size:64;not null"`
	Feature   string `gorm:"column:feature_code;size:96;not null"`
	Response  string `gorm:"column:response_json;type:text;not null"`
}

func (featureIdempotencyRow) TableName() string { return "biz_commercial_feature_idempotency" }

func rowToFeature(row featureRow, refs []featureModuleRow) (Feature, error) {
	feature := Feature{Code: row.Code, Name: row.Name, Version: row.Version, Product: ProductState(row.Product), Sales: SalesState(row.Sales), Runtime: RuntimeState(row.Runtime), Migration: MigrationState(row.Migration), ReplacementCode: row.ReplacementCode}
	for _, ref := range refs {
		var capabilities []string
		if err := json.Unmarshal([]byte(ref.CapabilityCodes), &capabilities); err != nil {
			return Feature{}, err
		}
		feature.ModuleRefs = append(feature.ModuleRefs, ModuleReference{ModuleCode: ref.ModuleCode, CapabilityCodes: capabilities})
	}
	return feature, feature.Validate()
}

func featureToRow(feature Feature) featureRow {
	return featureRow{Code: feature.Code, Name: feature.Name, Product: string(feature.Product), Sales: string(feature.Sales), Runtime: string(feature.Runtime), Migration: string(feature.Migration), ReplacementCode: feature.ReplacementCode, Version: feature.Version}
}

func NewStore(db *gorm.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("commercial feature: database required")
	}
	return &Store{db: db}, nil
}

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		data, err := migrationFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if err := s.db.WithContext(ctx).Exec(string(data)).Error; err != nil {
			return err
		}
	}
	return nil
}
