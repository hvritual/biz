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

func (repository *TenantMemberRepository) newMembershipRecord(member domain.Membership, contactEmail string) (membershipRecord, error) {
	row := membershipRecord{TenantID: member.TenantID, UserID: member.UserID, Status: member.Status, Version: member.Version, CreatedAt: member.CreatedAt, UpdatedAt: member.UpdatedAt}
	contactEmail = strings.TrimSpace(contactEmail)
	if contactEmail == "" {
		return row, nil
	}
	normalized, err := NormalizeEmail(contactEmail)
	if err != nil {
		return membershipRecord{}, err
	}
	if repository.contactProtection == nil {
		row.Email = normalized
		return row, nil
	}
	ciphertext, lookup, version, err := repository.contactProtection.ProtectEmail(normalized)
	if err != nil {
		return membershipRecord{}, err
	}
	row.EmailCiphertext = ciphertext
	row.EmailLookupHash = &lookup
	row.EmailKeyVersion = version
	return row, nil
}

func (repository *TenantMemberRepository) newMembershipRecordWithProfile(member domain.Membership, email, phone string) (membershipRecord, error) {
	row, err := repository.newMembershipRecord(member, email)
	if err != nil {
		return membershipRecord{}, err
	}
	row.Name = member.Name
	row.EmployeeID = member.EmployeeID
	row.Position = member.Position
	row.DepartmentID = member.DepartmentID
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return row, nil
	}
	normalized, err := NormalizePhone(phone)
	if err != nil {
		return membershipRecord{}, err
	}
	if repository.contactProtection == nil {
		row.Phone = normalized
		return row, nil
	}
	ciphertext, lookup, version, err := repository.contactProtection.ProtectPhone(normalized)
	if err != nil {
		return membershipRecord{}, err
	}
	row.PhoneCiphertext = ciphertext
	row.PhoneLookupHash = &lookup
	row.PhoneKeyVersion = version
	return row, nil
}

func usernameValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (repository *TenantMemberRepository) assertMemberContactsAvailable(ctx context.Context, tenantID, userID, email, phone string) error {
	if repository == nil || repository.database == nil {
		return errors.New("access persistence: tenant member repository unavailable")
	}
	tenantID = strings.TrimSpace(tenantID)
	userID = strings.TrimSpace(userID)
	if tenantID == "" {
		return errors.New("access persistence: tenant member contact check requires tenant")
	}
	base := repository.database.WithContext(ctx).Model(&membershipRecord{}).Where("tenant_id = ?", tenantID)
	if userID != "" {
		base = base.Where("user_id <> ?", userID)
	}

	if email = strings.TrimSpace(email); email != "" && !IsMaskedContact(email) {
		normalized, err := NormalizeEmail(email)
		if err != nil {
			return err
		}
		query := base
		if repository.contactProtection != nil {
			lookup, err := repository.contactProtection.LookupEmail(normalized)
			if err != nil {
				return err
			}
			query = query.Where("(email_lookup_hash = ? OR LOWER(email) = ?)", lookup, normalized)
		} else {
			query = query.Where("LOWER(email) = ?", normalized)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ports.ErrTenantMemberContactConflict
		}
	}

	if phone = strings.TrimSpace(phone); phone != "" && !IsMaskedContact(phone) {
		normalized, err := NormalizePhone(phone)
		if err != nil {
			return err
		}
		query := base
		if repository.contactProtection != nil {
			lookup, err := repository.contactProtection.LookupPhone(normalized)
			if err != nil {
				return err
			}
			query = query.Where("(phone_lookup_hash = ? OR phone = ?)", lookup, normalized)
		} else {
			query = query.Where("phone = ?", normalized)
		}
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ports.ErrTenantMemberContactConflict
		}
	}
	return nil
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
	row, err := repository.newMembershipRecord(member, normalizedEmail)
	if err != nil {
		return domain.Membership{}, err
	}
	if err := db.Create(&row).Error; err != nil {
		return domain.Membership{}, err
	}
	return member, nil
}

