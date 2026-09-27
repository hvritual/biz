package application

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	notificationv1 "github.com/hvritual/biz/contracts/gen/notification/v1"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	deviceports "github.com/hvritual/biz/internal/deviceops/ports"
	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
	"yunka.io/framework/requestscope"
)

const (
	permissionRead   = "tenant.notification.read"
	permissionCreate = "tenant.notification.create"
	permissionUpdate = "tenant.notification.update"
	permissionDelete = "tenant.notification.delete"
)

type ConfigurationService struct {
	repositories requestscope.RepositoryFactory[ports.ConfigurationRepositories]
	types        *domain.MessageTypeCatalog
	channels     *domain.ChannelRegistry
}

var _ MessageConfigurationApplication = (*ConfigurationService)(nil)

func NewConfigurationService(repositories requestscope.RepositoryFactory[ports.ConfigurationRepositories], types *domain.MessageTypeCatalog, channels *domain.ChannelRegistry) (*ConfigurationService, error) {
	if repositories == nil {
		return nil, domain.ErrConfigurationUnavailable
	}
	if _, err := types.List(domain.MessageTypeQuery{Page: 1, PageSize: 1}); err != nil {
		return nil, err
	}
	if _, err := channels.List(); err != nil {
		return nil, err
	}
	return &ConfigurationService{repositories: repositories, types: types, channels: channels}, nil
}

func actor(ctx context.Context) (identity.Principal, error) {
	if ctx == nil {
		return identity.Principal{}, domain.ErrConfigurationScopeDenied
	}
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || !domain.ValidConfigurationID(p.TenantID, 64) || !domain.ValidConfigurationID(p.UserID, 64) {
		return identity.Principal{}, domain.ErrConfigurationScopeDenied
	}
	return p, nil
}
func requestValid(request proto.Message) bool {
	return request != nil && request.ProtoReflect().IsValid() && len(request.ProtoReflect().GetUnknown()) == 0
}
func pagination(page, size uint32) (int, int, error) {
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	if page > 1000000 || size > 100 {
		return 0, 0, domain.ErrConfigurationInvalid
	}
	return int(page), int(size), nil
}
func queryValid(value string) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= 200 && strings.IndexFunc(value, unicode.IsControl) < 0
}

func expose(err error) error {
	if err == nil {
		return nil
	}
	code, reason := codes.Unavailable, "NOTIFICATION_UNAVAILABLE"
	switch {
	case errors.Is(err, domain.ErrConfigurationInvalid), errors.Is(err, domain.ErrCatalogQueryInvalid), errors.Is(err, domain.ErrCatalogEntryInvalid), errors.Is(err, domain.ErrChannelSelectionDuplicate), errors.Is(err, deviceports.ErrNotificationSiteQueryInvalid):
		code, reason = codes.InvalidArgument, "NOTIFICATION_INVALID"
	case errors.Is(err, domain.ErrConfigurationScopeDenied), errors.Is(err, deviceports.ErrNotificationSiteScopeDenied):
		code, reason = codes.PermissionDenied, "NOTIFICATION_FORBIDDEN"
	case errors.Is(err, domain.ErrConfigurationRecipientInvalid):
		code, reason = codes.InvalidArgument, "NOTIFICATION_RECIPIENT_INVALID"
	case errors.Is(err, domain.ErrConfigurationNotFound), errors.Is(err, deviceports.ErrNotFound):
		code, reason = codes.NotFound, "NOTIFICATION_NOT_FOUND"
	case errors.Is(err, domain.ErrConfigurationDuplicate):
		code, reason = codes.AlreadyExists, "NOTIFICATION_DUPLICATE"
	case errors.Is(err, domain.ErrConfigurationConflict):
		code, reason = codes.Aborted, "NOTIFICATION_VERSION_CONFLICT"
	case errors.Is(err, domain.ErrConfigurationReplayConflict):
		code, reason = codes.Aborted, "NOTIFICATION_REPLAY_CONFLICT"
	case errors.Is(err, domain.ErrChannelUnknown), errors.Is(err, domain.ErrChannelUnavailable):
		code, reason = codes.FailedPrecondition, "NOTIFICATION_CHANNEL_UNAVAILABLE"
	}
	return status.Error(code, reason)
}

