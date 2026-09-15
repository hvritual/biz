package domain

import (
	"errors"
	"sort"
	"strings"
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
	"commercial.catalog.read",
	"tenant.entitlement.read",
	"tenant.member.manage",
	"tenant.member.read",
	"tenant.organization.manage",
	"tenant.organization.read",
	"tenant.role.manage",
	"tenant.role.read",
	"tenant.subscription.manage",
}

var (
	ErrInvalidTenantTransition       = errors.New("access: invalid tenant state transition")
	ErrInvalidTenantMemberTransition = errors.New("access: invalid tenant member state transition")
	ErrInvalidTenantMemberProfile    = errors.New("access: invalid tenant member profile")
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

type MemberRoleSummary struct {
	ID     string
	Name   string
	Status string
}

type Membership struct {
	TenantID         string
	UserID           string
	Email            string
	Status           string
	Version          uint64
	Name             string
	Phone            string
	EmployeeID       string
	Position         string
	DepartmentID     string
	Roles            []MemberRoleSummary
	DerivedDataScope DataScope
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewInvitedMembership(tenantID, userID, email string, now time.Time) Membership {
	return Membership{TenantID: tenantID, UserID: userID, Email: email, Status: TenantMemberStatusInvited, Version: 1, DerivedDataScope: DataScopeNone, CreatedAt: now, UpdatedAt: now}
}

func NewActiveMembership(tenantID, userID, email string, now time.Time) Membership {
	return Membership{TenantID: tenantID, UserID: userID, Email: email, Status: TenantMemberStatusActive, Version: 1, DerivedDataScope: DataScopeNone, CreatedAt: now, UpdatedAt: now}
}

func (membership *Membership) UpdateProfile(name, phone, employeeID, position, departmentID string, now time.Time) error {
	if membership == nil || membership.Status == TenantMemberStatusRemoved {
		return ErrInvalidTenantMemberProfile
	}
	name = strings.TrimSpace(name)
	phone = strings.TrimSpace(phone)
	employeeID = strings.TrimSpace(employeeID)
	position = strings.TrimSpace(position)
	departmentID = strings.TrimSpace(departmentID)
	if len(name) > 120 || len(phone) > 40 || len(employeeID) > 80 || len(position) > 120 || len(departmentID) > 64 {
		return ErrInvalidTenantMemberProfile
	}
	membership.Name = name
	membership.Phone = phone
	membership.EmployeeID = employeeID
	membership.Position = position
	membership.DepartmentID = departmentID
	membership.UpdatedAt = now
	return nil
}

func (membership *Membership) Activate(now time.Time) error {
	if membership == nil || membership.Status != TenantMemberStatusInvited {
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
	if membership == nil || (membership.Status != TenantMemberStatusInvited && membership.Status != TenantMemberStatusActive && membership.Status != TenantMemberStatusSuspended) {
		return ErrInvalidTenantMemberTransition
	}
	membership.Status = TenantMemberStatusRemoved
	membership.UpdatedAt = now
	return nil
}

func (membership *Membership) SetRoles(roles []MemberRoleSummary) {
	if membership == nil {
		return
	}
	membership.Roles = append([]MemberRoleSummary(nil), roles...)
	sort.Slice(membership.Roles, func(i, j int) bool {
		if membership.Roles[i].Name == membership.Roles[j].Name {
			return membership.Roles[i].ID < membership.Roles[j].ID
		}
		return membership.Roles[i].Name < membership.Roles[j].Name
	})
}
