package application

import (
	"os"
	"strings"
	"testing"
)

func TestApplicationContainsNoAuthorizationPolicy(t *testing.T) {
	source, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, forbidden := range []string{
		"device.read", "device.create", "device.update", "device.delete",
		"ResolveGrants", "ResolveDeviceScope", "HasPermissions", "Authorize(",
		"internal/access", "gateway/authz",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Application security boundary leak %q in service.go", forbidden)
		}
	}
}
