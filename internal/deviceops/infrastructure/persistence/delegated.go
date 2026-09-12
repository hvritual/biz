package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/deviceops/domain"
	"github.com/hvritual/biz/internal/deviceops/ports"
	devicesecurity "github.com/hvritual/biz/internal/deviceops/security"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
	"yunka.io/gateway/authz"
)

var ErrDelegatedResourceProofRequired = errors.New("deviceops persistence: trusted delegated resource proof is required")

type DelegatedDeviceRepository struct {
	database *gorm.DB
}

func NewDelegatedDeviceRepository(database *gorm.DB) (*DelegatedDeviceRepository, error) {
	if database == nil {
		return nil, errors.New("deviceops persistence: database is required")
	}
	return &DelegatedDeviceRepository{database: database}, nil
}

func (repository *DelegatedDeviceRepository) GetAuthorized(ctx context.Context, id string) (domain.Device, error) {
	proof, err := requireDelegatedProof(ctx, id, "device.read", "device.update")
	if err != nil {
		return domain.Device{}, err
	}
	var record DevicePORecord
	if err := repository.database.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", proof.OwnerTenantID, proof.ResourceID).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Device{}, ports.ErrNotFound
		}
		return domain.Device{}, err
	}
	return record.Domain(), nil
}

func (repository *DelegatedDeviceRepository) UpdateAuthorized(ctx context.Context, value *domain.Device, expectedVersion uint64) error {
	if value == nil || strings.TrimSpace(value.ID) == "" || expectedVersion == 0 {
		return errors.New("deviceops persistence: delegated update value/id/version is required")
	}
	proof, err := requireDelegatedProof(ctx, value.ID, "device.update")
	if err != nil {
		return err
	}
	result := repository.database.WithContext(ctx).
		Model(&DevicePORecord{}).
		Where("tenant_id = ? AND id = ? AND version = ?", proof.OwnerTenantID, proof.ResourceID, expectedVersion).
		Updates(map[string]any{
			"name":       value.Name,
			"version":    gorm.Expr("version + 1"),
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ports.ErrConflict
	}
	return nil
}

func NewDelegatedRepositoryFactory(database *gorm.DB) (requestscope.RepositoryFactory[ports.DelegatedRepositories], error) {
	if database == nil {
		return nil, errors.New("deviceops persistence: database is required")
	}
	return requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.DelegatedRepositories, error) {
		repository, err := NewDelegatedDeviceRepository(transaction)
		if err != nil {
			return ports.DelegatedRepositories{}, err
		}
		return ports.DelegatedRepositories{Device: repository}, nil
	}), nil
}

func requireDelegatedProof(ctx context.Context, resourceID string, permissions ...string) (devicesecurity.DelegatedResourceProof, error) {
	proof, ok := devicesecurity.DelegatedResourceProofFromContext(ctx)
	if !ok {
		return devicesecurity.DelegatedResourceProof{}, ErrDelegatedResourceProofRequired
	}
	proof.OwnerTenantID = strings.TrimSpace(proof.OwnerTenantID)
	proof.GranteeTenantID = strings.TrimSpace(proof.GranteeTenantID)
	proof.ResourceID = strings.TrimSpace(proof.ResourceID)
	proof.Permission = strings.TrimSpace(proof.Permission)
	resourceID = strings.TrimSpace(resourceID)
	if proof.OwnerTenantID == "" || proof.GranteeTenantID == "" || proof.ResourceID == "" || proof.ResourceID != resourceID || proof.DelegationVersion == 0 {
		return devicesecurity.DelegatedResourceProof{}, ErrDelegatedResourceProofRequired
	}
	principal, principalOK := identity.FromContext(ctx)
	if !principalOK || !principal.Authenticated || strings.TrimSpace(principal.TenantID) != proof.GranteeTenantID {
		return devicesecurity.DelegatedResourceProof{}, ErrDelegatedResourceProofRequired
	}
	authorized, authorizedOK := authz.AuthorizedOperationFromContext(ctx)
	if !authorizedOK || !authorized.Decision.Allowed || strings.TrimSpace(authorized.Principal.TenantID) != proof.GranteeTenantID {
		return devicesecurity.DelegatedResourceProof{}, ErrDelegatedResourceProofRequired
	}
	permissionAllowed := false
	for _, permission := range permissions {
		if proof.Permission == strings.TrimSpace(permission) {
			permissionAllowed = true
			break
		}
	}
	if !permissionAllowed || len(authorized.Policy.Permissions) != 1 || strings.TrimSpace(string(authorized.Policy.Permissions[0])) != proof.Permission {
		return devicesecurity.DelegatedResourceProof{}, ErrDelegatedResourceProofRequired
	}
	return proof, nil
}
