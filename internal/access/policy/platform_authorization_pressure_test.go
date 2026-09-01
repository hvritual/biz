package policy

import (
	"context"
	"testing"

	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

// platformGrantChecker is the smallest Biz-side pressure adapter for a
// platform authority store. A platform grant is intentionally not owned by a
// synthetic tenant, so an empty tenant ID is valid input for this checker.
type platformGrantChecker struct {
	calls    int
	tenantID string
}

func (checker *platformGrantChecker) ResolveGrants(_ context.Context, tenantID string, _ []string, permissions []authz.PermissionKey) ([]authz.Grant, error) {
	checker.calls++
	checker.tenantID = tenantID
	result := make([]authz.Grant, 0, len(permissions))
	for _, permission := range permissions {
		if permission == authz.PermissionKey("platform.tenant.create") {
			result = append(result, authz.Grant{
				Permission: permission,
				RoleID:     "platform-admin",
			})
		}
	}
	return result, nil
}

func TestPressurePlatformTenantCreateAllowsTenantlessPlatformPrincipal(t *testing.T) {
	ctx := context.Background()
	const method = "/access.v1.TenantLifecycleApplication/CreateTenant"

	resolver := TenantLifecycleResolver()
	operationPolicy, ok := resolver.ResolvePolicy(ctx, method)
	if !ok {
		t.Fatalf("tenant.create generated policy is missing")
	}
	if operationPolicy.TenantRequired {
		t.Fatalf("tenant.create must remain platform scoped: generated policy unexpectedly requires tenant")
	}
	if len(operationPolicy.Permissions) != 1 || operationPolicy.Permissions[0] != authz.PermissionKey("platform.tenant.create") {
		t.Fatalf("tenant.create permission mismatch: %v", operationPolicy.Permissions)
	}

	checker := &platformGrantChecker{}
	authorizer, err := authz.NewGrantAuthorizer(checker)
	if err != nil {
		t.Fatalf("new grant authorizer: %v", err)
	}
	runtime, err := authz.NewOperationRuntime(resolver, authorizer, nil)
	if err != nil {
		t.Fatalf("new operation runtime: %v", err)
	}

	principal := identity.Principal{
		Subject:       "platform-admin:root",
		Roles:         []string{"platform-admin"},
		AuthMethod:    identity.AuthMethodAPIKey,
		Authenticated: true,
		// TenantID is deliberately empty. Platform authority must not be
		// represented by a fake/synthetic tenant.
	}

	if _, err := runtime.Prepare(identity.WithPrincipal(ctx, principal), method, struct{}{}); err != nil {
		t.Fatalf("tenant.create should authorize a tenantless platform principal with platform.tenant.create grant: %v", err)
	}
	if checker.calls != 1 {
		t.Fatalf("platform grant checker calls = %d, want 1", checker.calls)
	}
	if checker.tenantID != "" {
		t.Fatalf("platform grant resolution received synthetic tenant %q", checker.tenantID)
	}
}
