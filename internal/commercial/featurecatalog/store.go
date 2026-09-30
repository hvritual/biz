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
// facts. Retirement reads reference facts from their existing authorities.
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

// ReferenceImpact obtains conservative retirement blockers from the actual
// plan, subscription and entitlement authorities. A historical subscription
// remains a blocker until an explicit migration has removed its module ref.
func (s *Store) ReferenceImpact(ctx context.Context, feature Feature) (ReferenceImpact, error) {
	modules := make([]string, 0, len(feature.ModuleRefs))
	for _, reference := range feature.ModuleRefs {
		modules = append(modules, reference.ModuleCode)
	}
	if len(modules) == 0 {
		return ReferenceImpact{}, ErrInvalid
	}
	impact := ReferenceImpact{}
	var publishedPlans, activeSubscriptions, entitlementSources int64
	if err := s.db.WithContext(ctx).Table("biz_commercial_plan_versions AS v").
		Joins("JOIN biz_commercial_plan_module_refs AS r ON r.plan_code = v.plan_code AND r.version = v.version").
		Where("v.state = ? AND r.module_code IN ?", "PUBLISHED", modules).
		Distinct("v.plan_code", "v.version").Count(&publishedPlans).Error; err != nil {
		return ReferenceImpact{}, err
	}
	if err := s.db.WithContext(ctx).Table("biz_commercial_subscriptions AS s").
		Joins("JOIN biz_commercial_plan_module_refs AS r ON r.plan_code = s.plan_code AND r.version = s.plan_version").
		Where("r.module_code IN ?", modules).Distinct("s.tenant_id").Count(&activeSubscriptions).Error; err != nil {
		return ReferenceImpact{}, err
	}
	if err := s.db.WithContext(ctx).Table("biz_commercial_entitlement_sources").Where("module_code IN ?", modules).Count(&entitlementSources).Error; err != nil {
		return ReferenceImpact{}, err
	}
	impact.PublishedPlans = uint64(publishedPlans)
	impact.ActiveSubscriptions = uint64(activeSubscriptions)
	impact.EntitlementSources = uint64(entitlementSources)
	return impact, nil
}
