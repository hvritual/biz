package ports

import (
	"context"

	"github.com/hvritual/yunka.io/framework/core/identity"
	"github.com/hvritual/yunka.io/gateway/authz"
)

type Authenticator interface {
	Authenticate(context.Context, string) (identity.Principal, error)
}

type GrantResolver interface {
	ResolveGrants(context.Context, string, []string, []authz.PermissionKey) ([]authz.Grant, error)
}

type MemberSiteResolver interface {
	ResolveMemberSites(context.Context, string, string) ([]string, error)
}
