package persistence

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (repository *TenantMemberRepository) GetBusinessScope(ctx context.Context, tenantID, userID string) (domain.MemberBusinessScope, error) {
	if repository == nil || repository.database == nil {
		return domain.MemberBusinessScope{}, errors.New("access persistence: tenant member repository unavailable")
	}
	tenantID, userID = strings.TrimSpace(tenantID), strings.TrimSpace(userID)
	if tenantID == "" || userID == "" {
		return domain.MemberBusinessScope{}, ports.ErrTenantMemberNotFound
	}
	var member membershipRecord
	if err := repository.database.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.MemberBusinessScope{}, ports.ErrTenantMemberNotFound
		}
		return domain.MemberBusinessScope{}, err
	}
	ids, err := repository.memberBusinessScopeSiteIDs(ctx, repository.database, tenantID, userID)
	if err != nil {
		return domain.MemberBusinessScope{}, err
	}
	return domain.MemberBusinessScope{TenantID: tenantID, UserID: userID, SiteIDs: ids, Version: member.Version, UpdatedAt: member.UpdatedAt}, nil
}

func (repository *TenantMemberRepository) ReplaceBusinessScope(
	ctx context.Context,
	tenantID, userID string,
	expectedVersion uint64,
	siteIDs []string,
	now time.Time,
) (domain.MemberBusinessScope, error) {
	if repository == nil || repository.database == nil || expectedVersion == 0 {
		return domain.MemberBusinessScope{}, errors.New("access persistence: member business scope requires repository and version")
	}
	tenantID, userID = strings.TrimSpace(tenantID), strings.TrimSpace(userID)
	if tenantID == "" || userID == "" {
		return domain.MemberBusinessScope{}, ports.ErrTenantMemberNotFound
	}
	normalized, err := normalizeMemberBusinessScopeSiteIDs(siteIDs)
	if err != nil {
		return domain.MemberBusinessScope{}, err
	}
	result := domain.MemberBusinessScope{}
	err = repository.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var member membershipRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND user_id = ?", tenantID, userID).
			First(&member).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrTenantMemberNotFound
			}
			return err
		}
		if member.Status == domain.TenantMemberStatusRemoved {
			return ports.ErrTenantMemberBusinessScopeUnavailable
		}
		if member.Version != expectedVersion {
			return ports.ErrTenantMemberConflict
		}
		current, err := repository.memberBusinessScopeSiteIDs(ctx, tx, tenantID, userID)
		if err != nil {
			return err
		}
		if equalMemberBusinessScopeSiteIDs(current, normalized) {
			result = domain.MemberBusinessScope{TenantID: tenantID, UserID: userID, SiteIDs: current, Version: member.Version, UpdatedAt: member.UpdatedAt}
			return nil
		}
		if err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, userID).Delete(&memberSiteRecord{}).Error; err != nil {
			return err
		}
		for _, siteID := range normalized {
			if err := tx.Create(&memberSiteRecord{TenantID: tenantID, UserID: userID, SiteID: siteID}).Error; err != nil {
				return err
			}
		}
		update := tx.Model(&membershipRecord{}).
			Where("tenant_id = ? AND user_id = ? AND version = ?", tenantID, userID, expectedVersion).
			Updates(map[string]any{"version": gorm.Expr("version + 1"), "updated_at": now.UTC()})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ports.ErrTenantMemberConflict
		}
		result = domain.MemberBusinessScope{TenantID: tenantID, UserID: userID, SiteIDs: normalized, Version: expectedVersion + 1, UpdatedAt: now.UTC()}
		return nil
	})
	if err != nil {
		return domain.MemberBusinessScope{}, err
	}
	return result, nil
}

func (repository *TenantMemberRepository) memberBusinessScopeSiteIDs(ctx context.Context, db *gorm.DB, tenantID, userID string) ([]string, error) {
	var rows []memberSiteRecord
	if err := db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("site_id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.SiteID)
	}
	return ids, nil
}

func normalizeMemberBusinessScopeSiteIDs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 64 {
			return nil, ports.ErrTenantMemberBusinessScopeUnavailable
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func equalMemberBusinessScopeSiteIDs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
