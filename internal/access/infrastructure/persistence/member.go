package persistence

import (
	"context"
	"errors"
	"sort"
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
	database          *gorm.DB
	contactProtection *ContactProtection
}

func NewTenantMemberRepository(database *gorm.DB) (*TenantMemberRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: tenant member database is required")
	}
	return &TenantMemberRepository{database: database}, nil
}

func NewTenantMemberRepositoryWithContactProtection(database *gorm.DB, protection *ContactProtection) (*TenantMemberRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: tenant member database is required")
	}
	if protection == nil {
		return nil, ErrSensitiveDataKeyUnavailable
	}
	return &TenantMemberRepository{database: database, contactProtection: protection}, nil
}

func (repository *TenantMemberRepository) accountStore() *Store {
	return &Store{database: repository.database, contactProtection: repository.contactProtection}
}

func (repository *TenantMemberRepository) Invite(ctx context.Context, tenantID, proposedUserID, email string, now time.Time) (domain.Membership, error) {
	if repository == nil || repository.database == nil {
		return domain.Membership{}, errors.New("access persistence: tenant member repository unavailable")
	}
	tenantID, proposedUserID = strings.TrimSpace(tenantID), strings.TrimSpace(proposedUserID)
	normalizedEmail, err := NormalizeEmail(email)
	if tenantID == "" || proposedUserID == "" || err != nil {
		return domain.Membership{}, errors.New("access persistence: invite requires tenant, user and valid email")
	}
	db := repository.database.WithContext(ctx)
	store := repository.accountStore()
	user, _, err := store.findUserByEmail(ctx, db, normalizedEmail, false)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Membership{}, err
		}
		user, err = store.newUserRecord(proposedUserID, normalizedEmail, now)
		if err != nil {
			return domain.Membership{}, err
		}
		if createErr := db.Create(&user).Error; createErr != nil {
			var mysqlErr *mysql.MySQLError
			if !errors.As(createErr, &mysqlErr) || mysqlErr.Number != 1062 {
				return domain.Membership{}, createErr
			}
			user, _, err = store.findUserByEmail(ctx, db.Clauses(clause.Locking{Strength: "UPDATE"}), normalizedEmail, false)
			if err != nil {
				return domain.Membership{}, err
			}
		}
	}
	var existing membershipRecord
	if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, user.ID).First(&existing).Error; err == nil {
		return domain.Membership{}, ports.ErrTenantMemberExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Membership{}, err
	}
	displayEmail, err := store.displayUserEmail(user)
	if err != nil {
		return domain.Membership{}, err
	}
	member := domain.NewInvitedMembership(tenantID, user.ID, displayEmail, now)
	row := membershipRecord{TenantID: member.TenantID, UserID: member.UserID, Status: member.Status, Version: member.Version, CreatedAt: member.CreatedAt, UpdatedAt: member.UpdatedAt}
	if err := db.Create(&row).Error; err != nil {
		return domain.Membership{}, err
	}
	return member, nil
}

