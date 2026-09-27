package authorization

import (
	"strings"
	"testing"
	"yunka.io/gateway/authz"
)

// Notification actions must be part of the generated, published Access catalog,
// not a handwritten frontend permission list or a second authorization source.
func TestNotificationConfigurationCatalogHasDistinctCurrentTenantWriteActions(t *testing.T) {
	expected := map[string]string{
		"notification.type.list": "read", "notification.channel.list": "read", "notification.group.list": "read", "notification.recipient.list": "read",
		"notification.configuration.list": "read", "notification.configuration.get": "read",
		"notification.configuration.create": "create", "notification.configuration.update": "update", "notification.configuration.delete": "delete",
	}
	seen := map[string]bool{}
	for _, a := range Catalog() {
		suffix, ok := expected[a.Code]
		if !ok {
			continue
		}
		if seen[a.Code] || !a.TenantRequired || a.Domain != "notification" || a.Application != "message_configuration" || len(a.HTTP) != 1 || a.RPC == "" || !containsString(a.Authentication, "web-session") || a.PermissionMode != "all" || len(a.Permissions) != 1 || string(a.Permissions[0]) != "tenant.notification."+suffix || a.Classification != "tenant_business" {
			t.Fatalf("invalid generated action %+v", a)
		}
		seen[a.Code] = true
	}
	if len(seen) != len(expected) {
		t.Fatalf("missing current Notification actions: got %d want %d", len(seen), len(expected))
	}
	for _, suffix := range []string{"read", "create", "update", "delete"} {
		got := AuthorizedActions([]authz.Grant{{Permission: authz.PermissionKey("tenant.notification." + suffix)}})
		count := 0
		for _, a := range got {
			if strings.HasPrefix(a.Code, "notification.") {
				count++
				if expected[a.Code] != suffix {
					t.Fatalf("%s implicitly granted %s", suffix, a.Code)
				}
			}
		}
		want := 1
		if suffix == "read" {
			want = 6
		}
		if count != want {
			t.Fatalf("%s: actions=%d want=%d", suffix, count, want)
		}
	}
}
