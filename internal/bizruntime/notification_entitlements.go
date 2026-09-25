package bizruntime

import (
	"context"
	notificationv1 "github.com/hvritual/biz/contracts/gen/notification/v1"
	"github.com/hvritual/biz/internal/commercial/enforcement"
	notificationapp "github.com/hvritual/biz/internal/notification/application"
)

type checkedNotification struct {
	inner notificationapp.MessageConfigurationApplication
}

var _ notificationapp.MessageConfigurationApplication = checkedNotification{}

func (w checkedNotification) ListMessageTypes(ctx context.Context, q *notificationv1.ListMessageTypesRequest) (*notificationv1.ListMessageTypesResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "notification.type.list"); err != nil {
		return nil, err
	}
	value, err := w.inner.ListMessageTypes(ctx, q)
	return value, rememberNotificationError(ctx, "notification.type.list", enforcement.ExecutionError(ctx, "notification.type.list", err))
}

func (w checkedNotification) ListMessageChannels(ctx context.Context, q *notificationv1.ListMessageChannelsRequest) (*notificationv1.ListMessageChannelsResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "notification.channel.list"); err != nil {
		return nil, err
	}
	value, err := w.inner.ListMessageChannels(ctx, q)
	return value, rememberNotificationError(ctx, "notification.channel.list", enforcement.ExecutionError(ctx, "notification.channel.list", err))
}

func (w checkedNotification) ListMessageGroups(ctx context.Context, q *notificationv1.ListMessageDirectoryRequest) (*notificationv1.ListMessageDirectoryResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "notification.group.list"); err != nil {
		return nil, err
	}
	value, err := w.inner.ListMessageGroups(ctx, q)
	return value, rememberNotificationError(ctx, "notification.group.list", enforcement.ExecutionError(ctx, "notification.group.list", err))
}

func (w checkedNotification) ListMessageRecipients(ctx context.Context, q *notificationv1.ListMessageDirectoryRequest) (*notificationv1.ListMessageDirectoryResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "notification.recipient.list"); err != nil {
		return nil, err
	}
	value, err := w.inner.ListMessageRecipients(ctx, q)
	return value, rememberNotificationError(ctx, "notification.recipient.list", enforcement.ExecutionError(ctx, "notification.recipient.list", err))
}

func (w checkedNotification) ListMessageConfigurations(ctx context.Context, q *notificationv1.ListMessageConfigurationsRequest) (*notificationv1.ListMessageConfigurationsResponse, error) {
	if err := enforcement.RequireExecuted(ctx, "notification.configuration.list"); err != nil {
		return nil, err
	}
	value, err := w.inner.ListMessageConfigurations(ctx, q)
	return value, rememberNotificationError(ctx, "notification.configuration.list", enforcement.ExecutionError(ctx, "notification.configuration.list", err))
}

func (w checkedNotification) GetMessageConfiguration(ctx context.Context, q *notificationv1.GetMessageConfigurationRequest) (*notificationv1.MessageConfigurationDTO, error) {
	if err := enforcement.RequireExecuted(ctx, "notification.configuration.get"); err != nil {
		return nil, err
	}
	value, err := w.inner.GetMessageConfiguration(ctx, q)
	return value, rememberNotificationError(ctx, "notification.configuration.get", enforcement.ExecutionError(ctx, "notification.configuration.get", err))
}

func (w checkedNotification) CreateMessageConfigurations(ctx context.Context, q *notificationv1.CreateMessageConfigurationsRequest) (*notificationv1.MessageConfigurationReceipt, error) {
	if err := enforcement.RequireExecuted(ctx, "notification.configuration.create"); err != nil {
		return nil, err
	}
	value, err := w.inner.CreateMessageConfigurations(ctx, q)
	return value, rememberNotificationError(ctx, "notification.configuration.create", enforcement.ExecutionError(ctx, "notification.configuration.create", err))
}

func (w checkedNotification) UpdateMessageConfiguration(ctx context.Context, q *notificationv1.UpdateMessageConfigurationRequest) (*notificationv1.MessageConfigurationReceipt, error) {
	if err := enforcement.RequireExecuted(ctx, "notification.configuration.update"); err != nil {
		return nil, err
	}
	value, err := w.inner.UpdateMessageConfiguration(ctx, q)
	return value, rememberNotificationError(ctx, "notification.configuration.update", enforcement.ExecutionError(ctx, "notification.configuration.update", err))
}

func (w checkedNotification) DeleteMessageConfiguration(ctx context.Context, q *notificationv1.DeleteMessageConfigurationRequest) (*notificationv1.MessageConfigurationReceipt, error) {
	if err := enforcement.RequireExecuted(ctx, "notification.configuration.delete"); err != nil {
		return nil, err
	}
	value, err := w.inner.DeleteMessageConfiguration(ctx, q)
	return value, rememberNotificationError(ctx, "notification.configuration.delete", enforcement.ExecutionError(ctx, "notification.configuration.delete", err))
}