func (repository *TenantMemberRepository) Create(ctx context.Context, tenantID string, input ports.TenantMemberCreateInput, now time.Time) (domain.Membership, bool, error) {
	if repository == nil || repository.database == nil {
		return domain.Membership{}, false, errors.New("access persistence: tenant member repository unavailable")
	}
	tenantID = strings.TrimSpace(tenantID)
	kind, username, err := NormalizeLoginIdentifier(input.Username)
	if tenantID == "" || err != nil || kind != LoginIdentifierUsername {
		return domain.Membership{}, false, errors.New("access persistence: member creation requires a valid username")
	}
	email := strings.TrimSpace(input.Email)
	phone := strings.TrimSpace(input.Phone)
	if email == "" && phone == "" {
		return domain.Membership{}, false, errors.New("access persistence: member creation requires email or phone")
	}
	if email != "" {
		if email, err = NormalizeEmail(email); err != nil {
			return domain.Membership{}, false, err
		}
	}
	if phone != "" {
		if phone, err = NormalizePhone(phone); err != nil {
			return domain.Membership{}, false, err
		}
	}

	if err := repository.assertMemberContactsAvailable(ctx, tenantID, "", email, phone); err != nil {
		return domain.Membership{}, false, err
	}
	db := repository.database.WithContext(ctx)
	store := repository.accountStore()
	var user userRecord
	accountCreated := false
	err = db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("LOWER(username) = ?", username).First(&user).Error
	switch {
	case err == nil:
		existingEmail, emailErr := store.userEmail(user)
		if emailErr != nil {
			return domain.Membership{}, false, emailErr
		}
		if email != "" && existingEmail != "" && !strings.EqualFold(existingEmail, email) {
			return domain.Membership{}, false, ports.ErrTenantMemberUsernameConflict
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		if email != "" {
			byEmail, authoritativeEmail, lookupErr := store.findUserByEmail(ctx, db.Clauses(clause.Locking{Strength: "UPDATE"}), email, false)
			if lookupErr == nil {
				existingUsername := usernameValue(byEmail.Username)
				if existingUsername != "" && existingUsername != username {
					return domain.Membership{}, false, ports.ErrTenantMemberContactConflict
				}
				user = byEmail
				if existingUsername == "" {
					user.Username = &username
					if err := db.Model(&userRecord{}).Where("id = ? AND username IS NULL", user.ID).Update("username", username).Error; err != nil {
						if isDuplicateMemberKey(err) {
							return domain.Membership{}, false, ports.ErrTenantMemberUsernameConflict
						}
						return domain.Membership{}, false, err
					}
				}
				if authoritativeEmail != "" && !strings.EqualFold(authoritativeEmail, email) {
					return domain.Membership{}, false, ports.ErrTenantMemberContactConflict
				}
			} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
				return domain.Membership{}, false, lookupErr
			}
		}
		if user.ID == "" {
			user, err = store.newUserRecordWithOptionalEmail(strings.TrimSpace(input.UserID), email, now)
			if err != nil {
				return domain.Membership{}, false, err
			}
			user.Username = &username
			if err := db.Create(&user).Error; err != nil {
				if isDuplicateMemberKey(err) {
					return domain.Membership{}, false, classifyMemberAccountDuplicate(err)
				}
				return domain.Membership{}, false, err
			}
			accountCreated = true
		}
	default:
		return domain.Membership{}, false, err
	}

	var existing membershipRecord
	if err := db.Where("tenant_id = ? AND user_id = ?", tenantID, user.ID).First(&existing).Error; err == nil {
		return domain.Membership{}, false, ports.ErrTenantMemberExists
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Membership{}, false, err
	}

	member := domain.NewInvitedMembership(tenantID, user.ID, email, now)
	member.Username = username
	member.Name = strings.TrimSpace(input.Name)
	member.Phone = phone
	member.EmployeeID = strings.TrimSpace(input.EmployeeID)
	member.Position = strings.TrimSpace(input.Position)
	member.DepartmentID = strings.TrimSpace(input.DepartmentID)
	if err := member.UpdateProfile(member.Name, member.Phone, member.EmployeeID, member.Position, member.DepartmentID, now); err != nil {
		return domain.Membership{}, false, err
	}
	row, err := repository.newMembershipRecordWithProfile(member, email, phone)
	if err != nil {
		return domain.Membership{}, false, err
	}
	if err := db.Create(&row).Error; err != nil {
		if isDuplicateMemberKey(err) {
			return domain.Membership{}, false, ports.ErrTenantMemberContactConflict
		}
		return domain.Membership{}, false, err
	}
	created, err := repository.memberFromRecord(ctx, row)
	if err != nil {
		return domain.Membership{}, false, err
	}
	return created, accountCreated, nil
}

