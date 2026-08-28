package deviceops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC9ModuleUsesOneUnifiedExecutorPath(t *testing.T) {
	source, err := os.ReadFile("module.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{"authz.NewExecutionSecurity", "operation.NewExecutor", "devicerest.RegisterOperationExecutor", "devicerpc.RegisterOperationExecutor"} {
		if !strings.Contains(text, required) {
			t.Fatalf("C9 executor composition missing %q", required)
		}
	}
	for _, forbidden := range []string{"authz.NewOperationRuntime", "SecuredUnaryServerInterceptor", "AuthorizedUnaryServerInterceptor"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("legacy C8.5 execution path returned: %q", forbidden)
		}
	}
	for _, stale := range []string{
		"../../internal/deviceops/transport/rest/zz_yunka_device_management_rest_adapter_gen.go",
		"../../internal/deviceops/transport/rpc/zz_yunka_device_management_rpc_adapter_gen.go",
	} {
		if _, err := os.Stat(filepath.Clean(stale)); err == nil {
			t.Fatalf("legacy generated transport remains: %s", stale)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}
