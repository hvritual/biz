package application

import (
	"context"
	notificationv1 "github.com/hvritual/biz/contracts/gen/notification/v1"
	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
	"time"
	"yunka.io/framework/requestscope"
)

func (s *ConfigurationService) CreateMessageConfigurations(ctx context.Context, q *notificationv1.CreateMessageConfigurationsRequest) (*notificationv1.MessageConfigurationReceipt, error) {
	p, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if !requestValid(q) || !domain.ValidConfigurationID(q.GroupId, 64) {
		return nil, expose(domain.ErrConfigurationInvalid)
	}
	raw := make([]domain.MessageLevel, 0, len(q.Levels))
	for _, l := range q.Levels {
		raw = append(raw, domain.MessageLevel(l))
	}
	levels, err := domain.CanonicalConfigurationLevels(raw)
	if err != nil {
		return nil, expose(err)
	}
	values, err := (domain.ConfigurationValues{Channels: q.Channels, PrimaryUserID: q.PrimaryUserId, SecondaryUserID: q.SecondaryUserId, AdditionalUserIDs: q.AdditionalUserIds, Notes: q.Notes}).Canonical(s.channels)
	if err != nil {
		return nil, expose(err)
	}
	receipt, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ConfigurationRepositories]) (domain.ConfigurationReceipt, error) {
		r, call := sc.Repositories(), sc.Context()
		if err := r.Authority.Require(call, permissionCreate, true); err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		old, key, digest, err := claim(call, r, p, "create", q)
		if err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		if old != nil {
			if err := validateReplayScope(call, r.CreateGroups, old); err != nil {
				return domain.ConfigurationReceipt{}, err
			}
			return *old, nil
		}
		if _, err := r.CreateGroups.Resolve(call, []string{q.GroupId}, true); err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		if err := r.Recipients.Validate(call, p.TenantID, values.RecipientIDs(), true); err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		now := time.Now().UTC()
		configs := make([]domain.Configuration, 0, len(levels))
		for _, l := range levels {
			configs = append(configs, newConfiguration(p, q.GroupId, l, values, now))
		}
		if err := r.Configurations.CreateMany(call, configs); err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		return finish(call, r, p, "create", key, digest, configs)
	})
	if err != nil {
		return nil, expose(err)
	}
	return receiptDTO(receipt), nil
}
func (s *ConfigurationService) UpdateMessageConfiguration(ctx context.Context, q *notificationv1.UpdateMessageConfigurationRequest) (*notificationv1.MessageConfigurationReceipt, error) {
	p, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if !requestValid(q) || !domain.ValidConfigurationID(q.Id, 64) || q.ExpectedVersion == 0 || q.ExpectedVersion == ^uint64(0) {
		return nil, expose(domain.ErrConfigurationInvalid)
	}
	values, err := (domain.ConfigurationValues{Channels: q.Channels, PrimaryUserID: q.PrimaryUserId, SecondaryUserID: q.SecondaryUserId, AdditionalUserIDs: q.AdditionalUserIds, Notes: q.Notes}).Canonical(s.channels)
	if err != nil {
		return nil, expose(err)
	}
	receipt, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ConfigurationRepositories]) (domain.ConfigurationReceipt, error) {
		r, call := sc.Repositories(), sc.Context()
		if err := r.Authority.Require(call, permissionUpdate, true); err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		old, key, digest, err := claim(call, r, p, "update", q)
		if err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		if old != nil {
			if err := validateReplayScope(call, r.UpdateGroups, old); err != nil {
				return domain.ConfigurationReceipt{}, err
			}
			return *old, nil
		}
		before, err := r.Configurations.Get(call, p.TenantID, q.Id, false)
		if err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		if _, err := r.UpdateGroups.Resolve(call, []string{before.GroupID}, true); err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		if err := r.Recipients.Validate(call, p.TenantID, values.RecipientIDs(), true); err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		current, err := r.Configurations.Get(call, p.TenantID, q.Id, true)
		if err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		if current.Version != q.ExpectedVersion {
			return domain.ConfigurationReceipt{}, domain.ErrConfigurationConflict
		}
		current.Channels = values.Channels
		current.PrimaryUserID = values.PrimaryUserID
		current.SecondaryUserID = values.SecondaryUserID
		current.AdditionalUserIDs = values.AdditionalUserIDs
		current.Notes = values.Notes
		current.Version++
		current.UpdatedAt = time.Now().UTC()
		if err := r.Configurations.Replace(call, current, q.ExpectedVersion); err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		return finish(call, r, p, "update", key, digest, []domain.Configuration{current})
	})
	if err != nil {
		return nil, expose(err)
	}
	return receiptDTO(receipt), nil
}
func (s *ConfigurationService) DeleteMessageConfiguration(ctx context.Context, q *notificationv1.DeleteMessageConfigurationRequest) (*notificationv1.MessageConfigurationReceipt, error) {
	p, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if !requestValid(q) || !domain.ValidConfigurationID(q.Id, 64) || q.ExpectedVersion == 0 || q.ExpectedVersion == ^uint64(0) {
		return nil, expose(domain.ErrConfigurationInvalid)
	}
	receipt, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ConfigurationRepositories]) (domain.ConfigurationReceipt, error) {
		r, call := sc.Repositories(), sc.Context()
		if err := r.Authority.Require(call, permissionDelete, true); err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		old, key, digest, err := claim(call, r, p, "delete", q)
		if err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		if old != nil {
			if err := validateReplayScope(call, r.DeleteGroups, old); err != nil {
				return domain.ConfigurationReceipt{}, err
			}
			return *old, nil
		}
		before, err := r.Configurations.Get(call, p.TenantID, q.Id, false)
		if err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		if _, err := r.DeleteGroups.Resolve(call, []string{before.GroupID}, true); err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		current, err := r.Configurations.Get(call, p.TenantID, q.Id, true)
		if err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		if err := r.Configurations.Delete(call, p.TenantID, q.Id, q.ExpectedVersion); err != nil {
			return domain.ConfigurationReceipt{}, err
		}
		current.Deleted = true
		current.Version = q.ExpectedVersion + 1
		current.UpdatedAt = time.Now().UTC()
		return finish(call, r, p, "delete", key, digest, []domain.Configuration{current})
	})
	if err != nil {
		return nil, expose(err)
	}
	return receiptDTO(receipt), nil
}
