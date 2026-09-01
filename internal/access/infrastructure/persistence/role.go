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
	"yunka.io/framework/requestscope"
)

type TenantRoleRepository struct {
	database *gorm.DB
}

func NewTenantRoleRepository(database *gorm.DB) (*TenantRoleRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: tenant role database is required")
	}
	return &TenantRoleRepository{database: database}, nil
}

func (repository *TenantRoleRepository) Create(ctx context.Context, role *domain.Role) error {
	if repository == nil || repository.database == nil || role == nil {
		return errors.New("access persistence: tenant role repository unavailable")
	}
	row := roleRecord{ID: role.ID, TenantID: role.TenantID, Name: role.Name, Status: role.Status, Version: role.Version}
	if err := repository.database.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	return nil
}

func (repository *TenantRoleRepository) BootstrapOwner(ctx context.Context, tenantID, userID string, now time.Time) (domain.Role, error) {
	if repository == nil || repository.database == nil {
		return domain.Role{}, errors.New("access persistence: tenant role repository unavailable")
	}
	tenantID, userID = strings.TrimSpace(tenantID), strings.TrimSpace(userID)
	if tenantID == "" || userID == "" {
		return domain.Role{}, errors.New("access persistence: owner bootstrap requires tenant and user")
	}
	db := repository.database.WithContext(ctx)
	var membership membershipRecord
	if err := db.Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.TenantMemberStatusActive).First(&membership).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Role{}, ports.ErrTenantRoleMember
		}
		return domain.Role{}, err
	}
	roleID := tenantID + ":owner"
	var existing roleRecord
	if err := db.Where("tenant_id = ? AND id = ?", tenantID, roleID).First(&existing).Error; err == nil {
		if existing.Name != domain.TenantOwnerRoleName || existing.Status != domain.TenantRoleStatusActive {
			return domain.Role{}, ports.ErrTenantRoleConflict
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&memberRoleRecord{TenantID: tenantID, UserID: userID, RoleID: roleID}).Error; err != nil {
			return domain.Role{}, err
		}
		return repository.Get(ctx, tenantID, roleID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Role{}, err
	}

	owner := domain.NewOwnerRole(roleID, tenantID, now)
	if err := db.Create(&roleRecord{ID: owner.ID, TenantID: tenantID, Name: owner.Name, Status: owner.Status, Version: owner.Version}).Error; err != nil {
		return domain.Role{}, err
	}
	for _, grant := range owner.Permissions {
		if err := db.Create(&permissionGrantRecord{TenantID: tenantID, RoleID: roleID, Permission: grant.Permission, Scope: grant.Scope}).Error; err != nil {
			return domain.Role{}, err
		}
	}
	if err := db.Create(&memberRoleRecord{TenantID: tenantID, UserID: userID, RoleID: roleID}).Error; err != nil {
		return domain.Role{}, err
	}
	return repository.Get(ctx, tenantID, roleID)
}

func (repository *TenantRoleRepository) Get(ctx context.Context, tenantID, roleID string) (domain.Role, error) {
	if repository == nil || repository.database == nil {
		return domain.Role{}, errors.New("access persistence: tenant role repository unavailable")
	}
	var row roleRecord
	if err := repository.database.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, roleID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Role{}, ports.ErrTenantRoleNotFound
		}
		return domain.Role{}, err
	}
	return repository.roleFromRecord(ctx, row)
}

