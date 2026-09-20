package persistence

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type memberRemovedRoleSnapshotRecord struct {
	TenantID       string    `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID         string    `gorm:"column:user_id;primaryKey;size:64"`
	RoleID         string    `gorm:"column:role_id;primaryKey;size:160"`
	RemovedVersion uint64    `gorm:"column:removed_version;not null;index"`
	CapturedAt     time.Time `gorm:"column:captured_at;type:datetime(6);not null"`
}

func (memberRemovedRoleSnapshotRecord) TableName() string { return "biz_member_removed_role_snapshots" }

type memberRemovedSiteSnapshotRecord struct {
	TenantID       string    `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID         string    `gorm:"column:user_id;primaryKey;size:64"`
	SiteID         string    `gorm:"column:site_id;primaryKey;size:64"`
	RemovedVersion uint64    `gorm:"column:removed_version;not null;index"`
	CapturedAt     time.Time `gorm:"column:captured_at;type:datetime(6);not null"`
}

func (memberRemovedSiteSnapshotRecord) TableName() string { return "biz_member_removed_site_snapshots" }

type TenantMemberLifecycleNotificationRepository struct {
	database          *gorm.DB
	contactProtection *ContactProtection
	verification      *VerificationRepository
}

func NewTenantMemberLifecycleNotificationRepository(
	database *gorm.DB,
	contactProtection *ContactProtection,
	verificationProtection *VerificationProtection,
) (*TenantMemberLifecycleNotificationRepository, error) {
	if database == nil {
		return nil, errors.New("access persistence: member lifecycle notification database is required")
	}
	if verificationProtection == nil {
		return nil, ErrVerificationKeyUnavailable
	}
	verification, err := NewVerificationRepository(database, verificationProtection)
	if err != nil {
		return nil, err
	}
	return &TenantMemberLifecycleNotificationRepository{
		database: database, contactProtection: contactProtection, verification: verification,
	}, nil
}

func (repository *TenantMemberLifecycleNotificationRepository) Notify(
	ctx context.Context,
	input ports.TenantMemberLifecycleNotificationInput,
) (domain.NotificationDeliveryReceipt, error) {
	if repository == nil || repository.verification == nil {
		return domain.NotificationDeliveryReceipt{}, domain.ErrNotificationUnavailable
	}
	input.TenantID = strings.TrimSpace(input.TenantID)
	input.UserID = strings.TrimSpace(input.UserID)
	input.Status = strings.TrimSpace(input.Status)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.TenantID == "" || input.UserID == "" || input.Status == "" || input.Version == 0 {
		return domain.NotificationDeliveryReceipt{}, domain.ErrVerificationInvalid
	}
	if len([]rune(input.Reason)) > 500 {
		return domain.NotificationDeliveryReceipt{}, domain.ErrVerificationInvalid
	}
	channel, destination, err := repository.memberNotificationDestination(ctx, input.TenantID, input.UserID)
	if err != nil {
		return domain.NotificationDeliveryReceipt{}, err
	}
	secret := "status=" + input.Status
	if input.Reason != "" {
		secret += "\nreason=" + input.Reason
	}
	return repository.verification.EnqueueSecurityNotification(ctx, domain.SecurityNotificationRequest{
		BusinessEventID: fmt.Sprintf("member-lifecycle/%s/%s/%s/%d", input.TenantID, input.UserID, input.Status, input.Version),
		Kind:            domain.SecurityNotificationMemberLifecycle,
		Purpose:         domain.VerificationPurposeMemberLifecycle,
		UserID:          input.UserID,
		TenantID:        input.TenantID,
		FlowID:          input.TenantID + "/" + input.UserID,
		Channel:         channel,
		Destination:     destination,
		Secret:          secret,
		ExpiresAt:       time.Now().UTC().Add(24 * time.Hour),
	})
}

func (repository *TenantMemberLifecycleNotificationRepository) memberNotificationDestination(
	ctx context.Context,
	tenantID, userID string,
) (domain.SecurityNotificationChannel, string, error) {
	var member membershipRecord
	if err := repository.database.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&member).Error; err != nil {
		return "", "", err
	}
	var account userRecord
	if err := repository.database.WithContext(ctx).Where("id = ?", userID).First(&account).Error; err != nil {
		return "", "", err
	}
	email := ""
	var err error
	switch {
	case member.EmailCiphertext != "":
		if repository.contactProtection == nil {
			return "", "", ErrSensitiveDataKeyUnavailable
		}
		email, err = repository.contactProtection.DecryptEmail(member.EmailCiphertext, member.EmailKeyVersion)
	case strings.TrimSpace(member.Email) != "":
		email, err = NormalizeEmail(member.Email)
	default:
		email, err = (&Store{database: repository.database, contactProtection: repository.contactProtection}).userEmail(account)
	}
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(email) != "" {
		return domain.SecurityNotificationEmail, email, nil
	}
	phone := ""
	switch {
	case member.PhoneCiphertext != "":
		if repository.contactProtection == nil {
			return "", "", ErrSensitiveDataKeyUnavailable
		}
		phone, err = repository.contactProtection.DecryptPhone(member.PhoneCiphertext, member.PhoneKeyVersion)
	case strings.TrimSpace(member.Phone) != "":
		phone, err = NormalizePhone(member.Phone)
	}
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(phone) != "" {
		return domain.SecurityNotificationSMS, phone, nil
	}
	return "", "", domain.ErrNotificationUnavailable
}

