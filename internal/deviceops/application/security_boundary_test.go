package application

import (
	"os"
	"strings"
	"testing"
)

func TestApplicationContainsNoAuthorizationPolicy(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		source, err := os.ReadFile(entry.Name())
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
				t.Fatalf("Application security boundary leak %q in %s", forbidden, entry.Name())
			}
		}
	}
}