func (repository *TenantRoleRepository) List(ctx context.Context, tenantID string) ([]domain.Role, error) {
	if repository == nil || repository.database == nil {
		return nil, errors.New("access persistence: tenant role repository unavailable")
	}
	var rows []roleRecord
	if err := repository.database.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("name ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	roles := make([]domain.Role, 0, len(rows))
	for _, row := range rows {
		role, err := repository.roleFromRecord(ctx, row)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func (repository *TenantRoleRepository) Update(ctx context.Context, role *domain.Role, expectedVersion uint64) error {
	if repository == nil || repository.database == nil || role == nil || expectedVersion == 0 {
		return errors.New("access persistence: role update requires repository, value and version")
	}
	result := repository.database.WithContext(ctx).Model(&roleRecord{}).
		Where("tenant_id = ? AND id = ? AND version = ?", role.TenantID, role.ID, expectedVersion).
		Updates(map[string]any{"name": role.Name, "status": role.Status, "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return repository.classifyRoleWrite(ctx, role.TenantID, role.ID)
	}
	role.Version = expectedVersion + 1
	return nil
}

func (repository *TenantRoleRepository) ReplacePermissions(ctx context.Context, role *domain.Role, expectedVersion uint64) error {
	if repository == nil || repository.database == nil || role == nil || expectedVersion == 0 {
		return errors.New("access persistence: permission replacement requires repository, value and version")
	}
	db := repository.database.WithContext(ctx)
	var locked roleRecord
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", role.TenantID, role.ID).First(&locked).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ports.ErrTenantRoleNotFound
		}
		return err
	}
	if locked.Version != expectedVersion {
		return ports.ErrTenantRoleConflict
	}
	if err := db.Where("tenant_id = ? AND role_id = ?", role.TenantID, role.ID).Delete(&permissionGrantRecord{}).Error; err != nil {
		return err
	}
	for _, grant := range role.Permissions {
		if err := db.Create(&permissionGrantRecord{TenantID: role.TenantID, RoleID: role.ID, Permission: grant.Permission, Scope: grant.Scope}).Error; err != nil {
			return err
		}
	}
	result := db.Model(&roleRecord{}).Where("tenant_id = ? AND id = ? AND version = ?", role.TenantID, role.ID, expectedVersion).
		Update("version", gorm.Expr("version + 1"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ports.ErrTenantRoleConflict
	}
	role.Version = expectedVersion + 1
	return nil
}

func (repository *TenantRoleRepository) AssignMember(ctx context.Context, tenantID, roleID, userID string) (domain.Role, error) {
	db := repository.database.WithContext(ctx)
	var role roleRecord
	if err := db.Where("tenant_id = ? AND id = ? AND status = ?", tenantID, roleID, domain.TenantRoleStatusActive).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Role{}, ports.ErrTenantRoleNotFound
		}
		return domain.Role{}, err
	}
	var member membershipRecord
	if err := db.Where("tenant_id = ? AND user_id = ? AND status = ?", tenantID, userID, domain.TenantMemberStatusActive).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Role{}, ports.ErrTenantRoleMember
		}
		return domain.Role{}, err
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&memberRoleRecord{TenantID: tenantID, UserID: userID, RoleID: roleID}).Error; err != nil {
		return domain.Role{}, err
	}
	return repository.Get(ctx, tenantID, roleID)
}

func (repository *TenantRoleRepository) RevokeMember(ctx context.Context, tenantID, roleID, userID string) (domain.Role, error) {
	db := repository.database.WithContext(ctx)
	var role roleRecord
	if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, roleID).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Role{}, ports.ErrTenantRoleNotFound
		}
		return domain.Role{}, err
	}
	var assignment memberRoleRecord
	if err := db.Where("tenant_id = ? AND role_id = ? AND user_id = ?", tenantID, roleID, userID).First(&assignment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return repository.Get(ctx, tenantID, roleID)
		}
		return domain.Role{}, err
	}
	if role.Name == domain.TenantOwnerRoleName {
		var activeOwners int64
		if err := db.Table("biz_member_roles mr").
			Joins("JOIN biz_memberships m ON m.tenant_id = mr.tenant_id AND m.user_id = mr.user_id AND m.status = ?", domain.TenantMemberStatusActive).
			Where("mr.tenant_id = ? AND mr.role_id = ?", tenantID, roleID).
			Count(&activeOwners).Error; err != nil {
			return domain.Role{}, err
		}
		if activeOwners <= 1 {
			return domain.Role{}, ports.ErrLastTenantOwner
		}
	}
	if err := db.Delete(&assignment).Error; err != nil {
		return domain.Role{}, err
	}
	return repository.Get(ctx, tenantID, roleID)
}

func (repository *TenantRoleRepository) roleFromRecord(ctx context.Context, row roleRecord) (domain.Role, error) {
	var grantRows []permissionGrantRecord
	if err := repository.database.WithContext(ctx).Where("tenant_id = ? AND role_id = ?", row.TenantID, row.ID).Order("permission ASC").Find(&grantRows).Error; err != nil {
		return domain.Role{}, err
	}
	role := domain.Role{ID: row.ID, TenantID: row.TenantID, Name: row.Name, Status: row.Status, Version: row.Version}
	role.Permissions = make([]domain.PermissionGrant, 0, len(grantRows))
	for _, grant := range grantRows {
		role.Permissions = append(role.Permissions, domain.PermissionGrant{TenantID: row.TenantID, RoleID: row.ID, Permission: grant.Permission, Scope: grant.Scope})
	}
	return role, nil
}

func (repository *TenantRoleRepository) classifyRoleWrite(ctx context.Context, tenantID, roleID string) error {
	var count int64
	if err := repository.database.WithContext(ctx).Model(&roleRecord{}).Where("tenant_id = ? AND id = ?", tenantID, roleID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ports.ErrTenantRoleNotFound
	}
	return ports.ErrTenantRoleConflict
}

func NewTenantRoleRepositoryFactory(database *gorm.DB) (requestscope.RepositoryFactory[ports.TenantRoleRepositories], error) {
	if database == nil {
		return nil, errors.New("access persistence: database is required")
	}
	return requestscope.GORMRepositories(func(_ context.Context, transaction *gorm.DB) (ports.TenantRoleRepositories, error) {
		role, err := NewTenantRoleRepository(transaction)
		if err != nil {
			return ports.TenantRoleRepositories{}, err
		}
		return ports.TenantRoleRepositories{Role: role}, nil
	}), nil
}
