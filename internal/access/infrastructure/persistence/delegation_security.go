package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	"gorm.io/gorm"
)

type DelegatedDeviceGrantResolver struct {
	database *gorm.DB
}

func NewDelegatedDeviceGrantResolver(database *gorm.DB) (*DelegatedDeviceGrantResolver, error) {
	if database == nil {
		return nil, errors.New("access persistence: database is required")
	}
	return &DelegatedDeviceGrantResolver{database: database}, nil
}

func (resolver *DelegatedDeviceGrantResolver) ResolveActiveDeviceDelegation(
	ctx context.Context,
	ownerTenantID string,
	granteeTenantID string,
	deviceID string,
	permission string,
	now time.Time,
) (string, uint64, bool, error) {
	if resolver == nil || resolver.database == nil {
		return "", 0, false, errors.New("access persistence: delegation resolver unavailable")
	}
	ownerTenantID = strings.TrimSpace(ownerTenantID)
	granteeTenantID = strings.TrimSpace(granteeTenantID)
	deviceID = strings.TrimSpace(deviceID)
	permission = strings.TrimSpace(permission)
	if ownerTenantID == "" || granteeTenantID == "" || deviceID == "" || permission == "" {
		return "", 0, false, nil
	}
	now = now.UTC()
	var row tenantDelegationRecord
	if err := resolver.database.WithContext(ctx).
		Where("owner_tenant_id = ? AND grantee_tenant_id = ? AND resource_kind = ? AND resource_id = ? AND status = ? AND (expires_at IS NULL OR expires_at > ?)", ownerTenantID, granteeTenantID, domain.TenantDelegationResourceDevice, deviceID, domain.TenantDelegationStatusActive, now).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", 0, false, nil
		}
		return "", 0, false, err
	}
	delegation, err := delegationRecordDomain(row)
	if err != nil {
		return "", 0, false, err
	}
	// Keep a domain-level expiry check as a fail-closed defense even though the
	// database predicate already excludes temporally invalid historical rows.
	if delegation.Expired(now) || !containsDelegationPermission(delegation.Permissions, permission) {
		return "", 0, false, nil
	}
	return delegation.ID, delegation.Version, true, nil
}

func containsDelegationPermission(values []string, permission string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == permission {
			return true
		}
	}
	return false
}
