package persistence

import (
	"context"
	"errors"
	"strings"

	"github.com/hvritual/biz/internal/access/domain"
)

// CountQuotaMembers returns the authoritative member quota usage for one tenant.
// Invited, active and suspended memberships consume quota; only removed releases it.
func (repository *TenantMemberRepository) CountQuotaMembers(ctx context.Context, tenantID string) (uint64, error) {
	if repository == nil || repository.database == nil {
		return 0, errors.New("access persistence: tenant member repository unavailable")
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return 0, errors.New("access persistence: tenant member quota count requires tenant")
	}
	var count int64
	if err := repository.database.WithContext(ctx).
		Model(&membershipRecord{}).
		Where("tenant_id = ? AND status <> ?", tenantID, domain.TenantMemberStatusRemoved).
		Count(&count).Error; err != nil {
		return 0, err
	}
	if count < 0 {
		return 0, errors.New("access persistence: tenant member quota count is negative")
	}
	return uint64(count), nil
}
