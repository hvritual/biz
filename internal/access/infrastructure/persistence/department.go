package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type departmentRecord struct {
	ID           string    `gorm:"column:id;primaryKey;size:64"`
	TenantID     string    `gorm:"column:tenant_id;size:64;not null;index:idx_department_tenant;uniqueIndex:uniq_department_name,priority:1"`
	Name         string    `gorm:"column:name;size:100;not null;uniqueIndex:uniq_department_name,priority:2"`
	ParentID     string    `gorm:"column:parent_id;size:64;not null;default:'';index"`
	LeaderUserID string    `gorm:"column:leader_user_id;size:64;not null;default:'';index"`
	Email        string    `gorm:"column:email;size:320;not null;default:''"`
	Phone        string    `gorm:"column:phone;size:40;not null;default:''"`
	Status       string    `gorm:"column:status;size:32;not null;index"`
	Sort         int32     `gorm:"column:sort;not null;default:0"`
	Version      uint64    `gorm:"column:version;not null;default:1"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

func (departmentRecord) TableName() string { return "biz_departments" }

type TenantDepartmentRepository struct{ database *gorm.DB }

func NewTenantDepartmentRepository(database *gorm.DB) (*TenantDepartmentRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: tenant department database is required")
	}
	return &TenantDepartmentRepository{database: database}, nil
}

func AutoMigrateTenantDepartment(ctx context.Context, database *gorm.DB) error {
	if database == nil {
		return errors.New("access persistence: department database is required")
	}
	db := database.WithContext(ctx)
	if err := db.AutoMigrate(&departmentRecord{}); err != nil {
		return err
	}
	var ownerRows []roleRecord
	if err := db.Where("name = ?", domain.TenantOwnerRoleName).Find(&ownerRows).Error; err != nil {
		return err
	}
	for _, role := range ownerRows {
		for _, permission := range []string{"tenant.organization.read", "tenant.organization.manage"} {
			grant := permissionGrantRecord{TenantID: role.TenantID, RoleID: role.ID, Permission: permission, Scope: domain.DataScopeAll}
			if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&grant).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (repository *TenantDepartmentRepository) Create(ctx context.Context, department *domain.Department) error {
	if repository == nil || repository.database == nil || department == nil {
		return errors.New("access persistence: tenant department repository unavailable")
	}
	row := departmentRecord{ID: department.ID, TenantID: department.TenantID, Name: department.Name, ParentID: department.ParentID, LeaderUserID: department.LeaderUserID, Email: department.Email, Phone: department.Phone, Status: department.Status, Sort: department.Sort, Version: department.Version, CreatedAt: department.CreatedAt, UpdatedAt: department.UpdatedAt}
	if err := repository.database.WithContext(ctx).Create(&row).Error; err != nil {
		if isDuplicateKey(err) {
			return ports.ErrTenantDepartmentExists
		}
		return err
	}
	return nil
}

func (repository *TenantDepartmentRepository) Get(ctx context.Context, tenantID, departmentID string) (domain.Department, error) {
	var row departmentRecord
	if err := repository.database.WithContext(ctx).Where("tenant_id = ? AND id = ?", strings.TrimSpace(tenantID), strings.TrimSpace(departmentID)).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Department{}, ports.ErrTenantDepartmentNotFound
		}
		return domain.Department{}, err
	}
	return departmentFromRecord(row), nil
}

func (repository *TenantDepartmentRepository) List(ctx context.Context, tenantID string) ([]domain.Department, error) {
	var rows []departmentRecord
	if err := repository.database.WithContext(ctx).Where("tenant_id = ?", strings.TrimSpace(tenantID)).Order("sort ASC, name ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]domain.Department, 0, len(rows))
	for _, row := range rows {
		result = append(result, departmentFromRecord(row))
	}
	return result, nil
}

func (repository *TenantDepartmentRepository) Update(ctx context.Context, department *domain.Department, expectedVersion uint64) error {
	if repository == nil || repository.database == nil || department == nil || expectedVersion == 0 {
		return errors.New("access persistence: department update requires repository, value and version")
	}
	result := repository.database.WithContext(ctx).Model(&departmentRecord{}).
		Where("tenant_id = ? AND id = ? AND version = ?", department.TenantID, department.ID, expectedVersion).
		Updates(map[string]any{"name": department.Name, "parent_id": department.ParentID, "leader_user_id": department.LeaderUserID, "email": department.Email, "phone": department.Phone, "status": department.Status, "sort": department.Sort, "version": gorm.Expr("version + 1"), "updated_at": department.UpdatedAt})
	if result.Error != nil {
		if isDuplicateKey(result.Error) {
			return ports.ErrTenantDepartmentExists
		}
		return result.Error
	}
	if result.RowsAffected != 1 {
		return repository.classifyWrite(ctx, department.TenantID, department.ID)
	}
	department.Version = expectedVersion + 1
	return nil
}

func (repository *TenantDepartmentRepository) ValidateParent(ctx context.Context, tenantID, departmentID, parentID string) error {
	parentID = strings.TrimSpace(parentID)
	departmentID = strings.TrimSpace(departmentID)
	if parentID == "" {
		return nil
	}
	if parentID == departmentID {
		return ports.ErrTenantDepartmentHierarchy
	}
	visited := map[string]bool{}
	current := parentID
	for current != "" {
		if visited[current] || current == departmentID {
			return ports.ErrTenantDepartmentHierarchy
		}
		visited[current] = true
		var row departmentRecord
		if err := repository.database.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, current).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrTenantDepartmentHierarchy
			}
			return err
		}
		if row.Status != domain.TenantDepartmentStatusActive {
			return ports.ErrTenantDepartmentHierarchy
		}
		current = row.ParentID
	}
	return nil
}

func (repository *TenantDepartmentRepository) ValidateLeader(ctx context.Context, tenantID, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	var row membershipRecord
	if err := repository.database.WithContext(ctx).Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.TenantMemberStatusActive).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ports.ErrTenantDepartmentLeader
		}
		return err
	}
	return nil
}

func (repository *TenantDepartmentRepository) AssertMemberAssignmentAllowed(ctx context.Context, tenantID, departmentID string) error {
	departmentID = strings.TrimSpace(departmentID)
	if departmentID == "" {
		return nil
	}
	var row departmentRecord
	if err := repository.database.WithContext(ctx).Where("tenant_id = ? AND id = ? AND status = ?", tenantID, departmentID, domain.TenantDepartmentStatusActive).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ports.ErrTenantDepartmentAssignment
		}
		return err
	}
	return nil
}

func (repository *TenantDepartmentRepository) classifyWrite(ctx context.Context, tenantID, departmentID string) error {
	var count int64
	if err := repository.database.WithContext(ctx).Model(&departmentRecord{}).Where("tenant_id = ? AND id = ?", tenantID, departmentID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ports.ErrTenantDepartmentNotFound
	}
	return ports.ErrTenantDepartmentConflict
}

func departmentFromRecord(row departmentRecord) domain.Department {
	return domain.Department{ID: row.ID, TenantID: row.TenantID, Name: row.Name, ParentID: row.ParentID, LeaderUserID: row.LeaderUserID, Email: row.Email, Phone: row.Phone, Status: row.Status, Sort: row.Sort, Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}