func (repository *TenantMemberRepository) Bootstrap(ctx context.Context, tenantID, userID, email string, now time.Time) (domain.Membership, error) {
	if repository == nil || repository.database == nil {
		return domain.Membership{}, errors.New("access persistence: tenant member repository unavailable")
	}
	tenantID, userID = strings.TrimSpace(tenantID), strings.TrimSpace(userID)
	normalizedEmail, err := NormalizeEmail(email)
	if tenantID == "" || userID == "" || err != nil {
		return domain.Membership{}, errors.New("access persistence: bootstrap requires tenant, user and valid email")
	}
	db := repository.database.WithContext(ctx)
	store := repository.accountStore()
	var user userRecord
	if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Membership{}, err
		}
		user, err = store.newUserRecord(userID, normalizedEmail, now)
		if err != nil {
			return domain.Membership{}, err
		}
		if err := db.Create(&user).Error; err != nil {
			return domain.Membership{}, err
		}
	} else {
		existingEmail, emailErr := store.userEmail(user)
		if emailErr != nil || !strings.EqualFold(existingEmail, normalizedEmail) {
			return domain.Membership{}, ports.ErrTenantMemberConflict
		}
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
	displayEmail, err := store.displayUserEmail(user)
	if err != nil {
		return domain.Membership{}, err
	}
	member := domain.NewActiveMembership(tenantID, userID, displayEmail, now)
	row := membershipRecord{TenantID: member.TenantID, UserID: member.UserID, Status: member.Status, Version: member.Version, CreatedAt: member.CreatedAt, UpdatedAt: member.UpdatedAt}
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
		TenantID, UserID, Email, EmailCiphertext, EmailKeyVersion, Status, Name string
		EmailLookupHash                                                       *string
		Phone, PhoneCiphertext, PhoneKeyVersion, EmployeeID, Position, DepartmentID string
		PhoneLookupHash                                                       *string
		Version                                                               uint64
		CreatedAt, UpdatedAt                                                  time.Time
	}
	var rows []row
	if err := repository.database.WithContext(ctx).Table("biz_memberships m").
		Select("m.tenant_id, m.user_id, u.email, u.email_ciphertext, u.email_lookup_hash, u.email_key_version, m.status, m.name, m.phone, m.phone_ciphertext, m.phone_lookup_hash, m.phone_key_version, m.employee_id, m.position, m.department_id, m.version, m.created_at, m.updated_at").
		Joins("JOIN biz_users u ON u.id = m.user_id").Where("m.tenant_id = ?", tenantID).
		Order("m.created_at ASC, m.user_id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	store := repository.accountStore()
	members := make([]domain.Membership, 0, len(rows))
	for _, value := range rows {
		email, err := store.displayUserEmail(userRecord{ID: value.UserID, Email: value.Email, EmailCiphertext: value.EmailCiphertext, EmailLookupHash: value.EmailLookupHash, EmailKeyVersion: value.EmailKeyVersion})
		if err != nil {
			return nil, err
		}
		phone, err := repository.displayPhone(membershipRecord{TenantID: value.TenantID, UserID: value.UserID, Phone: value.Phone, PhoneCiphertext: value.PhoneCiphertext, PhoneLookupHash: value.PhoneLookupHash, PhoneKeyVersion: value.PhoneKeyVersion})
		if err != nil {
			return nil, err
		}
		members = append(members, domain.Membership{TenantID: value.TenantID, UserID: value.UserID, Email: email, Status: value.Status, Name: value.Name, Phone: phone, EmployeeID: value.EmployeeID, Position: value.Position, DepartmentID: value.DepartmentID, Version: value.Version, DerivedDataScope: domain.DataScopeNone, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt})
	}
	if err := repository.enrichMemberAccess(ctx, members); err != nil {
		return nil, err
	}
	return members, nil
}

func (repository *TenantMemberRepository) Update(ctx context.Context, member *domain.Membership, expectedVersion uint64) error {
	if repository == nil || repository.database == nil || member == nil || expectedVersion == 0 {
		return errors.New("access persistence: member update requires repository, value and version")
	}
	updates := map[string]any{"status": member.Status, "name": member.Name, "employee_id": member.EmployeeID, "position": member.Position, "department_id": member.DepartmentID, "updated_at": member.UpdatedAt, "version": gorm.Expr("version + 1")}
	if repository.contactProtection == nil {
		updates["phone"] = member.Phone
	} else if !IsMaskedContact(member.Phone) {
		if strings.TrimSpace(member.Phone) == "" {
			updates["phone"] = ""
			updates["phone_ciphertext"] = ""
			updates["phone_lookup_hash"] = nil
			updates["phone_key_version"] = ""
		} else {
			ciphertext, lookup, version, err := repository.contactProtection.ProtectPhone(member.Phone)
			if err != nil {
				return err
			}
			updates["phone"] = ""
			updates["phone_ciphertext"] = ciphertext
			updates["phone_lookup_hash"] = lookup
			updates["phone_key_version"] = version
			member.Phone = MaskPhone(member.Phone)
		}
	}
	result := repository.database.WithContext(ctx).Model(&membershipRecord{}).
		Where("tenant_id = ? AND user_id = ? AND version = ?", member.TenantID, member.UserID, expectedVersion).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		var count int64
		if err := repository.database.WithContext(ctx).Model(&membershipRecord{}).Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).Count(&count).Error; err != nil {
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
	email, err := repository.accountStore().displayUserEmail(user)
	if err != nil {
		return domain.Membership{}, err
	}
	phone, err := repository.displayPhone(row)
	if err != nil {
		return domain.Membership{}, err
	}
	members := []domain.Membership{{TenantID: row.TenantID, UserID: row.UserID, Email: email, Status: row.Status, Name: row.Name, Phone: phone, EmployeeID: row.EmployeeID, Position: row.Position, DepartmentID: row.DepartmentID, Version: row.Version, DerivedDataScope: domain.DataScopeNone, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}}
	if err := repository.enrichMemberAccess(ctx, members); err != nil {
		return domain.Membership{}, err
	}
	return members[0], nil
}

