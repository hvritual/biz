package deviceops

import "time"

const (
	PermissionDeviceRead   = "device.read"
	PermissionDeviceCreate = "device.create"
	PermissionDeviceUpdate = "device.update"
	PermissionDeviceDelete = "device.delete"
)

type DataScope string

const (
	DataScopeNone  DataScope = "none"
	DataScopeSelf  DataScope = "self"
	DataScopeSites DataScope = "sites"
	DataScopeAll   DataScope = "all"
)

type Tenant struct {
	ID        string    `gorm:"column:id;primaryKey;size:64" json:"id"`
	Name      string    `gorm:"column:name;size:200;not null" json:"name"`
	Status    string    `gorm:"column:status;size:32;not null;index" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

func (Tenant) TableName() string { return "biz_tenants" }

type User struct {
	ID        string    `gorm:"column:id;primaryKey;size:64" json:"id"`
	Email     string    `gorm:"column:email;size:320;not null;uniqueIndex" json:"email"`
	Status    string    `gorm:"column:status;size:32;not null;index" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"createdAt"`
}

func (User) TableName() string { return "biz_users" }

type Membership struct {
	TenantID  string    `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID    string    `gorm:"column:user_id;primaryKey;size:64"`
	Status    string    `gorm:"column:status;size:32;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (Membership) TableName() string { return "biz_memberships" }

type Role struct {
	ID       string `gorm:"column:id;primaryKey;size:160" json:"id"`
	TenantID string `gorm:"column:tenant_id;size:64;not null;index:idx_role_tenant;uniqueIndex:uniq_role_name,priority:1" json:"tenantId"`
	Name     string `gorm:"column:name;size:100;not null;uniqueIndex:uniq_role_name,priority:2" json:"name"`
	Status   string `gorm:"column:status;size:32;not null" json:"status"`
}

func (Role) TableName() string { return "biz_roles" }

type MemberRole struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID   string `gorm:"column:user_id;primaryKey;size:64"`
	RoleID   string `gorm:"column:role_id;primaryKey;size:160"`
}

func (MemberRole) TableName() string { return "biz_member_roles" }

type RolePermission struct {
	TenantID   string    `gorm:"column:tenant_id;primaryKey;size:64"`
	RoleID     string    `gorm:"column:role_id;primaryKey;size:160"`
	Permission string    `gorm:"column:permission;primaryKey;size:120"`
	DataScope  DataScope `gorm:"column:data_scope;size:16;not null"`
}

func (RolePermission) TableName() string { return "biz_role_permissions" }

type MemberSite struct {
	TenantID string `gorm:"column:tenant_id;primaryKey;size:64"`
	UserID   string `gorm:"column:user_id;primaryKey;size:64"`
	SiteID   string `gorm:"column:site_id;primaryKey;size:64"`
}

func (MemberSite) TableName() string { return "biz_member_sites" }

type APIToken struct {
	TokenHash string     `gorm:"column:token_hash;primaryKey;size:64"`
	TenantID  string     `gorm:"column:tenant_id;size:64;not null;index"`
	UserID    string     `gorm:"column:user_id;size:64;not null;index"`
	ExpiresAt *time.Time `gorm:"column:expires_at"`
	Disabled  bool       `gorm:"column:disabled;not null;default:false"`
	CreatedAt time.Time  `gorm:"column:created_at;not null"`
}

func (APIToken) TableName() string { return "biz_api_tokens" }

type Site struct {
	ID       string `gorm:"column:id;primaryKey;size:64" json:"id"`
	TenantID string `gorm:"column:tenant_id;size:64;not null;index" json:"tenantId"`
	Name     string `gorm:"column:name;size:200;not null" json:"name"`
}

func (Site) TableName() string { return "biz_sites" }

type Device struct {
	ID        string    `gorm:"column:id;primaryKey;size:64" json:"id"`
	TenantID  string    `gorm:"column:tenant_id;size:64;not null;index:idx_device_tenant_site,priority:1;uniqueIndex:uniq_device_tenant_serial,priority:1" json:"tenantId"`
	SiteID    string    `gorm:"column:site_id;size:64;not null;index:idx_device_tenant_site,priority:2" json:"siteId"`
	Name      string    `gorm:"column:name;size:200;not null" json:"name"`
	Serial    string    `gorm:"column:serial;size:128;not null;uniqueIndex:uniq_device_tenant_serial,priority:2" json:"serial"`
	CreatedBy string    `gorm:"column:created_by;size:64;not null;index" json:"createdBy"`
	Version   uint64    `gorm:"column:version;not null" json:"version"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (Device) TableName() string { return "biz_devices" }
