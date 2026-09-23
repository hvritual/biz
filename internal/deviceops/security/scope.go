package security

import (
	"context"
	"errors"
	"strings"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accessports "github.com/hvritual/biz/internal/access/ports"
	"yunka.io/gateway/authz"
)

var ErrAuthorizedScopeMissing = errors.New("deviceops security: authorized scope missing")

type Scope struct {
	All           bool
	Self          bool
	Sites         bool
	UserID        string
	SiteIDs       []string
	TenantID      string
	PolicyBound   bool
	PolicySiteIDs []string
}

func (scope Scope) AllowsSite(siteID string) bool {
	if scope.PolicyBound {
		found := false
		for _, allowed := range scope.PolicySiteIDs {
			if allowed == siteID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
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

// Guard requires a current Access policy resolver; production cannot silently
// fall back to legacy grants or a session's cached authorization summary.
type Guard struct {
	policies accessports.BusinessScopeResolver
}

func NewGuard(policies accessports.BusinessScopeResolver) (*Guard, error) {
	if policies == nil {
		return nil, errors.New("deviceops security: business scope resolver is required")
	}
	return &Guard{policies: policies}, nil
}

func (guard *Guard) Prepare(ctx context.Context, authorized authz.AuthorizedOperation, input any) (context.Context, error) {
	scope := Scope{UserID: authorized.Principal.UserID, TenantID: authorized.Principal.TenantID}
	resourcePermission := map[authz.OperationID]authz.PermissionKey{
		"device.list": "device.read", "device.get": "device.read", "device.create": "device.create",
		"device.update": "device.update", "device.delete": "device.delete", "device.transfer": "device.update",
		"site.validate_transfer_target": "site.read",
		// Historical internal pressure Operations retain the same resource-specific scope.
		"device.transfer.local": "device.update", "device.provision.remote": "device.create",
	}[authorized.Policy.Operation]
	if resourcePermission == "" {
		return nil, denied(authorized)
	}
	current, err := guard.policies.ResolveBusinessScope(ctx, scope.TenantID, scope.UserID, resourcePermission)
	if errors.Is(err, accessdomain.ErrInvalidTenantDataPolicy) {
		return nil, denied(authorized)
	}
	if err != nil {
		return nil, err
	}
	scope.All, scope.Self, scope.Sites = current.All, current.Self, current.Sites
	scope.SiteIDs = append([]string(nil), current.MemberSiteIDs...)
	scope.PolicyBound = current.PolicyBound
	scope.PolicySiteIDs = append([]string(nil), current.PolicySiteIDs...)
	if !scope.All && !scope.Sites && !scope.Self {
		return nil, denied(authorized)
	}
	if (scope.Sites || scope.Self || scope.PolicyBound) && (strings.TrimSpace(scope.UserID) == "" || strings.TrimSpace(scope.TenantID) == "") {
		return nil, denied(authorized)
	}
	// Resource write scope is resolved before the Application boundary.
	switch request := input.(type) {
	case *deviceopsv1.ValidateTransferTargetRequest:
		if siteID := strings.TrimSpace(request.GetSiteId()); siteID != "" && !scope.AllowsSite(siteID) {
			return nil, denied(authorized)
		}
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