func configurationDTO(c domain.Configuration, groupName string) *notificationv1.MessageConfigurationDTO {
	return &notificationv1.MessageConfigurationDTO{Id: c.ID, TenantId: c.TenantID, GroupId: c.GroupID, GroupName: groupName, Level: string(c.Level), Channels: append([]string{}, c.Channels...), PrimaryUserId: c.PrimaryUserID, SecondaryUserId: c.SecondaryUserID, AdditionalUserIds: append([]string{}, c.AdditionalUserIDs...), Notes: c.Notes, Version: c.Version, CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339Nano), UpdatedAt: c.UpdatedAt.UTC().Format(time.RFC3339Nano), Deleted: c.Deleted}
}
func receiptDTO(receipt domain.ConfigurationReceipt) *notificationv1.MessageConfigurationReceipt {
	out := &notificationv1.MessageConfigurationReceipt{ReceiptId: receipt.ReceiptID, TenantId: receipt.TenantID, Configurations: make([]*notificationv1.MessageConfigurationDTO, 0, len(receipt.Configurations))}
	for _, c := range receipt.Configurations {
		out.Configurations = append(out.Configurations, configurationDTO(c, ""))
	}
	return out
}
func newConfiguration(p identity.Principal, group string, level domain.MessageLevel, values domain.ConfigurationValues, now time.Time) domain.Configuration {
	return domain.Configuration{ID: uuid.NewString(), TenantID: p.TenantID, GroupID: group, Level: level, Channels: append([]string{}, values.Channels...), PrimaryUserID: values.PrimaryUserID, SecondaryUserID: values.SecondaryUserID, AdditionalUserIDs: append([]string{}, values.AdditionalUserIDs...), Notes: values.Notes, Version: 1, CreatedAt: now, UpdatedAt: now}
}

func persistAudit(ctx context.Context, r ports.ConfigurationRepositories, p identity.Principal, operation string, receipt domain.ConfigurationReceipt, digest string) error {
	if r.Audit == nil {
		return domain.ErrConfigurationUnavailable
	}
	// These success records share the configuration's transaction. Existing
	// transport attempt/failure audit remains independent and is not suppressed.
	now := time.Now().UTC()
	id := "nca-" + receipt.ReceiptID
	event := accessdomain.AuditEvent{AuditID: id, EventID: id + "-a", EventType: accessdomain.AuditEventAttempt, TenantID: p.TenantID, ActorSubject: p.Subject, ActorUserID: p.UserID, AuthMethod: p.AuthMethod, OperationID: "notification.configuration." + operation, Module: "notification", Target: "site:" + receipt.Configurations[0].GroupID, ResourceTenantID: p.TenantID, RequestDigest: digest, ReceiptRef: receipt.ReceiptID, Risk: accessdomain.AuditRiskMedium, Outcome: accessdomain.AuditResultPending, OccurredAt: now}
	if operation == "delete" {
		event.Risk = accessdomain.AuditRiskHigh
	}
	if err := r.Audit.AppendAuditEvent(ctx, event); err != nil {
		return err
	}
	event.EventID = id + "-o"
	event.EventType = accessdomain.AuditEventOutcome
	event.Outcome = accessdomain.AuditResultSuccess
	return r.Audit.AppendAuditEvent(ctx, event)
}
func claim(ctx context.Context, r ports.ConfigurationRepositories, p identity.Principal, operation string, input proto.Message) (*domain.ConfigurationReceipt, string, string, error) {
	key := execution.IdempotencyKeyFrom(ctx)
	if !domain.ValidConfigurationID(key, 256) {
		return nil, "", "", domain.ErrConfigurationInvalid
	}
	digest, err := domain.DigestConfigurationRequest(input)
	if err != nil {
		return nil, "", "", err
	}
	old, err := r.Configurations.ClaimReceipt(ctx, p.TenantID, p.UserID, operation, key, digest)
	return old, key, digest, err
}
func finish(ctx context.Context, r ports.ConfigurationRepositories, p identity.Principal, operation, key, digest string, configs []domain.Configuration) (domain.ConfigurationReceipt, error) {
	receipt := domain.ConfigurationReceipt{ReceiptID: uuid.NewString(), TenantID: p.TenantID, Configurations: configs}
	if err := persistAudit(ctx, r, p, operation, receipt, digest); err != nil {
		return domain.ConfigurationReceipt{}, err
	}
	if err := r.Configurations.CompleteReceipt(ctx, p.TenantID, p.UserID, operation, key, receipt); err != nil {
		return domain.ConfigurationReceipt{}, err
	}
	return receipt, nil
}
func validateReplayScope(ctx context.Context, groups deviceports.NotificationSites, receipt *domain.ConfigurationReceipt) error {
	set := map[string]bool{}
	ids := []string{}
	for _, c := range receipt.Configurations {
		if !set[c.GroupID] {
			set[c.GroupID] = true
			ids = append(ids, c.GroupID)
		}
	}
	sort.Strings(ids)
	_, err := groups.Resolve(ctx, ids, true)
	return err
}