func isDuplicateMemberKey(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}

func classifyMemberAccountDuplicate(err error) error {
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) {
		return err
	}
	message := strings.ToLower(mysqlErr.Message)
	if strings.Contains(message, "username") {
		return ports.ErrTenantMemberUsernameConflict
	}
	return ports.ErrTenantMemberContactConflict
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
	row, err := repository.newMembershipRecord(member, normalizedEmail)
	if err != nil {
		return domain.Membership{}, err
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

func (repository *TenantMemberRepository) List(ctx context.Context, tenantID string, filter ports.TenantMemberListQuery) (ports.TenantMemberListPage, error) {
	if repository == nil || repository.database == nil {
		return ports.TenantMemberListPage{}, errors.New("access persistence: tenant member repository unavailable")
	}
	if filter.Status == "" {
		switch strings.TrimSpace(ports.TenantMemberListStatusQuery(ctx)) {
		case "":
		case "TENANT_MEMBER_STATUS_INVITED":
			filter.Status = domain.TenantMemberStatusInvited
		case "TENANT_MEMBER_STATUS_ACTIVE":
			filter.Status = domain.TenantMemberStatusActive
		case "TENANT_MEMBER_STATUS_SUSPENDED":
			filter.Status = domain.TenantMemberStatusSuspended
		case "TENANT_MEMBER_STATUS_REMOVED":
			filter.Status = domain.TenantMemberStatusRemoved
		default:
			return ports.TenantMemberListPage{}, errors.New("access persistence: invalid tenant member status filter")
		}
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" || filter.Page == 0 || filter.PageSize == 0 || filter.PageSize > 100 {
		return ports.TenantMemberListPage{}, errors.New("access persistence: invalid tenant member list query")
	}
	countQuery, err := repository.memberListBaseQuery(ctx, tenantID, filter)
	if err != nil {
		return ports.TenantMemberListPage{}, err
	}
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return ports.TenantMemberListPage{}, err
	}

	type row struct {
		TenantID, UserID, AccountUsername, AccountEmail, AccountEmailCiphertext, AccountEmailKeyVersion, Status, Name        string
		AccountEmailLookupHash                                                                                               *string
		Email, EmailCiphertext, EmailKeyVersion, Phone, PhoneCiphertext, PhoneKeyVersion, EmployeeID, Position, DepartmentID string
		EmailLookupHash, PhoneLookupHash                                                                                     *string
		Version                                                                                                              uint64
		CreatedAt, UpdatedAt                                                                                                 time.Time
	}
	var rows []row
	listQuery, err := repository.memberListBaseQuery(ctx, tenantID, filter)
	if err != nil {
		return ports.TenantMemberListPage{}, err
	}
	offset := int((uint64(filter.Page) - 1) * uint64(filter.PageSize))
	if err := listQuery.
		Select("m.tenant_id, m.user_id, COALESCE(u.username, '') AS account_username, u.email AS account_email, u.email_ciphertext AS account_email_ciphertext, u.email_lookup_hash AS account_email_lookup_hash, u.email_key_version AS account_email_key_version, m.status, m.name, m.email, m.email_ciphertext, m.email_lookup_hash, m.email_key_version, m.phone, m.phone_ciphertext, m.phone_lookup_hash, m.phone_key_version, m.employee_id, m.position, m.department_id, m.version, m.created_at, m.updated_at").
		Order("m.created_at ASC, m.user_id ASC").
		Limit(int(filter.PageSize)).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return ports.TenantMemberListPage{}, err
	}
	members := make([]domain.Membership, 0, len(rows))
	for _, value := range rows {
		membershipRow := membershipRecord{TenantID: value.TenantID, UserID: value.UserID, Status: value.Status, Name: value.Name, Email: value.Email, EmailCiphertext: value.EmailCiphertext, EmailLookupHash: value.EmailLookupHash, EmailKeyVersion: value.EmailKeyVersion, Phone: value.Phone, PhoneCiphertext: value.PhoneCiphertext, PhoneLookupHash: value.PhoneLookupHash, PhoneKeyVersion: value.PhoneKeyVersion, EmployeeID: value.EmployeeID, Position: value.Position, DepartmentID: value.DepartmentID, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
		accountRow := userRecord{ID: value.UserID, Email: value.AccountEmail, EmailCiphertext: value.AccountEmailCiphertext, EmailLookupHash: value.AccountEmailLookupHash, EmailKeyVersion: value.AccountEmailKeyVersion}
		email, err := repository.displayMemberEmail(membershipRow, accountRow)
		if err != nil {
			return ports.TenantMemberListPage{}, err
		}
		phone, err := repository.displayPhone(membershipRow)
		if err != nil {
			return ports.TenantMemberListPage{}, err
		}
		members = append(members, domain.Membership{TenantID: value.TenantID, UserID: value.UserID, Username: value.AccountUsername, Email: email, Status: value.Status, Name: value.Name, Phone: phone, EmployeeID: value.EmployeeID, Position: value.Position, DepartmentID: value.DepartmentID, Version: value.Version, DerivedDataScope: domain.DataScopeNone, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt})
	}
	if err := repository.enrichMemberAccess(ctx, members); err != nil {
		return ports.TenantMemberListPage{}, err
	}
	return ports.TenantMemberListPage{Members: members, Total: uint64(total)}, nil
}

