package domain

import (
	"errors"
	"sort"
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

const (
	TenantRoleStatusActive   = "active"
	TenantRoleStatusDisabled = "disabled"
	TenantOwnerRoleName      = "owner"
)

var OwnerRequiredPermissions = []string{
	"tenant.member.manage",
	"tenant.member.read",
	"tenant.role.manage",
	"tenant.role.read",
}

var (
	ErrInvalidTenantTransition       = errors.New("access: invalid tenant state transition")
	ErrInvalidTenantMemberTransition = errors.New("access: invalid tenant member state transition")
	ErrInvalidTenantRoleTransition   = errors.New("access: invalid tenant role state transition")
	ErrProtectedOwnerRole            = errors.New("access: owner role invariant would be violated")
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
	return Membership{TenantID: tenantID, UserID: userID, Email: email, Status: TenantMemberStatusInvited, Version: 1, CreatedAt: now, UpdatedAt: now}
}

func NewActiveMembership(tenantID, userID, email string, now time.Time) Membership {
	return Membership{TenantID: tenantID, UserID: userID, Email: email, Status: TenantMemberStatusActive, Version: 1, CreatedAt: now, UpdatedAt: now}
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
	ID          string
	TenantID    string
	Name        string
	Status      string
	Permissions []PermissionGrant
	Version     uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewRole(id, tenantID, name string, now time.Time) Role {
	return Role{ID: id, TenantID: tenantID, Name: name, Status: TenantRoleStatusActive, Version: 1, CreatedAt: now, UpdatedAt: now}
}

func NewOwnerRole(id, tenantID string, now time.Time) Role {
	role := NewRole(id, tenantID, TenantOwnerRoleName, now)
	role.Permissions = make([]PermissionGrant, 0, len(OwnerRequiredPermissions))
	for _, permission := range OwnerRequiredPermissions {
		role.Permissions = append(role.Permissions, PermissionGrant{TenantID: tenantID, RoleID: id, Permission: permission, Scope: DataScopeAll})
	}
	return role
}

func (role Role) IsOwner() bool { return role.Name == TenantOwnerRoleName }

func (role *Role) Rename(name string, now time.Time) error {
	if role == nil || role.Status == TenantRoleStatusDisabled || role.IsOwner() || name == TenantOwnerRoleName {
		return ErrInvalidTenantRoleTransition
	}
	role.Name = name
	role.UpdatedAt = now
	return nil
}

func (role *Role) Disable(now time.Time) error {
	if role == nil || role.Status != TenantRoleStatusActive {
		return ErrInvalidTenantRoleTransition
	}
	if role.IsOwner() {
		return ErrProtectedOwnerRole
	}
	role.Status = TenantRoleStatusDisabled
	role.UpdatedAt = now
	return nil
}

func (role *Role) Enable(now time.Time) error {
	if role == nil || role.Status != TenantRoleStatusDisabled {
		return ErrInvalidTenantRoleTransition
	}
	role.Status = TenantRoleStatusActive
	role.UpdatedAt = now
	return nil
}

func (role *Role) ReplacePermissions(grants []PermissionGrant, now time.Time) error {
	if role == nil {
		return ErrInvalidTenantRoleTransition
	}
	if role.IsOwner() && !containsOwnerRequiredPermissions(grants) {
		return ErrProtectedOwnerRole
	}
	role.Permissions = append([]PermissionGrant(nil), grants...)
	sort.Slice(role.Permissions, func(i, j int) bool { return role.Permissions[i].Permission < role.Permissions[j].Permission })
	role.UpdatedAt = now
	return nil
}

func containsOwnerRequiredPermissions(grants []PermissionGrant) bool {
	values := make(map[string]DataScope, len(grants))
	for _, grant := range grants {
		values[grant.Permission] = grant.Scope
	}
	for _, required := range OwnerRequiredPermissions {
		if values[required] != DataScopeAll {
			return false
		}
	}
	return true
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
