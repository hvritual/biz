package domain

import (
	"errors"
	"time"
)

const (
	TenantDelegationStatusActive   = "active"
	TenantDelegationStatusRevoked  = "revoked"
	TenantDelegationResourceDevice = "device"
)

var ErrInvalidTenantDelegationTransition = errors.New("access: invalid tenant delegation state transition")

type TenantDelegation struct {
	ID              string
	OwnerTenantID   string
	GranteeTenantID string
	ResourceKind    string
	ResourceID      string
	Permissions     []string
	Status          string
	Version         uint64
	ExpiresAt       *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewTenantDelegation(id, ownerTenantID, granteeTenantID, resourceKind, resourceID string, permissions []string, expiresAt *time.Time, now time.Time) TenantDelegation {
	return TenantDelegation{
		ID:              id,
		OwnerTenantID:   ownerTenantID,
		GranteeTenantID: granteeTenantID,
		ResourceKind:    resourceKind,
		ResourceID:      resourceID,
		Permissions:     append([]string(nil), permissions...),
		Status:          TenantDelegationStatusActive,
		Version:         1,
		ExpiresAt:       expiresAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func (delegation *TenantDelegation) Revoke(now time.Time) error {
	if delegation == nil || delegation.Status != TenantDelegationStatusActive {
		return ErrInvalidTenantDelegationTransition
	}
	delegation.Status = TenantDelegationStatusRevoked
	delegation.UpdatedAt = now
	return nil
}

func (delegation TenantDelegation) Expired(now time.Time) bool {
	return delegation.ExpiresAt != nil && !delegation.ExpiresAt.After(now)
}
