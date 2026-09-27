package application

import (
	"context"
	"sort"
	"strings"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/deviceops/domain"
	"github.com/hvritual/biz/internal/deviceops/ports"
	"yunka.io/framework/requestscope"
)

func (service *SiteManagementService) ListAssignableRoleSites(ctx context.Context, request *deviceopsv1.SiteScopeDirectoryRequest) (*deviceopsv1.SiteScopeDirectoryResponse, error) {
	if request == nil || !request.GetResolve() {
		return nil, ErrInvalid
	}
	return service.resolveAssignableSites(ctx, request.GetSiteIds())
}

func (service *SiteManagementService) ListAssignableMemberSites(ctx context.Context, request *deviceopsv1.SiteScopeDirectoryRequest) (*deviceopsv1.SiteScopeDirectoryResponse, error) {
	if request == nil {
		request = &deviceopsv1.SiteScopeDirectoryRequest{}
	}
	if request.GetResolve() {
		return service.resolveAssignableSites(ctx, request.GetSiteIds())
	}
	query := strings.ToLower(strings.TrimSpace(request.GetQuery()))
	if len([]rune(query)) > 200 {
		return nil, ErrInvalid
	}
	page := request.GetPage()
	if page == 0 {
		page = 1
	}
	pageSize := request.GetPageSize()
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		return nil, ErrInvalid
	}
	sites, err := requestscope.JoinValue(ctx, service.repositories, func(view *requestscope.View[ports.ScopedRepositories]) ([]domain.Site, error) {
		return view.Repositories().Site.List(view.Context(), 0, 0)
	})
	if err != nil {
		return nil, err
	}
	filtered := make([]domain.Site, 0, len(sites))
	for _, site := range sites {
		if query != "" && !strings.Contains(strings.ToLower(site.ID), query) && !strings.Contains(strings.ToLower(site.Name), query) {
			continue
		}
		filtered = append(filtered, site)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Name == filtered[j].Name {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].Name < filtered[j].Name
	})
	total := uint64(len(filtered))
	start := int((uint64(page) - 1) * uint64(pageSize))
	if start >= len(filtered) {
		return &deviceopsv1.SiteScopeDirectoryResponse{Sites: []*deviceopsv1.SiteDTO{}, Total: total}, nil
	}
	end := start + int(pageSize)
	if end > len(filtered) {
		end = len(filtered)
	}
	return siteDirectoryResponse(filtered[start:end], total), nil
}

func (service *SiteManagementService) resolveAssignableSites(ctx context.Context, values []string) (*deviceopsv1.SiteScopeDirectoryResponse, error) {
	seen := map[string]struct{}{}
	ids := make([]string, 0, len(values))
	for _, raw := range values {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, ErrInvalid
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, ErrInvalid
	}
	sort.Strings(ids)
	sites, err := requestscope.JoinValue(ctx, service.repositories, func(view *requestscope.View[ports.ScopedRepositories]) ([]domain.Site, error) {
		result := make([]domain.Site, 0, len(ids))
		for _, id := range ids {
			site, err := view.Repositories().Site.Get(view.Context(), id)
			if err != nil {
				return nil, err
			}
			result = append(result, site)
		}
		return result, nil
	})
	if err != nil {
		return nil, err
	}
	return siteDirectoryResponse(sites, uint64(len(sites))), nil
}

func siteDirectoryResponse(sites []domain.Site, total uint64) *deviceopsv1.SiteScopeDirectoryResponse {
	response := &deviceopsv1.SiteScopeDirectoryResponse{Sites: make([]*deviceopsv1.SiteDTO, 0, len(sites)), Total: total}
	for _, site := range sites {
		response.Sites = append(response.Sites, &deviceopsv1.SiteDTO{Id: site.ID, Name: site.Name, Version: site.Version})
	}
	return response
}
