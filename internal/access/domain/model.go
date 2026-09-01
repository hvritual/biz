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

var ErrInvalidTenantTransition = errors.New("access: invalid tenant state transition")

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
	Status    string
	CreatedAt time.Time
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
