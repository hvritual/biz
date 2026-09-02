package security

import (
	"context"
	"errors"
	"sort"
	"strings"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"github.com/hvritual/yunka.io/gateway/authz"
)

var ErrAuthorizedScopeMissing = errors.New("deviceops security: authorized scope missing")

type Scope struct {
	All     bool
	Self    bool
	Sites   bool
	UserID  string
	SiteIDs []string
}

func (scope Scope) AllowsSite(siteID string) bool {
	if scope.All || scope.Self {
		return true
	}
	if !scope.Sites {
		return false
	}
	for _, allowed := range scope.SiteIDs {
		if allowed == siteID {
			return true
		}
	}
	return false
}

type scopeKey struct{}

func WithScope(ctx context.Context, scope Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

func FromContext(ctx context.Context) (Scope, bool) {
	if ctx == nil {
		return Scope{}, false
	}
	scope, ok := ctx.Value(scopeKey{}).(Scope)
	return scope, ok
}

func RequireScope(ctx context.Context) (Scope, error) {
	scope, ok := FromContext(ctx)
	if !ok {
		return Scope{}, ErrAuthorizedScopeMissing
	}
	return scope, nil
}

type Guard struct {
	sites accessports.MemberSiteResolver
}

func NewGuard(sites accessports.MemberSiteResolver) (*Guard, error) {
	if sites == nil {
		return nil, errors.New("deviceops security: member site resolver is required")
	}
	return &Guard{sites: sites}, nil
}

func (guard *Guard) Prepare(ctx context.Context, authorized authz.AuthorizedOperation, input any) (context.Context, error) {
	scope := Scope{UserID: authorized.Principal.UserID}
	for _, grant := range authorized.Decision.Grants {
		switch accessdomain.DataScope(strings.TrimSpace(grant.Scope)) {
		case accessdomain.DataScopeAll:
			scope.All = true
		case accessdomain.DataScopeSites:
			scope.Sites = true
		case accessdomain.DataScopeSelf:
			scope.Self = true
		}
	}
	if !scope.All && !scope.Sites && !scope.Self {
		return nil, denied(authorized)
	}
	if (scope.Sites || scope.Self) && strings.TrimSpace(scope.UserID) == "" {
		return nil, denied(authorized)
	}
	if scope.Sites {
		sites, err := guard.sites.ResolveMemberSites(ctx, authorized.Principal.TenantID, authorized.Principal.UserID)
		if err != nil {
			return nil, err
		}
		scope.SiteIDs = append([]string(nil), sites...)
		sort.Strings(scope.SiteIDs)
	}
	// Resource write scope is resolved before the Application boundary.
	switch request := input.(type) {
	case *deviceopsv1.CreateDeviceRequest:
		if siteID := strings.TrimSpace(request.GetSiteId()); siteID != "" && !scope.AllowsSite(siteID) {
			return nil, denied(authorized)
		}
	case *deviceopsv1.UpdateDeviceRequest:
		if siteID := strings.TrimSpace(request.GetSiteId()); siteID != "" && !scope.AllowsSite(siteID) {
			return nil, denied(authorized)
		}
	case *deviceopsv1.TransferDeviceRequest:
		if siteID := strings.TrimSpace(request.GetTargetSiteId()); siteID != "" && !scope.AllowsSite(siteID) {
			return nil, denied(authorized)
		}
	}
	return WithScope(ctx, scope), nil
}

func denied(authorized authz.AuthorizedOperation) error {
	decision := authorized.Decision
	decision.Allowed = false
	decision.Reason = authz.ReasonPermissionDenied
	return authz.Denied(decision)
}
