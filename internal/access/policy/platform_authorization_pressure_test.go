package policy

import (
	"context"
	"testing"

	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

// platformGrantResolver is the smallest Biz-side pressure adapter for a
// non-tenant authority store. It receives the trusted Principal and the
// explicit TenantBound fact instead of treating an empty tenant ID as a magic
// platform/global authority value.
type platformGrantResolver struct {
	calls       int
	tenantBound bool
	tenantID    string
	operation   authz.OperationID
}

func (resolver *platformGrantResolver) ResolveGrants(_ context.Context, request authz.GrantRequest) ([]authz.Grant, error) {
	resolver.calls++
	resolver.tenantBound = request.TenantBound
	resolver.tenantID = request.Principal.TenantID
	resolver.operation = request.Operation
	result := make([]authz.Grant, 0, len(request.Permissions))
	allowed := map[authz.PermissionKey]bool{
		authz.PermissionKey("commercial.catalog.read"): true,
		authz.PermissionKey("platform.plan.read"):      true,
		authz.PermissionKey("platform.tenant.create"):  true,
	}
	for _, permission := range request.Permissions {
		if allowed[permission] {
			result = append(result, authz.Grant{Permission: permission, RoleID: "platform-admin"})
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
		t.Fatalf("tenant.create must remain non-tenant-bound: generated policy unexpectedly requires tenant")
	}
	expectedPermissions := []authz.PermissionKey{"commercial.catalog.read", "platform.plan.read", "platform.tenant.create"}
	if len(operationPolicy.Permissions) != len(expectedPermissions) {
		t.Fatalf("tenant.create permission mismatch: %v", operationPolicy.Permissions)
	}
	for index, permission := range expectedPermissions {
		if operationPolicy.Permissions[index] != permission {
			t.Fatalf("tenant.create permission mismatch: %v", operationPolicy.Permissions)
		}
	}

	grants := &platformGrantResolver{}
	authorizer, err := authz.NewGrantAuthorizerWithResolver(grants)
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
		// TenantID is deliberately empty. Non-tenant authority must not be
		// represented by a fake/synthetic tenant.
	}

	if _, err := runtime.Prepare(identity.WithPrincipal(ctx, principal), method, struct{}{}); err != nil {
		t.Fatalf("tenant.create should authorize a tenantless principal with platform.tenant.create grant: %v", err)
	}
	if grants.calls != 1 {
		t.Fatalf("principal-aware grant resolver calls = %d, want 1", grants.calls)
	}
	if grants.tenantBound {
		t.Fatal("tenant.create unexpectedly resolved through tenant-bound authority")
	}
	if grants.tenantID != "" {
		t.Fatalf("non-tenant grant resolution received synthetic tenant %q", grants.tenantID)
	}
	if grants.operation != authz.OperationID("tenant.create") {
		t.Fatalf("grant resolution operation = %q, want tenant.create", grants.operation)
	}
}
