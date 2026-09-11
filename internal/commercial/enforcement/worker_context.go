package enforcement

import (
	"context"
	"errors"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

type workerContextKey struct{}
type workerContext struct {
	guard           *Guard
	subject, tenant string
}

// WorkerContext is wired exclusively into the compiled process runner. No HTTP
// header, RPC field or generic context string is accepted as process authority.
// It is not authorization: the same Executor still resolves live platform IAM
// grants and the declared ALL permission closure on every invocation.
func (g *Guard) WorkerContext(ctx context.Context) (context.Context, error) {
	if ctx == nil || g == nil {
		return nil, errors.New("commercial: worker context unavailable")
	}
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.Subject == "" || p.TenantID != "" {
		return nil, errors.New("commercial: authenticated platform worker required")
	}
	return context.WithValue(ctx, workerContextKey{}, workerContext{guard: g, subject: p.Subject, tenant: p.TenantID}), nil
}
func hasWorkerPermission(values []authz.PermissionKey, want string) bool {
	for _, p := range values {
		if string(p) == want {
			return true
		}
	}
	return false
}
