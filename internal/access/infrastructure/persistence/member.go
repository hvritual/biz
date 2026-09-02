package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/requestscope"
)

type TenantMemberRepository struct {
	database *gorm.DB
}

func NewTenantMemberRepository(database *gorm.DB) (*TenantMemberRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: tenant member database is required")
	}
	return &TenantMemberRepository{database: database}, nil
}

func (repository *TenantMemberRepository) Invite(ctx context.Context, tenantID, proposedUserID, email string, now time.Time) (domain.Membership, error) {
	if repository == nil || repository.database == nil {
		return domain.Membership{}, errors.New("access persistence: tenant member repository unavailable")
	}
	tenantID, proposedUserID, email = strings.TrimSpace(tenantID), strings.TrimSpace(proposedUserID), strings.TrimSpace(email)
	if tenantID == "" || proposedUserID == "" || email == "" {
		return domain.Membership{}, errors.New("access persistence: invite requires tenant, user and email")
	}
	db := repository.database.WithContext(ctx)
	var user userRecord
	err := db.Where("email = ?", email).First(&user).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Membership{}, err
		}
		user = userRecord{ID: proposedUserID, Email: email, Status: "active", CreatedAt: now}
		if createErr := db.Create(&user).Error; createErr != nil {
			var mysqlErr *mysql.MySQLError
			if !errors.As(createErr, &mysqlErr) || mysqlErr.Number != 1062 {
				return domain.Membership{}, createErr
			}
			// A concurrent invite won the unique email race. Reset the losing
			// candidate so GORM cannot retain its primary key as an implicit
			// predicate, then perform a locking current read. FOR UPDATE is
			// intentional here: under MySQL REPEATABLE READ it observes the
			// winner committed before the duplicate-key result was returned.
			user = userRecord{}
			if lookupErr := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("email = ?", email).First(&user).Error; lookupErr != nil {
				return domain.Membership{}, lookupErr
			}
		}
	}
	var existing membershipRecord
	if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, user.ID).First(&existing).Error; err == nil {
		return domain.Membership{}, ports.ErrTenantMemberExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Membership{}, err
	}
	member := domain.NewInvitedMembership(tenantID, user.ID, user.Email, now)
	row := membershipRecord{
		TenantID: member.TenantID, UserID: member.UserID, Status: member.Status,
		Version: member.Version, CreatedAt: member.CreatedAt, UpdatedAt: member.UpdatedAt,
	}
	if err := db.Create(&row).Error; err != nil {
		return domain.Membership{}, err
	}
	return member, nil
}

func (repository *TenantMemberRepository) Bootstrap(ctx context.Context, tenantID, userID, email string, now time.Time) (domain.Membership, error) {
	if repository == nil || repository.database == nil {
		return domain.Membership{}, errors.New("access persistence: tenant member repository unavailable")
	}
	tenantID, userID, email = strings.TrimSpace(tenantID), strings.TrimSpace(userID), strings.TrimSpace(email)
	if tenantID == "" || userID == "" || email == "" {
		return domain.Membership{}, errors.New("access persistence: bootstrap requires tenant, user and email")
	}
	db := repository.database.WithContext(ctx)
	var user userRecord
	if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Membership{}, err
		}
		user = userRecord{ID: userID, Email: email, Status: "active", CreatedAt: now}
		if err := db.Create(&user).Error; err != nil {
			return domain.Membership{}, err
		}
	} else if user.Email != email {
		return domain.Membership{}, ports.ErrTenantMemberConflict
	}
	var existing membershipRecord
	if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&existing).Error; err == nil {
		if existing.Status != domain.TenantMemberStatusActive {
			return domain.Membership{}, ports.ErrTenantMemberConflict
		}
		return repository.memberFromRecord(ctx, existing)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Membership{}, err
	}
	member := domain.NewActiveMembership(tenantID, userID, email, now)
	row := membershipRecord{
		TenantID: member.TenantID, UserID: member.UserID, Status: member.Status,
		Version: member.Version, CreatedAt: member.CreatedAt, UpdatedAt: member.UpdatedAt,
	}
	if err := db.Create(&row).Error; err != nil {
		return domain.Membership{}, err
	}
	return member, nil
}

func (repository *TenantMemberRepository) Get(ctx context.Context, tenantID, userID string) (domain.Membership, error) {
	if repository == nil || repository.database == nil {
		return domain.Membership{}, errors.New("access persistence: tenant member repository unavailable")
	}
	var row membershipRecord
	if err := repository.database.WithContext(ctx).Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Membership{}, ports.ErrTenantMemberNotFound
		}
		return domain.Membership{}, err
	}
	return repository.memberFromRecord(ctx, row)
}

func (repository *TenantMemberRepository) List(ctx context.Context, tenantID string) ([]domain.Membership, error) {
	if repository == nil || repository.database == nil {
		return nil, errors.New("access persistence: tenant member repository unavailable")
	}
	type row struct {
		TenantID  string
		UserID    string
		Email     string
		Status    string
		Version   uint64
		CreatedAt time.Time
		UpdatedAt time.Time
	}
	var rows []row
	if err := repository.database.WithContext(ctx).Table("biz_memberships m").
		Select("m.tenant_id, m.user_id, u.email, m.status, m.version, m.created_at, m.updated_at").
		Joins("JOIN biz_users u ON u.id = m.user_id").
		Where("m.tenant_id = ?", tenantID).
		Order("m.created_at ASC, m.user_id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	members := make([]domain.Membership, 0, len(rows))
	for _, value := range rows {
		members = append(members, domain.Membership{
			TenantID: value.TenantID, UserID: value.UserID, Email: value.Email,
			Status: value.Status, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		})
	}
	return members, nil
}

func (repository *TenantMemberRepository) Update(ctx context.Context, member *domain.Membership, expectedVersion uint64) error {
	if repository == nil || repository.database == nil || member == nil || expectedVersion == 0 {
		return errors.New("access persistence: member update requires repository, value and version")
	}
	result := repository.database.WithContext(ctx).Model(&membershipRecord{}).
		Where("tenant_id = ? AND user_id = ? AND version = ?", member.TenantID, member.UserID, expectedVersion).
		Updates(map[string]any{
			"status": member.Status,
			"updated_at": member.UpdatedAt,
			"version": gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		var count int64
		if err := repository.database.WithContext(ctx).Model(&membershipRecord{}).
			Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ports.ErrTenantMemberNotFound
		}
		return ports.ErrTenantMemberConflict
	}
	member.Version = expectedVersion + 1
	return nil
}

func (repository *TenantMemberRepository) memberFromRecord(ctx context.Context, row membershipRecord) (domain.Membership, error) {
	var user userRecord
	if err := repository.database.WithContext(ctx).Where("id = ?", row.UserID).First(&user).Error; err != nil {
		return domain.Membership{}, err
	}
	return domain.Membership{
		TenantID: row.TenantID, UserID: row.UserID, Email: user.Email,
		Status: row.Status, Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func NewTenantMemberRepositoryFactory(database *gorm.DB) (requestscope.RepositoryFactory[ports.TenantMemberRepositories], error) {
	if database == nil {
		return nil, errors.New("access persistence: database is required")
	}
	return requestscope.GORMRepositories(func(_ context.Context, transaction *gorm.DB) (ports.TenantMemberRepositories, error) {
		member, err := NewTenantMemberRepository(transaction)
		if err != nil {
			return ports.TenantMemberRepositories{}, err
		}
		return ports.TenantMemberRepositories{Member: member}, nil
	}), nil
}