func (repository *TenantMemberRepository) memberListBaseQuery(ctx context.Context, tenantID string, filter ports.TenantMemberListQuery) (*gorm.DB, error) {
	query := repository.database.WithContext(ctx).
		Table("biz_memberships m").
		Joins("JOIN biz_users u ON u.id = m.user_id").
		Where("m.tenant_id = ? AND m.status <> ?", tenantID, domain.TenantMemberStatusRemoved)

	if filter.Status != "" {
		if filter.Status == domain.TenantMemberStatusRemoved {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("m.status = ?", filter.Status)
		}
	}
	if departmentID := strings.TrimSpace(filter.DepartmentID); departmentID != "" {
		query = query.Where("m.department_id = ?", departmentID)
	}
	if roleID := strings.TrimSpace(filter.RoleID); roleID != "" {
		query = query.Where(
			"EXISTS (SELECT 1 FROM biz_member_roles mr WHERE mr.tenant_id = m.tenant_id AND mr.user_id = m.user_id AND mr.role_id = ?)",
			roleID,
		)
	}
	keyword := strings.TrimSpace(filter.Query)
	if keyword == "" {
		return query, nil
	}
	if strings.Contains(keyword, "@") {
		normalized, err := NormalizeEmail(keyword)
		if err != nil {
			return query.Where("1 = 0"), nil
		}
		if repository.contactProtection != nil {
			lookup, err := repository.contactProtection.LookupEmail(normalized)
			if err != nil {
				return nil, err
			}
			return query.Where(
				"(m.email_lookup_hash = ? OR u.email_lookup_hash = ? OR LOWER(m.email) = ? OR LOWER(u.email) = ?)",
				lookup, lookup, normalized, normalized,
			), nil
		}
		return query.Where("(LOWER(m.email) = ? OR LOWER(u.email) = ?)", normalized, normalized), nil
	}
	if looksLikePhoneIdentifier(keyword) {
		normalized, err := NormalizePhone(keyword)
		if err != nil || normalized == "" {
			return query.Where("1 = 0"), nil
		}
		if repository.contactProtection != nil {
			lookup, err := repository.contactProtection.LookupPhone(normalized)
			if err != nil {
				return nil, err
			}
			return query.Where("(m.phone_lookup_hash = ? OR m.phone = ?)", lookup, normalized), nil
		}
		return query.Where("m.phone = ?", normalized), nil
	}
	pattern := "%" + strings.ToLower(keyword) + "%"
	return query.Where(
		"(LOWER(m.name) LIKE ? OR LOWER(COALESCE(u.username, '')) LIKE ? OR LOWER(m.employee_id) LIKE ?)",
		pattern, pattern, pattern,
	), nil
}

