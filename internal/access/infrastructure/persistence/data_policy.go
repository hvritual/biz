package persistence

import (
	"context"
	"errors"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

type dataPolicyRecord struct {
	TenantID  string     `gorm:"column:tenant_id;primaryKey;size:64"`
	ID        string     `gorm:"column:id;primaryKey;size:160"`
	Name      string     `gorm:"column:name;size:100;not null"`
	Status    string     `gorm:"column:status;size:32;not null;index"`
	Version   uint64     `gorm:"column:version;not null;default:1"`
	NotBefore *time.Time `gorm:"column:not_before"`
	ExpiresAt *time.Time `gorm:"column:expires_at"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt time.Time  `gorm:"column:updated_at;not null"`
}

func (dataPolicyRecord) TableName() string { return "biz_data_policies" }

type dataPolicySiteRecord struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;size:64"`
	PolicyID string `gorm:"column:policy_id;primaryKey;size:160"`
	SiteID   string `gorm:"column:site_id;primaryKey;size:160;index"`
}

func (dataPolicySiteRecord) TableName() string { return "biz_data_policy_sites" }

type TenantDataPolicyRepository struct{ database *gorm.DB }

func NewTenantDataPolicyRepository(database *gorm.DB) (*TenantDataPolicyRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: tenant data policy database is required")
	}
	return &TenantDataPolicyRepository{database: database}, nil
}
func (r *TenantDataPolicyRepository) Create(ctx context.Context, p *domain.DataPolicy) error {
	if r == nil || r.database == nil || p == nil {
		return errors.New("access persistence: tenant data policy repository unavailable")
	}
	row := dataPolicyRecord{TenantID: p.TenantID, ID: p.ID, Name: p.Name, Status: p.Status, Version: p.Version, NotBefore: p.NotBefore, ExpiresAt: p.ExpiresAt, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt}
	if err := r.database.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	return r.replaceSites(ctx, p.TenantID, p.ID, p.SiteIDs)
}
func (r *TenantDataPolicyRepository) Get(ctx context.Context, tenantID, policyID string) (domain.DataPolicy, error) {
	var row dataPolicyRecord
	err := r.database.WithContext(ctx).Where("tenant_id = ? AND id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(policyID)).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.DataPolicy{}, ports.ErrTenantDataPolicyNotFound
		}
		return domain.DataPolicy{}, err
	}
	return r.policyFromRecord(ctx, row)
}
func (r *TenantDataPolicyRepository) List(ctx context.Context, tenantID string) ([]domain.DataPolicy, error) {
	var rows []dataPolicyRecord
	if err := r.database.WithContext(ctx).Where("tenant_id = ?", strings.TrimSpace(tenantID)).Order("name ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.DataPolicy, 0, len(rows))
	for _, row := range rows {
		p, err := r.policyFromRecord(ctx, row)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, nil
}
func (r *TenantDataPolicyRepository) Update(ctx context.Context, p *domain.DataPolicy, expected uint64) error {
	if p == nil || expected == 0 {
		return ports.ErrTenantDataPolicyConflict
	}
	db := r.database.WithContext(ctx)
	var locked dataPolicyRecord
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", p.TenantID, p.ID).First(&locked).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ports.ErrTenantDataPolicyNotFound
		}
		return err
	}
	if locked.Version != expected {
		return ports.ErrTenantDataPolicyConflict
	}
	if locked.Status != domain.TenantDataPolicyStatusActive {
		return ports.ErrTenantDataPolicyUnavailable
	}
	res := db.Model(&dataPolicyRecord{}).Where("tenant_id = ? AND id = ? AND version = ?", p.TenantID, p.ID, expected).Updates(map[string]any{"name": p.Name, "not_before": p.NotBefore, "expires_at": p.ExpiresAt, "version": gorm.Expr("version + 1"), "updated_at": p.UpdatedAt})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ports.ErrTenantDataPolicyConflict
	}
	if err := r.replaceSites(ctx, p.TenantID, p.ID, p.SiteIDs); err != nil {
		return err
	}
	p.Version = expected + 1
	return nil
}
func (r *TenantDataPolicyRepository) Revoke(ctx context.Context, tenantID, policyID string, expected uint64) (domain.DataPolicy, error) {
	db := r.database.WithContext(ctx)
	var row dataPolicyRecord
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, policyID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.DataPolicy{}, ports.ErrTenantDataPolicyNotFound
		}
		return domain.DataPolicy{}, err
	}
	if row.Version != expected {
		return domain.DataPolicy{}, ports.ErrTenantDataPolicyConflict
	}
	if row.Status != domain.TenantDataPolicyStatusActive {
		return domain.DataPolicy{}, ports.ErrTenantDataPolicyUnavailable
	}
	now := time.Now().UTC()
	res := db.Model(&dataPolicyRecord{}).Where("tenant_id = ? AND id = ? AND version = ?", tenantID, policyID, expected).Updates(map[string]any{"status": domain.TenantDataPolicyStatusRevoked, "version": gorm.Expr("version + 1"), "updated_at": now})
	if res.Error != nil {
		return domain.DataPolicy{}, res.Error
	}
	if res.RowsAffected != 1 {
		return domain.DataPolicy{}, ports.ErrTenantDataPolicyConflict
	}
	return r.Get(ctx, tenantID, policyID)
}
func (r *TenantDataPolicyRepository) replaceSites(ctx context.Context, tenantID, policyID string, ids []string) error {
	db := r.database.WithContext(ctx)
	if err := db.Where("tenant_id = ? AND policy_id = ?", tenantID, policyID).Delete(&dataPolicySiteRecord{}).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := db.Create(&dataPolicySiteRecord{TenantID: tenantID, PolicyID: policyID, SiteID: id}).Error; err != nil {
			return err
		}
	}
	return nil
}
func (r *TenantDataPolicyRepository) policyFromRecord(ctx context.Context, row dataPolicyRecord) (domain.DataPolicy, error) {
	var sites []dataPolicySiteRecord
	if err := r.database.WithContext(ctx).Where("tenant_id = ? AND policy_id = ?", row.TenantID, row.ID).Order("site_id ASC").Find(&sites).Error; err != nil {
		return domain.DataPolicy{}, err
	}
	ids := make([]string, 0, len(sites))
	for _, s := range sites {
		ids = append(ids, s.SiteID)
	}
	return domain.DataPolicy{ID: row.ID, TenantID: row.TenantID, Name: row.Name, Status: row.Status, SiteIDs: ids, Version: row.Version, NotBefore: row.NotBefore, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}, nil
}
func (r *TenantRoleRepository) SetDataPolicy(ctx context.Context, tenantID, roleID string, roleVersion uint64, policyID string, policyVersion uint64, now time.Time) (domain.Role, error) {
	db := r.database.WithContext(ctx)
	var role roleRecord
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Role{}, ports.ErrTenantRoleNotFound
		}
		return domain.Role{}, err
	}
	if role.Version != roleVersion {
		return domain.Role{}, ports.ErrTenantRoleConflict
	}
	updates := map[string]any{"version": gorm.Expr("version + 1")}
	policyID = strings.TrimSpace(policyID)
	if policyID == "" {
		if policyVersion != 0 {
			return domain.Role{}, ports.ErrTenantDataPolicyConflict
		}
		updates["data_policy_id"] = nil
		updates["data_policy_accepted_version"] = nil
	} else {
		var row dataPolicyRecord
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, policyID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.Role{}, ports.ErrTenantDataPolicyUnavailable
			}
			return domain.Role{}, err
		}
		if row.Version != policyVersion {
			return domain.Role{}, ports.ErrTenantDataPolicyConflict
		}
		pr, _ := NewTenantDataPolicyRepository(db)
		p, err := pr.policyFromRecord(ctx, row)
		if err != nil {
			return domain.Role{}, err
		}
		if ok, _ := p.EffectiveAt(now); !ok {
			return domain.Role{}, ports.ErrTenantDataPolicyUnavailable
		}
		updates["data_policy_id"] = policyID
		updates["data_policy_accepted_version"] = policyVersion
	}
	res := db.Model(&roleRecord{}).Where("tenant_id = ? AND id = ? AND version = ?", tenantID, roleID, roleVersion).Updates(updates)
	if res.Error != nil {
		return domain.Role{}, res.Error
	}
	if res.RowsAffected != 1 {
		return domain.Role{}, ports.ErrTenantRoleConflict
	}
	return r.Get(ctx, tenantID, roleID)
}
func (r *TenantRoleRepository) roleDataPolicyReference(ctx context.Context, role roleRecord) (*domain.DataPolicyReference, error) {
	if role.DataPolicyID == nil || strings.TrimSpace(*role.DataPolicyID) == "" {
		return nil, nil
	}
	accepted := uint64(0)
	if role.DataPolicyAcceptedVersion != nil {
		accepted = *role.DataPolicyAcceptedVersion
	}
	pr, _ := NewTenantDataPolicyRepository(r.database)
	p, err := pr.Get(ctx, role.TenantID, *role.DataPolicyID)
	if err != nil {
		if errors.Is(err, ports.ErrTenantDataPolicyNotFound) {
			return &domain.DataPolicyReference{PolicyID: *role.DataPolicyID, AcceptedVersion: accepted, InvalidReason: domain.DataPolicyInvalidMissing}, nil
		}
		return nil, err
	}
	effective, reason := p.EffectiveAt(time.Now().UTC())
	return &domain.DataPolicyReference{PolicyID: p.ID, PolicyName: p.Name, PolicyVersion: p.Version, AcceptedVersion: accepted, Effective: effective, InvalidReason: reason}, nil
}

var _ ports.TenantDataPolicyRepository = (*TenantDataPolicyRepository)(nil)
