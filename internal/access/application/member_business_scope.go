package application

import (
	"context"
	"sort"
	"strings"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/requestscope"
)

func (service *TenantMemberLifecycleService) ListTenantMemberScopeCandidates(
	ctx context.Context,
	request *accessv1.ListTenantMemberScopeCandidatesRequest,
) (*accessv1.ListTenantMemberScopeCandidatesResponse, error) {
	query, page, pageSize, err := tenantMemberScopeCandidateQuery(request)
	if err != nil {
		return nil, err
	}
	response, err := service.capabilities.DeviceopsSiteManagement().ListAssignableMemberSites(ctx, &deviceopsv1.SiteScopeDirectoryRequest{
		Query: query, Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, err
	}
	result := &accessv1.ListTenantMemberScopeCandidatesResponse{Candidates: make([]*accessv1.TenantMemberScopeCandidateDTO, 0), Total: response.GetTotal()}
	for _, site := range response.GetSites() {
		if site == nil || strings.TrimSpace(site.GetId()) == "" {
			continue
		}
		result.Candidates = append(result.Candidates, &accessv1.TenantMemberScopeCandidateDTO{
			Id: site.GetId(), Name: site.GetName(), Version: site.GetVersion(), Assignable: true,
		})
	}
	return result, nil
}

func (service *TenantMemberLifecycleService) GetTenantMemberBusinessScope(
	ctx context.Context,
	request *accessv1.GetTenantMemberBusinessScopeRequest,
) (*accessv1.TenantMemberBusinessScopeDTO, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" {
		return nil, ErrInvalidTenantMemberRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	scope, err := requestscope.JoinValue(ctx, service.repositories, func(view *requestscope.View[ports.TenantMemberRepositories]) (domain.MemberBusinessScope, error) {
		return view.Repositories().Member.GetBusinessScope(view.Context(), tenantID, strings.TrimSpace(request.GetUserId()))
	})
	if err != nil {
		return nil, err
	}
	return tenantMemberBusinessScopeDTO(scope), nil
}

func (service *TenantMemberLifecycleService) SetTenantMemberBusinessScope(
	ctx context.Context,
	request *accessv1.SetTenantMemberBusinessScopeRequest,
) (*accessv1.TenantMemberBusinessScopeDTO, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantMemberRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(request.GetUserId())
	siteIDs, err := normalizeTenantMemberBusinessScopeSiteIDs(request.GetSiteIds())
	if err != nil {
		return nil, err
	}
	current, err := requestscope.JoinValue(ctx, service.repositories, func(view *requestscope.View[ports.TenantMemberRepositories]) (domain.MemberBusinessScope, error) {
		return view.Repositories().Member.GetBusinessScope(view.Context(), tenantID, userID)
	})
	if err != nil {
		return nil, err
	}
	if current.Version != request.GetVersion() {
		return nil, wrapTenantMemberConflict(ports.ErrTenantMemberConflict)
	}
	if len(siteIDs) > 0 {
		if err := service.validateTenantMemberBusinessScopeSites(ctx, siteIDs); err != nil {
			return nil, wrapTenantMemberConflict(err)
		}
	}
	updated, err := requestscope.JoinValue(ctx, service.repositories, func(view *requestscope.View[ports.TenantMemberRepositories]) (domain.MemberBusinessScope, error) {
		return view.Repositories().Member.ReplaceBusinessScope(view.Context(), tenantID, userID, request.GetVersion(), siteIDs, time.Now().UTC())
	})
	if err != nil {
		return nil, wrapTenantMemberConflict(err)
	}
	return tenantMemberBusinessScopeDTO(updated), nil
}

func (service *TenantMemberLifecycleService) validateTenantMemberBusinessScopeSites(ctx context.Context, siteIDs []string) error {
	response, err := service.capabilities.DeviceopsSiteManagement().ListAssignableMemberSites(ctx, &deviceopsv1.SiteScopeDirectoryRequest{
		SiteIds: siteIDs, Resolve: true,
	})
	if err != nil {
		return ports.ErrTenantMemberBusinessScopeUnavailable
	}
	if response == nil || int(response.GetTotal()) != len(siteIDs) || len(response.GetSites()) != len(siteIDs) {
		return ports.ErrTenantMemberBusinessScopeUnavailable
	}
	seen := make(map[string]struct{}, len(response.GetSites()))
	for _, site := range response.GetSites() {
		if site == nil || strings.TrimSpace(site.GetId()) == "" {
			return ports.ErrTenantMemberBusinessScopeUnavailable
		}
		seen[strings.TrimSpace(site.GetId())] = struct{}{}
	}
	for _, siteID := range siteIDs {
		if _, ok := seen[siteID]; !ok {
			return ports.ErrTenantMemberBusinessScopeUnavailable
		}
	}
	return nil
}

func tenantMemberScopeCandidateQuery(request *accessv1.ListTenantMemberScopeCandidatesRequest) (string, uint32, uint32, error) {
	if request == nil {
		request = &accessv1.ListTenantMemberScopeCandidatesRequest{}
	}
	query := strings.TrimSpace(request.GetQuery())
	if len([]rune(query)) > 200 {
		return "", 0, 0, ErrInvalidTenantMemberRequest
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
		return "", 0, 0, ErrInvalidTenantMemberRequest
	}
	return query, page, pageSize, nil
}

func normalizeTenantMemberBusinessScopeSiteIDs(values []string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 64 {
			return nil, ErrInvalidTenantMemberRequest
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func tenantMemberBusinessScopeDTO(scope domain.MemberBusinessScope) *accessv1.TenantMemberBusinessScopeDTO {
	return &accessv1.TenantMemberBusinessScopeDTO{
		UserId: scope.UserID, Version: scope.Version, SiteIds: append([]string(nil), scope.SiteIDs...), TenantId: scope.TenantID,
	}
}
