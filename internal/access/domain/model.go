package domain

import (
	"errors"
	"time"
)

type DataScope string

const (
	DataScopeNone  DataScope = "none"
	DataScopeSelf  DataScope = "self"
	DataScopeSites DataScope = "sites"
	DataScopeAll   DataScope = "all"
)

const (
	TenantStatusPending   = "pending"
	TenantStatusActive    = "active"
	TenantStatusSuspended = "suspended"
	TenantStatusClosed    = "closed"
)

const (
	TenantMemberStatusInvited   = "invited"
	TenantMemberStatusActive    = "active"
	TenantMemberStatusSuspended = "suspended"
	TenantMemberStatusRemoved   = "removed"
)

var (
	ErrInvalidTenantTransition       = errors.New("access: invalid tenant state transition")
	ErrInvalidTenantMemberTransition = errors.New("access: invalid tenant member state transition")
)

type Tenant struct {
	ID        string
	Name      string
	Status    string
	Version   uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewTenant(id, name string, now time.Time) Tenant {
	return Tenant{ID: id, Name: name, Status: TenantStatusPending, Version: 1, CreatedAt: now, UpdatedAt: now}
}

func (tenant *Tenant) Rename(name string, now time.Time) error {
	if tenant == nil || tenant.Status == TenantStatusClosed {
		return ErrInvalidTenantTransition
	}
	tenant.Name = name
	tenant.UpdatedAt = now
	return nil
}

func (tenant *Tenant) Activate(now time.Time) error {
	if tenant == nil || (tenant.Status != TenantStatusPending && tenant.Status != TenantStatusSuspended) {
		return ErrInvalidTenantTransition
	}
	tenant.Status = TenantStatusActive
	tenant.UpdatedAt = now
	return nil
}

func (tenant *Tenant) Suspend(now time.Time) error {
	if tenant == nil || tenant.Status != TenantStatusActive {
		return ErrInvalidTenantTransition
	}
	tenant.Status = TenantStatusSuspended
	tenant.UpdatedAt = now
	return nil
}

func (tenant *Tenant) Close(now time.Time) error {
	if tenant == nil || (tenant.Status != TenantStatusActive && tenant.Status != TenantStatusSuspended) {
		return ErrInvalidTenantTransition
	}
	tenant.Status = TenantStatusClosed
	tenant.UpdatedAt = now
	return nil
}

type User struct {
	ID        string
	Email     string
	Status    string
	CreatedAt time.Time
}

type Membership struct {
	TenantID  string
	UserID    string
	Email     string
	Status    string
	Version   uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewInvitedMembership(tenantID, userID, email string, now time.Time) Membership {
	return Membership{
		TenantID: tenantID,
		UserID: userID,
		Email: email,
		Status: TenantMemberStatusInvited,
		Version: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func NewActiveMembership(tenantID, userID, email string, now time.Time) Membership {
	return Membership{
		TenantID: tenantID,
		UserID: userID,
		Email: email,
		Status: TenantMemberStatusActive,
		Version: 1,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (membership *Membership) Activate(now time.Time) error {
	if membership == nil || (membership.Status != TenantMemberStatusInvited && membership.Status != TenantMemberStatusSuspended) {
		return ErrInvalidTenantMemberTransition
	}
	membership.Status = TenantMemberStatusActive
	membership.UpdatedAt = now
	return nil
}

func (membership *Membership) Suspend(now time.Time) error {
	if membership == nil || membership.Status != TenantMemberStatusActive {
		return ErrInvalidTenantMemberTransition
	}
	membership.Status = TenantMemberStatusSuspended
	membership.UpdatedAt = now
	return nil
}

func (membership *Membership) Remove(now time.Time) error {
	if membership == nil || membership.Status == TenantMemberStatusRemoved {
		return ErrInvalidTenantMemberTransition
	}
	membership.Status = TenantMemberStatusRemoved
	membership.UpdatedAt = now
	return nil
}

type Role struct {
	ID       string
	TenantID string
	Name     string
	Status   string
}

type MemberRole struct {
	TenantID string
	UserID   string
	RoleID   string
}

type PermissionGrant struct {
	TenantID   string
	RoleID     string
	Permission string
	Scope      DataScope
}

type MemberSite struct {
	TenantID string
	UserID   string
	SiteID   string
}

type Credential struct {
	TokenHash string
	TenantID  string
	UserID    string
	ExpiresAt *time.Time
	Disabled  bool
	CreatedAt time.Time
}
