// Package persistence owns commercial entitlement source storage. Repository
// instances only receive the existing root transaction through requestscope.
package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/requestscope"
)

type entitlementStateRow struct {
	TenantID string `gorm:"column:tenant_id;primaryKey"`
	Version  uint64 `gorm:"column:version"`
}

func (entitlementStateRow) TableName() string { return "biz_commercial_entitlement_state" }

type overrideRow struct {
	ModuleCode string `gorm:"column:module_code"`
	TenantID   string `gorm:"column:tenant_id;primaryKey"`
	ID         string `gorm:"column:source_id;primaryKey"`
	Version    uint64 `gorm:"column:version"`
	Payload    string `gorm:"column:payload"`
}

func (overrideRow) TableName() string { return "biz_commercial_entitlement_sources" }

type entitlementReceiptRow struct {
	TenantID    string `gorm:"column:tenant_id;primaryKey"`
	RequestID   string `gorm:"column:request_id;primaryKey"`
	Fingerprint string `gorm:"column:fingerprint"`
	Payload     string `gorm:"column:payload"`
}

func (entitlementReceiptRow) TableName() string { return "biz_commercial_entitlement_receipts" }

type entitlementAuditRow struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID      string    `gorm:"column:tenant_id"`
	ActorID       string    `gorm:"column:actor_id"`
	ActorType     string    `gorm:"column:actor_type"`
	Action        string    `gorm:"column:action"`
	Reason        string    `gorm:"column:reason"`
	RequestID     string    `gorm:"column:request_id"`
	BeforeVersion uint64    `gorm:"column:before_version"`
	AfterVersion  uint64    `gorm:"column:after_version"`
	BeforeJSON    string    `gorm:"column:before_json"`
	AfterJSON     string    `gorm:"column:after_json"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (entitlementAuditRow) TableName() string { return "biz_commercial_entitlement_audit" }

type entitlementRepository struct{ tx *gorm.DB }

func NewEntitlementRepositoryFactory() requestscope.RepositoryFactory[ports.EntitlementRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.EntitlementRepositories, error) {
		if tx == nil {
			return ports.EntitlementRepositories{}, errors.New("commercial: root transaction required")
		}
		return ports.EntitlementRepositories{Entitlements: &entitlementRepository{tx: tx.Session(&gorm.Session{SkipDefaultTransaction: true})}}, nil
	})
}
func (r *entitlementRepository) Read(ctx context.Context, tenant string) (ports.EntitlementState, error) {
	var state entitlementStateRow
	err := r.tx.WithContext(ctx).Where("tenant_id = ?", tenant).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.EntitlementState{Sources: []entitlement.Source{}}, nil
	}
	if err != nil {
		return ports.EntitlementState{}, err
	}
	return r.sources(ctx, tenant, state.Version)
}
func (r *entitlementRepository) Lock(ctx context.Context, tenant string) (ports.EntitlementState, error) {
	db := r.tx.WithContext(ctx)
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entitlementStateRow{TenantID: tenant}).Error; err != nil {
		return ports.EntitlementState{}, err
	}
	var state entitlementStateRow
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ?", tenant).First(&state).Error; err != nil {
		return ports.EntitlementState{}, err
	}
	// A locking read after the aggregate lock must see a previous writer's commit,
	// even if a typed child already opened a REPEATABLE READ snapshot.
	var rows []overrideRow
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ?", tenant).Order("source_id").Find(&rows).Error; err != nil {
		return ports.EntitlementState{}, err
	}
	return decodeSources(tenant, state.Version, rows)
}
func (r *entitlementRepository) sources(ctx context.Context, tenant string, version uint64) (ports.EntitlementState, error) {
	var rows []overrideRow
	if err := r.tx.WithContext(ctx).Where("tenant_id = ?", tenant).Order("source_id").Find(&rows).Error; err != nil {
		return ports.EntitlementState{}, err
	}
	return decodeSources(tenant, version, rows)
}
func decodeSources(tenant string, version uint64, rows []overrideRow) (ports.EntitlementState, error) {
	out := ports.EntitlementState{Version: version, Sources: make([]entitlement.Source, 0, len(rows))}
	for _, row := range rows {
		var source entitlement.Source
		if err := json.Unmarshal([]byte(row.Payload), &source); err != nil {
			return ports.EntitlementState{}, err
		}
		if source.ModuleCode != row.ModuleCode || source.TenantID != tenant || source.ID != row.ID || source.Version != row.Version || source.SourceKind != entitlement.OverrideSource {
			return ports.EntitlementState{}, entitlement.ErrScope
		}
		if err := source.ValidateShape(); err != nil {
			return ports.EntitlementState{}, err
		}
		out.Sources = append(out.Sources, source)
	}
	return out, nil
}
func (r *entitlementRepository) Receipt(ctx context.Context, tenant, key, hash string) (*ports.OverrideReceipt, error) {
	var row entitlementReceiptRow
	err := r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND request_id = ?", tenant, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.Fingerprint != hash {
		return nil, entitlement.ErrRequestConflict
	}
	var receipt ports.OverrideReceipt
	if err := json.Unmarshal([]byte(row.Payload), &receipt); err != nil {
		return nil, err
	}
	if receipt.Source.TenantID != tenant {
		return nil, entitlement.ErrScope
	}
	return &receipt, nil
}
func (r *entitlementRepository) Insert(ctx context.Context, s entitlement.Source) error {
	payload, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return r.tx.WithContext(ctx).Create(&overrideRow{TenantID: s.TenantID, ModuleCode: s.ModuleCode, ID: s.ID, Version: s.Version, Payload: string(payload)}).Error
}
func (r *entitlementRepository) Revoke(ctx context.Context, s entitlement.Source, expected uint64) error {
	payload, err := json.Marshal(s)
	if err != nil {
		return err
	}
	res := r.tx.WithContext(ctx).Model(&overrideRow{}).Where("tenant_id = ? AND source_id = ? AND version = ?", s.TenantID, s.ID, expected).Updates(map[string]any{"payload": string(payload), "version": s.Version})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return entitlement.ErrConflict
	}
	return nil
}
func (r *entitlementRepository) Advance(ctx context.Context, tenant string, expected uint64) error {
	if expected == math.MaxUint64 {
		return entitlement.ErrConflict
	}
	res := r.tx.WithContext(ctx).Model(&entitlementStateRow{}).Where("tenant_id = ? AND version = ?", tenant, expected).Update("version", expected+1)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return entitlement.ErrConflict
	}
	return nil
}
func (r *entitlementRepository) Audit(ctx context.Context, a ports.EntitlementAudit) error {
	before, err := json.Marshal(a.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(a.After)
	if err != nil {
		return err
	}
	return r.tx.WithContext(ctx).Create(&entitlementAuditRow{TenantID: a.TenantID, ActorID: a.ActorID, ActorType: "platform", Action: a.Action, Reason: a.Reason, RequestID: a.RequestID, BeforeVersion: a.BeforeVersion, AfterVersion: a.AfterVersion, BeforeJSON: string(before), AfterJSON: string(after), CreatedAt: a.At}).Error
}
func (r *entitlementRepository) SaveReceipt(ctx context.Context, tenant, key, hash string, receipt ports.OverrideReceipt) error {
	payload, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	return r.tx.WithContext(ctx).Create(&entitlementReceiptRow{TenantID: tenant, RequestID: key, Fingerprint: hash, Payload: string(payload)}).Error
}
