package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestECRI06TenantSubscriptionReadIsSessionScoped(t *testing.T) {
	root := filepath.Join("..", "..")
	protoBytes, err := os.ReadFile(filepath.Join(root, "contracts", "proto", "commercial", "v1", "subscription.proto"))
	if err != nil {
		t.Fatal(err)
	}
	proto := string(protoBytes)
	for _, required := range []string{
		"message GetMySubscriptionRequest {}",
		`get:"/v1/tenant/subscription"`,
		`id:"commercial.subscription.get_my"`,
		`permissions:"tenant.entitlement.read"`,
		"tenant_required:true",
	} {
		if !strings.Contains(proto, required) {
			t.Fatalf("tenant subscription contract missing %q", required)
		}
	}
	if strings.Contains(proto, "message GetMySubscriptionRequest { string tenant_id") {
		t.Fatal("tenant self-service subscription request must not accept tenant_id")
	}

	serviceBytes, err := os.ReadFile(filepath.Join(root, "internal", "commercial", "application", "subscriptionmanagement", "internal", "usecase", "service.go"))
	if err != nil {
		t.Fatal(err)
	}
	service := string(serviceBytes)
	start := strings.Index(service, "func (s *service) GetMySubscription(")
	if start < 0 {
		t.Fatal("tenant subscription service method missing")
	}
	body := service[start:]
	for _, required := range []string{"tenantActor(ctx)", "p.TenantID", "Subscriptions.GetBase"} {
		if !strings.Contains(body, required) {
			t.Fatalf("tenant subscription service does not bind authority to identity: missing %q", required)
		}
	}
	if strings.Contains(body, "r.TenantId") || strings.Contains(body, "r.TenantID") {
		t.Fatal("tenant self-service subscription read must not use a request-supplied tenant id")
	}
}

func TestECRI06TenantOwnerCanReadCommercialEntitlements(t *testing.T) {
	root := filepath.Join("..", "..")
	modelBytes, err := os.ReadFile(filepath.Join(root, "internal", "access", "domain", "model.go"))
	if err != nil {
		t.Fatal(err)
	}
	model := string(modelBytes)
	start := strings.Index(model, "var OwnerRequiredPermissions = []string{")
	if start < 0 {
		t.Fatal("owner permission baseline missing")
	}
	end := strings.Index(model[start:], "}\n")
	if end < 0 {
		t.Fatal("owner permission baseline malformed")
	}
	ownerPermissions := model[start : start+end]
	for _, permission := range []string{"tenant.entitlement.read", "commercial.catalog.read"} {
		if !strings.Contains(ownerPermissions, `"`+permission+`"`) {
			t.Fatalf("tenant owner missing required commercial read permission %q", permission)
		}
	}
}
