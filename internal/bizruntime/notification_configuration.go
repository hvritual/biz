package bizruntime

import (
	"context"
	"errors"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
	generatedassembly "github.com/hvritual/biz/internal/assembly"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	notificationapp "github.com/hvritual/biz/internal/notification/application"
	notificationdomain "github.com/hvritual/biz/internal/notification/domain"
	notificationpersistence "github.com/hvritual/biz/internal/notification/infrastructure/persistence"
	notificationports "github.com/hvritual/biz/internal/notification/ports"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
)

// Notification configuration shares the existing root Executor and its GORM
// transaction. No module acquires another database or opens a nested UoW.
func (factory applicationFactories) BuildNotificationMessageConfiguration(generatedassembly.NotificationMessageConfigurationDependencies) (notificationapp.MessageConfigurationApplication, error) {
	types, err := notificationdomain.NewMessageTypeCatalog([]notificationdomain.MessageType{
		{Code: "device.fault", Name: "设备故障", Level: notificationdomain.LevelUrgent},
		{Code: "device.offline", Name: "设备通信离线", Level: notificationdomain.LevelImportant},
		{Code: "service.maintenance_due", Name: "维护到期提醒", Level: notificationdomain.LevelImportant},
		{Code: "device.connection_recovered", Name: "设备通信恢复", Level: notificationdomain.LevelGeneral},
		{Code: "system.announcement", Name: "企业系统通知", Level: notificationdomain.LevelGeneral},
	})
	if err != nil {
		return nil, err
	}
	// Registered/configurable is not a delivery status. Transport adapters and
	// event mappings belong to #185; do not expose fake email/SMS enable switches.
	channels, err := notificationdomain.NewChannelRegistry([]notificationdomain.Channel{
		{Code: "in_app", Name: "站内消息", Availability: notificationdomain.ChannelConfigurable},
		{Code: "sms", Name: "短信", Availability: notificationdomain.ChannelNotConfigurable, UnavailableReason: "短信投递适配器尚未配置"},
		{Code: "email", Name: "邮件", Availability: notificationdomain.ChannelNotConfigurable, UnavailableReason: "邮件投递适配器尚未配置"},
	})
	if err != nil {
		return nil, err
	}
	repositories := requestscope.GORMRepositories(buildNotificationRepositories)
	service, err := notificationapp.NewConfigurationService(repositories, types, channels)
	if err != nil {
		return nil, err
	}
	return checkedNotification{inner: service}, nil
}

func buildNotificationRepositories(_ context.Context, tx *gorm.DB) (notificationports.ConfigurationRepositories, error) {
	r := notificationports.ConfigurationRepositories{}
	config, err := notificationpersistence.NewConfigurationRepository(tx)
	if err != nil {
		return r, err
	}
	r.Configurations = config
	access, err := accesspersistence.NewNotificationRecipientDirectory(tx)
	if err != nil {
		return r, err
	}
	adapter := notificationIdentityDirectory{inner: access}
	r.Recipients = adapter
	r.Authority = adapter
	r.ReadGroups, err = devicepersistence.NewNotificationSiteDirectory(tx, adapter, "tenant.notification.read")
	if err != nil {
		return r, err
	}
	r.CreateGroups, err = devicepersistence.NewNotificationSiteDirectory(tx, adapter, "tenant.notification.create")
	if err != nil {
		return r, err
	}
	r.UpdateGroups, err = devicepersistence.NewNotificationSiteDirectory(tx, adapter, "tenant.notification.update")
	if err != nil {
		return r, err
	}
	r.DeleteGroups, err = devicepersistence.NewNotificationSiteDirectory(tx, adapter, "tenant.notification.delete")
	if err != nil {
		return r, err
	}
	r.Audit, err = accesspersistence.NewAuditRepository(tx)
	return r, err
}

type notificationIdentityDirectory struct {
	inner *accesspersistence.NotificationRecipientDirectory
}

func (d notificationIdentityDirectory) ManagementScope(ctx context.Context, tenant, user, permission string, lock bool) (accessdomain.EffectiveBusinessScope, error) {
	scope, err := d.inner.ManagementScope(ctx, tenant, user, permission, lock)
	if errors.Is(err, accesspersistence.ErrUnauthorized) {
		err = notificationdomain.ErrConfigurationScopeDenied
	}
	return scope, err
}
func (d notificationIdentityDirectory) Require(ctx context.Context, permission string, lock bool) error {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated {
		return notificationdomain.ErrConfigurationScopeDenied
	}
	scope, err := d.ManagementScope(ctx, p.TenantID, p.UserID, permission, lock)
	if err != nil {
		return err
	}
	if !scope.All && !scope.Self && !scope.Sites {
		return notificationdomain.ErrConfigurationScopeDenied
	}
	return nil
}
func (d notificationIdentityDirectory) List(ctx context.Context, tenant, query string, page, size int) ([]accessports.NotificationRecipient, uint64, error) {
	rows, total, err := d.inner.List(ctx, tenant, query, page, size)
	if errors.Is(err, accesspersistence.ErrUnauthorized) {
		err = notificationdomain.ErrConfigurationScopeDenied
	}
	return rows, total, err
}
func (d notificationIdentityDirectory) Validate(ctx context.Context, tenant string, ids []string, lock bool) error {
	err := d.inner.Validate(ctx, tenant, ids, lock)
	if errors.Is(err, accesspersistence.ErrUnauthorized) {
		return notificationdomain.ErrConfigurationRecipientInvalid
	}
	return err
}
func migrateNotificationConfigurations(ctx context.Context, db *gorm.DB) error {
	return notificationpersistence.MigrateConfigurations(ctx, db)
}