func (repository *TenantMemberRepository) displayPhone(row membershipRecord) (string, error) {
	if strings.TrimSpace(row.PhoneCiphertext) != "" || strings.TrimSpace(row.PhoneKeyVersion) != "" || row.PhoneLookupHash != nil {
		if repository.contactProtection == nil {
			return "", ErrSensitiveDataKeyUnavailable
		}
		plain, err := repository.contactProtection.DecryptPhone(row.PhoneCiphertext, row.PhoneKeyVersion)
		if err != nil {
			return "", err
		}
		return MaskPhone(plain), nil
	}
	if repository.contactProtection != nil && strings.TrimSpace(row.Phone) != "" {
		return MaskPhone(row.Phone), nil
	}
	return row.Phone, nil
}

func (repository *TenantMemberRepository) enrichMemberAccess(ctx context.Context, members []domain.Membership) error {
	if len(members) == 0 {
		return nil
	}
	tenantID := members[0].TenantID
	userIDs := make([]string, 0, len(members))
	index := make(map[string]int, len(members))
	for i := range members {
		userIDs = append(userIDs, members[i].UserID)
		index[members[i].UserID] = i
		members[i].Roles = nil
		members[i].DerivedDataScope = domain.DataScopeNone
	}
	type accessRow struct{ UserID, RoleID, RoleName, RoleStatus, Scope string }
	var rows []accessRow
	if err := repository.database.WithContext(ctx).Table("biz_member_roles mr").
		Select("mr.user_id, r.id AS role_id, r.name AS role_name, r.status AS role_status, COALESCE(pg.scope, '') AS scope").
		Joins("JOIN biz_roles r ON r.id = mr.role_id AND r.tenant_id = mr.tenant_id").
		Joins("LEFT JOIN biz_permission_grants pg ON pg.role_id = r.id AND pg.tenant_id = r.tenant_id").
		Where("mr.tenant_id = ? AND mr.user_id IN ?", tenantID, userIDs).
		Order("mr.user_id ASC, r.name ASC, r.id ASC").Scan(&rows).Error; err != nil {
		return err
	}
	seen := map[string]map[string]bool{}
	for _, row := range rows {
		i, ok := index[row.UserID]
		if !ok {
			continue
		}
		if seen[row.UserID] == nil {
			seen[row.UserID] = map[string]bool{}
		}
		if !seen[row.UserID][row.RoleID] {
			members[i].Roles = append(members[i].Roles, domain.MemberRoleSummary{ID: row.RoleID, Name: row.RoleName, Status: row.RoleStatus})
			seen[row.UserID][row.RoleID] = true
		}
		if row.RoleStatus == domain.TenantRoleStatusActive && scopeRank(domain.DataScope(row.Scope)) > scopeRank(members[i].DerivedDataScope) {
			members[i].DerivedDataScope = domain.DataScope(row.Scope)
		}
	}
	for i := range members {
		sort.Slice(members[i].Roles, func(a, b int) bool {
			if members[i].Roles[a].Name == members[i].Roles[b].Name {
				return members[i].Roles[a].ID < members[i].Roles[b].ID
			}
			return members[i].Roles[a].Name < members[i].Roles[b].Name
		})
	}
	return nil
}

func scopeRank(scope domain.DataScope) int {
	switch scope {
	case domain.DataScopeSelf:
		return 1
	case domain.DataScopeSites:
		return 2
	case domain.DataScopeAll:
		return 3
	default:
		return 0
	}
}

func NewTenantMemberRepositoryFactory(database *gorm.DB) (requestscope.RepositoryFactory[ports.TenantMemberRepositories], error) {
	return NewTenantMemberRepositoryFactoryWithContactProtection(database, nil)
}

func NewTenantMemberRepositoryFactoryWithContactProtection(database *gorm.DB, protection *ContactProtection) (requestscope.RepositoryFactory[ports.TenantMemberRepositories], error) {
	if database == nil {
		return nil, errors.New("access persistence: database is required")
	}
	return requestscope.GORMRepositories(func(_ context.Context, transaction *gorm.DB) (ports.TenantMemberRepositories, error) {
		var member *TenantMemberRepository
		var err error
		if protection == nil {
			member, err = NewTenantMemberRepository(transaction)
		} else {
			member, err = NewTenantMemberRepositoryWithContactProtection(transaction, protection)
		}
		if err != nil {
			return ports.TenantMemberRepositories{}, err
		}
		return ports.TenantMemberRepositories{Member: member}, nil
	}), nil
}
