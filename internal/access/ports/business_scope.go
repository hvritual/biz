package ports

import (
	"context"

	"github.com/hvritual/biz/internal/access/domain"
	"yunka.io/gateway/authz"
)

// BusinessScopeResolver exposes Access-owned policy facts, not a projection of
// CurrentDataPolicy or derived_data_scope. Implementations re-evaluate each call.
type BusinessScopeResolver interface {
	ResolveBusinessScope(context.Context, string, string, authz.PermissionKey) (domain.EffectiveBusinessScope, error)
}
