package ports

import (
	"context"

	"github.com/hvritual/biz/internal/access/domain"
)

// NotificationManagementScopes reads current Access-owned facts for the
// generated notification action. Production composition must bind this reader
// and the resource directory to the same borrowed root transaction. It is not
// an authorization cache and does not replace the gateway's action check.
type NotificationManagementScopes interface {
	ManagementScope(context.Context, string, string, string, bool) (domain.EffectiveBusinessScope, error)
}
