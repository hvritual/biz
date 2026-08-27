package domain

import "time"

type DataScope string

const (
	DataScopeNone  DataScope = "none"
	DataScopeSelf  DataScope = "self"
	DataScopeSites DataScope = "sites"
	DataScopeAll   DataScope = "all"
)

type Tenant struct {
	ID        string
	Name      string
	Status    string
	CreatedAt time.Time
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