func (repository *TenantMemberRepository) Update(ctx context.Context, member *domain.Membership, expectedVersion uint64) error {
	if repository == nil || repository.database == nil || member == nil || expectedVersion == 0 {
		return errors.New("access persistence: member update requires repository, value and version")
	}
	if err := repository.assertMemberContactsAvailable(ctx, member.TenantID, member.UserID, member.Email, member.Phone); err != nil {
		return err
	}
	updates := map[string]any{"status": member.Status, "name": member.Name, "employee_id": member.EmployeeID, "position": member.Position, "department_id": member.DepartmentID, "updated_at": member.UpdatedAt, "version": gorm.Expr("version + 1")}
	if repository.contactProtection == nil {
		if !IsMaskedContact(member.Email) {
			if email := strings.TrimSpace(member.Email); email != "" {
				normalized, err := NormalizeEmail(email)
				if err != nil {
					return err
				}
				updates["email"] = normalized
				member.Email = normalized
			} else {
				updates["email"] = ""
			}
		}
		if !IsMaskedContact(member.Phone) {
			if strings.TrimSpace(member.Phone) == "" {
				updates["phone"] = ""
				member.Phone = ""
			} else {
				normalized, err := NormalizePhone(member.Phone)
				if err != nil {
					return err
				}
				updates["phone"] = normalized
				member.Phone = normalized
			}
		}
	} else {
		if !IsMaskedContact(member.Email) {
			if strings.TrimSpace(member.Email) == "" {
				updates["email"] = ""
				updates["email_ciphertext"] = ""
				updates["email_lookup_hash"] = nil
				updates["email_key_version"] = ""
			} else {
				ciphertext, lookup, version, err := repository.contactProtection.ProtectEmail(member.Email)
				if err != nil {
					return err
				}
				updates["email"] = ""
				updates["email_ciphertext"] = ciphertext
				updates["email_lookup_hash"] = lookup
				updates["email_key_version"] = version
				member.Email = MaskEmail(member.Email)
			}
		}
		if !IsMaskedContact(member.Phone) {
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
	}
	err := repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&membershipRecord{}).
			Where("tenant_id = ? AND user_id = ? AND version = ?", member.TenantID, member.UserID, expectedVersion).
			Updates(updates)
		if result.Error != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(result.Error, &mysqlErr) && mysqlErr.Number == 1062 {
				return ports.ErrTenantMemberContactConflict
			}
			return result.Error
		}
		if result.RowsAffected != 1 {
			var count int64
			if err := tx.Model(&membershipRecord{}).Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ports.ErrTenantMemberNotFound
			}
			return ports.ErrTenantMemberConflict
		}
		if member.Status == domain.TenantMemberStatusSuspended || member.Status == domain.TenantMemberStatusRemoved {
			if err := revokeWebSessionsForTenantMember(ctx, tx, member.UserID, member.TenantID, "membership_"+strings.ToLower(member.Status)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	member.Version = expectedVersion + 1
	return nil
}

func (repository *TenantMemberRepository) memberFromRecord(ctx context.Context, row membershipRecord) (domain.Membership, error) {
	var user userRecord
	if err := repository.database.WithContext(ctx).Where("id = ?", row.UserID).First(&user).Error; err != nil {
		return domain.Membership{}, err
	}
	email, err := repository.displayMemberEmail(row, user)
	if err != nil {
		return domain.Membership{}, err
	}
	phone, err := repository.displayPhone(row)
	if err != nil {
		return domain.Membership{}, err
	}
	members := []domain.Membership{{TenantID: row.TenantID, UserID: row.UserID, Username: usernameValue(user.Username), Email: email, Status: row.Status, Name: row.Name, Phone: phone, EmployeeID: row.EmployeeID, Position: row.Position, DepartmentID: row.DepartmentID, Version: row.Version, DerivedDataScope: domain.DataScopeNone, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}}
	if err := repository.enrichMemberAccess(ctx, members); err != nil {
		return domain.Membership{}, err
	}
	return members[0], nil
}

func (repository *TenantMemberRepository) displayMemberEmail(row membershipRecord, account userRecord) (string, error) {
	if strings.TrimSpace(row.EmailCiphertext) != "" || strings.TrimSpace(row.EmailKeyVersion) != "" || row.EmailLookupHash != nil {
		if repository.contactProtection == nil {
			return "", ErrSensitiveDataKeyUnavailable
		}
		plain, err := repository.contactProtection.DecryptEmail(row.EmailCiphertext, row.EmailKeyVersion)
		if err != nil {
			return "", err
		}
		return MaskEmail(plain), nil
	}
	if strings.TrimSpace(row.Email) != "" {
		normalized, err := NormalizeEmail(row.Email)
		if err != nil {
			return "", err
		}
		if repository.contactProtection != nil {
			return MaskEmail(normalized), nil
		}
		return normalized, nil
	}
	return repository.accountStore().displayUserEmail(account)
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
	return newTenantMemberRepositoryFactory(database, nil, nil)
}

func NewTenantMemberRepositoryFactoryWithContactProtection(database *gorm.DB, protection *ContactProtection) (requestscope.RepositoryFactory[ports.TenantMemberRepositories], error) {
	if protection == nil {
		return nil, ErrSensitiveDataKeyUnavailable
	}
	return newTenantMemberRepositoryFactory(database, protection, nil)
}

func NewTenantMemberRepositoryFactoryWithSecurity(database *gorm.DB, contactProtection *ContactProtection, verificationProtection *VerificationProtection) (requestscope.RepositoryFactory[ports.TenantMemberRepositories], error) {
	if verificationProtection == nil {
		return nil, ErrVerificationKeyUnavailable
	}
	return newTenantMemberRepositoryFactory(database, contactProtection, verificationProtection)
}

func newTenantMemberRepositoryFactory(database *gorm.DB, contactProtection *ContactProtection, verificationProtection *VerificationProtection) (requestscope.RepositoryFactory[ports.TenantMemberRepositories], error) {
	if database == nil {
		return nil, errors.New("access persistence: database is required")
	}
	return requestscope.GORMRepositories(func(_ context.Context, transaction *gorm.DB) (ports.TenantMemberRepositories, error) {
		var member *TenantMemberRepository
		var err error
		if contactProtection == nil {
			member, err = NewTenantMemberRepository(transaction)
		} else {
			member, err = NewTenantMemberRepositoryWithContactProtection(transaction, contactProtection)
		}
		if err != nil {
			return ports.TenantMemberRepositories{}, err
		}
		var activation ports.TenantMemberActivationRepository
		var lifecycle ports.TenantMemberLifecycleNotificationRepository
		if verificationProtection != nil {
			value, err := NewTenantMemberActivationRepository(transaction, verificationProtection)
			if err != nil {
				return ports.TenantMemberRepositories{}, err
			}
			activation = value
			lifecycleValue, err := NewTenantMemberLifecycleNotificationRepository(transaction, contactProtection, verificationProtection)
			if err != nil {
				return ports.TenantMemberRepositories{}, err
			}
			lifecycle = lifecycleValue
		}
		return ports.TenantMemberRepositories{Member: member, BusinessScope: member, Activation: activation, Lifecycle: lifecycle}, nil
	}), nil
}