func (repository *TenantMemberRepository) ListRemoved(
	ctx context.Context,
	tenantID string,
	page, pageSize uint32,
) (ports.TenantMemberListPage, error) {
	if repository == nil || repository.database == nil {
		return ports.TenantMemberListPage{}, errors.New("access persistence: tenant member repository unavailable")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" || page == 0 || pageSize == 0 || pageSize > 100 {
		return ports.TenantMemberListPage{}, errors.New("access persistence: invalid removed member list query")
	}
	db := repository.database.WithContext(ctx).Model(&membershipRecord{}).
		Where("tenant_id = ? AND status = ?", tenantID, domain.TenantMemberStatusRemoved)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return ports.TenantMemberListPage{}, err
	}
	var rows []membershipRecord
	offset := int((uint64(page) - 1) * uint64(pageSize))
	if err := db.Order("updated_at DESC, user_id ASC").Limit(int(pageSize)).Offset(offset).Find(&rows).Error; err != nil {
		return ports.TenantMemberListPage{}, err
	}
	members := make([]domain.Membership, 0, len(rows))
	for _, row := range rows {
		member, err := repository.memberFromRecord(ctx, row)
		if err != nil {
			return ports.TenantMemberListPage{}, err
		}
		members = append(members, member)
	}
	return ports.TenantMemberListPage{Members: members, Total: uint64(total)}, nil
}

func (repository *TenantMemberRepository) Remove(
	ctx context.Context,
	member *domain.Membership,
	expectedVersion uint64,
) error {
	if repository == nil || repository.database == nil || member == nil || expectedVersion == 0 {
		return errors.New("access persistence: member removal requires repository, member and version")
	}
	if member.Status != domain.TenantMemberStatusRemoved {
		return domain.ErrInvalidTenantMemberTransition
	}
	removedVersion := expectedVersion + 1
	err := repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked membershipRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).
			First(&locked).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrTenantMemberNotFound
			}
			return err
		}
		if locked.Version != expectedVersion || locked.Status == domain.TenantMemberStatusRemoved {
			return ports.ErrTenantMemberConflict
		}
		if err := tx.Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).
			Delete(&memberRemovedRoleSnapshotRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).
			Delete(&memberRemovedSiteSnapshotRecord{}).Error; err != nil {
			return err
		}
		var roles []memberRoleRecord
		if err := tx.Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).Find(&roles).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		for _, role := range roles {
			if err := tx.Create(&memberRemovedRoleSnapshotRecord{
				TenantID: member.TenantID, UserID: member.UserID, RoleID: role.RoleID,
				RemovedVersion: removedVersion, CapturedAt: now,
			}).Error; err != nil {
				return err
			}
		}
		var sites []memberSiteRecord
		if err := tx.Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).Find(&sites).Error; err != nil {
			return err
		}
		for _, site := range sites {
			if err := tx.Create(&memberRemovedSiteSnapshotRecord{
				TenantID: member.TenantID, UserID: member.UserID, SiteID: site.SiteID,
				RemovedVersion: removedVersion, CapturedAt: now,
			}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).Delete(&memberRoleRecord{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).Delete(&memberSiteRecord{}).Error; err != nil {
			return err
		}
		result := tx.Model(&membershipRecord{}).
			Where("tenant_id = ? AND user_id = ? AND version = ?", member.TenantID, member.UserID, expectedVersion).
			Updates(map[string]any{
				"status": domain.TenantMemberStatusRemoved,
				"version": gorm.Expr("version + 1"),
				"updated_at": member.UpdatedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrTenantMemberConflict
		}
		return revokeWebSessionsForTenantMember(ctx, tx, member.UserID, member.TenantID, "membership_removed")
	})
	if err != nil {
		return err
	}
	member.Version = removedVersion
	return nil
}

func (repository *TenantMemberRepository) Restore(
	ctx context.Context,
	member *domain.Membership,
	expectedVersion uint64,
) ([]string, error) {
	if repository == nil || repository.database == nil || member == nil || expectedVersion == 0 {
		return nil, errors.New("access persistence: member restore requires repository, member and version")
	}
	if member.Status != domain.TenantMemberStatusActive {
		return nil, domain.ErrInvalidTenantMemberTransition
	}
	roleIDs := []string{}
	err := repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked membershipRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ?", member.TenantID, member.UserID).
			First(&locked).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrTenantMemberNotFound
			}
			return err
		}
		if locked.Version != expectedVersion || locked.Status != domain.TenantMemberStatusRemoved {
			return ports.ErrTenantMemberConflict
		}
		if err := tx.Table("biz_member_removed_role_snapshots s").
			Select("s.role_id").
			Joins("JOIN biz_roles r ON r.tenant_id = s.tenant_id AND r.id = s.role_id AND r.status = ?", domain.TenantRoleStatusActive).
			Where("s.tenant_id = ? AND s.user_id = ? AND s.removed_version = ?", member.TenantID, member.UserID, expectedVersion).
			Order("s.role_id ASC").Pluck("s.role_id", &roleIDs).Error; err != nil {
			return err
		}
		result := tx.Model(&membershipRecord{}).
			Where("tenant_id = ? AND user_id = ? AND version = ? AND status = ?", member.TenantID, member.UserID, expectedVersion, domain.TenantMemberStatusRemoved).
			Updates(map[string]any{
				"status": domain.TenantMemberStatusActive,
				"version": gorm.Expr("version + 1"),
				"updated_at": member.UpdatedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrTenantMemberConflict
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	member.Version = expectedVersion + 1
	sort.Strings(roleIDs)
	return roleIDs, nil
}
