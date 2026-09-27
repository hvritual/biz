package application

import (
	"context"
	notificationv1 "github.com/hvritual/biz/contracts/gen/notification/v1"
	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
	"yunka.io/framework/requestscope"
)

func (s *ConfigurationService) ListMessageTypes(ctx context.Context, q *notificationv1.ListMessageTypesRequest) (*notificationv1.ListMessageTypesResponse, error) {
	p, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if !requestValid(q) {
		return nil, expose(domain.ErrConfigurationInvalid)
	}
	page, size, err := pagination(q.Page, q.PageSize)
	if err != nil {
		return nil, expose(err)
	}
	result, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ConfigurationRepositories]) (*notificationv1.ListMessageTypesResponse, error) {
		if err := sc.Repositories().Authority.Require(sc.Context(), permissionRead, false); err != nil {
			return nil, err
		}
		rows, err := s.types.List(domain.MessageTypeQuery{CodeContains: q.Code, NameContains: q.Name, Level: domain.MessageLevel(q.Level), Page: page, PageSize: size})
		if err != nil {
			return nil, err
		}
		out := &notificationv1.ListMessageTypesResponse{TenantId: p.TenantID, Total: uint64(rows.Total), Page: uint32(page), PageSize: uint32(size), Items: []*notificationv1.MessageTypeDTO{}, Groups: []*notificationv1.MessageLevelCountDTO{}}
		for _, row := range rows.Items {
			out.Items = append(out.Items, &notificationv1.MessageTypeDTO{Code: row.Code, Name: row.Name, Level: string(row.Level)})
		}
		for _, group := range rows.Groups {
			out.Groups = append(out.Groups, &notificationv1.MessageLevelCountDTO{Level: string(group.Level), Total: uint64(group.Total)})
		}
		return out, nil
	})
	return result, expose(err)
}
func (s *ConfigurationService) ListMessageChannels(ctx context.Context, q *notificationv1.ListMessageChannelsRequest) (*notificationv1.ListMessageChannelsResponse, error) {
	p, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if !requestValid(q) {
		return nil, expose(domain.ErrConfigurationInvalid)
	}
	out, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ConfigurationRepositories]) (*notificationv1.ListMessageChannelsResponse, error) {
		if err := sc.Repositories().Authority.Require(sc.Context(), permissionRead, false); err != nil {
			return nil, err
		}
		rows, err := s.channels.List()
		if err != nil {
			return nil, err
		}
		out := &notificationv1.ListMessageChannelsResponse{TenantId: p.TenantID, Items: []*notificationv1.MessageChannelDTO{}}
		for _, c := range rows {
			out.Items = append(out.Items, &notificationv1.MessageChannelDTO{Code: c.Code, Name: c.Name, Configurable: c.Availability == domain.ChannelConfigurable, UnavailableReason: c.UnavailableReason})
		}
		return out, nil
	})
	return out, expose(err)
}
func (s *ConfigurationService) ListMessageGroups(ctx context.Context, q *notificationv1.ListMessageDirectoryRequest) (*notificationv1.ListMessageDirectoryResponse, error) {
	return s.directory(ctx, q, true)
}
func (s *ConfigurationService) ListMessageRecipients(ctx context.Context, q *notificationv1.ListMessageDirectoryRequest) (*notificationv1.ListMessageDirectoryResponse, error) {
	return s.directory(ctx, q, false)
}
func (s *ConfigurationService) directory(ctx context.Context, q *notificationv1.ListMessageDirectoryRequest, groups bool) (*notificationv1.ListMessageDirectoryResponse, error) {
	p, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if !requestValid(q) || !queryValid(q.Query) {
		return nil, expose(domain.ErrConfigurationInvalid)
	}
	page, size, err := pagination(q.Page, q.PageSize)
	if err != nil {
		return nil, expose(err)
	}
	out, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ConfigurationRepositories]) (*notificationv1.ListMessageDirectoryResponse, error) {
		r, call := sc.Repositories(), sc.Context()
		if err := r.Authority.Require(call, permissionRead, false); err != nil {
			return nil, err
		}
		out := &notificationv1.ListMessageDirectoryResponse{TenantId: p.TenantID, Page: uint32(page), PageSize: uint32(size), Items: []*notificationv1.MessageDirectoryEntryDTO{}}
		if groups {
			rows, total, err := r.ReadGroups.List(call, q.Query, page, size)
			if err != nil {
				return nil, err
			}
			out.Total = total
			for _, row := range rows {
				out.Items = append(out.Items, &notificationv1.MessageDirectoryEntryDTO{Id: row.ID, Name: row.Name})
			}
		} else {
			rows, total, err := r.Recipients.List(call, p.TenantID, q.Query, page, size)
			if err != nil {
				return nil, err
			}
			out.Total = total
			for _, row := range rows {
				out.Items = append(out.Items, &notificationv1.MessageDirectoryEntryDTO{Id: row.UserID, Name: row.Name})
			}
		}
		return out, nil
	})
	return out, expose(err)
}
func (s *ConfigurationService) ListMessageConfigurations(ctx context.Context, q *notificationv1.ListMessageConfigurationsRequest) (*notificationv1.ListMessageConfigurationsResponse, error) {
	p, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if !requestValid(q) || (q.GroupId != "" && !domain.ValidConfigurationID(q.GroupId, 64)) {
		return nil, expose(domain.ErrConfigurationInvalid)
	}
	page, size, err := pagination(q.Page, q.PageSize)
	if err != nil {
		return nil, expose(err)
	}
	out, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ConfigurationRepositories]) (*notificationv1.ListMessageConfigurationsResponse, error) {
		r, call := sc.Repositories(), sc.Context()
		if err := r.Authority.Require(call, permissionRead, false); err != nil {
			return nil, err
		}
		ids, err := r.ReadGroups.CurrentIDs(call)
		if err != nil {
			return nil, err
		}
		if q.GroupId != "" {
			if _, err := r.ReadGroups.Resolve(call, []string{q.GroupId}, false); err != nil {
				return nil, err
			}
			ids = []string{q.GroupId}
		}
		rows, total, err := r.Configurations.List(call, p.TenantID, domain.ConfigurationFilter{GroupIDs: ids, Level: domain.MessageLevel(q.Level), RecipientID: q.RecipientId, Page: page, PageSize: size})
		if err != nil {
			return nil, err
		}
		out := &notificationv1.ListMessageConfigurationsResponse{TenantId: p.TenantID, Items: []*notificationv1.MessageConfigurationDTO{}, Total: total, Page: uint32(page), PageSize: uint32(size)}
		for _, row := range rows {
			groups, err := r.ReadGroups.Resolve(call, []string{row.GroupID}, false)
			if err != nil {
				return nil, err
			}
			out.Items = append(out.Items, configurationDTO(row, groups[0].Name))
		}
		return out, nil
	})
	return out, expose(err)
}
func (s *ConfigurationService) GetMessageConfiguration(ctx context.Context, q *notificationv1.GetMessageConfigurationRequest) (*notificationv1.MessageConfigurationDTO, error) {
	p, err := actor(ctx)
	if err != nil {
		return nil, expose(err)
	}
	if !requestValid(q) || !domain.ValidConfigurationID(q.Id, 64) {
		return nil, expose(domain.ErrConfigurationInvalid)
	}
	out, err := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ConfigurationRepositories]) (*notificationv1.MessageConfigurationDTO, error) {
		r, call := sc.Repositories(), sc.Context()
		if err := r.Authority.Require(call, permissionRead, false); err != nil {
			return nil, err
		}
		row, err := r.Configurations.Get(call, p.TenantID, q.Id, false)
		if err != nil {
			return nil, err
		}
		groups, err := r.ReadGroups.Resolve(call, []string{row.GroupID}, false)
		if err != nil {
			return nil, err
		}
		return configurationDTO(row, groups[0].Name), nil
	})
	return out, expose(err)
}
