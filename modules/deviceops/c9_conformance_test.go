package deviceops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC104GeneratedAssemblyOwnsStructuralRuntimeWiring(t *testing.T) {
	read := func(relative string) string {
		t.Helper()
		contents, err := os.ReadFile(filepath.Clean(relative))
		if err != nil {
			t.Fatal(err)
		}
		return string(contents)
	}

	module := read("module.go")
	assembly := read("../../internal/assembly/zz_yunka_assembly_gen.go")
	runtime := read("../../internal/bizruntime/runtime.go")
	main := read("../../cmd/biz/main.go")

	for _, forbidden := range []string{
		"operation.NewExecutorWithOptions",
		"RegisterDeviceManagementOperationExecutor",
		"RegisterDeviceTransferOperationExecutor",
		"NewDeviceopsSiteManagementChildCapability",
		"NewDeviceopsDeviceManagementChildCapability",
		"net.Listen(",
		"grpc.NewServer(",
		"http.Server",
	} {
		if strings.Contains(module, forbidden) {
			t.Fatalf("deviceops module still owns structural runtime plumbing %q", forbidden)
		}
	}

	for _, required := range []string{
		"deviceopsapplication.NewDeviceopsDeviceManagementChildCapability",
		"deviceopsapplication.NewDeviceopsSiteManagementChildCapability",
		"deviceopsrest.RegisterDeviceManagementOperationExecutor",
		"deviceopsrest.RegisterDeviceTransferOperationExecutor",
		"deviceopsrpc.RegisterDeviceManagementOperationExecutor",
		"deviceopsrpc.RegisterDeviceTransferOperationExecutor",
		"type RuntimeBinder func(context.Context, *platform.Provider)",
		"kernel.Bootstrap(ctx",
	} {
		if !strings.Contains(assembly, required) {
			t.Fatalf("generated C10 assembly missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"RegisterSiteManagementOperationExecutor",
		"authz.NewExecutionSecurity",
		"requestscope.NewGORMExecutionFactory",
	} {
		if strings.Contains(assembly, forbidden) {
			t.Fatalf("generated assembly crossed consumer/runtime boundary with %q", forbidden)
		}
	}

	for _, required := range []string{
		"generatedassembly.Bootstrap",
		"BindRuntime:",
		"authz.NewExecutionSecurity",
		"operation.NewExecutorWithOptions",
		"requestscope.NewGORMExecutionFactory",
		"runtimecomponent.HTTP",
		"runtimecomponent.GRPC",
		"provider.ForModule(deviceops.GeneratedDescriptor())",
	} {
		if !strings.Contains(runtime, required) {
			t.Fatalf("Biz-owned runtime binding missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"NewDeviceopsSiteManagementChildCapability",
		"NewDeviceopsDeviceManagementChildCapability",
		"RegisterDeviceManagementOperationExecutor",
		"RegisterDeviceTransferOperationExecutor",
		"modulecatalog.Default()",
	} {
		if strings.Contains(runtime, forbidden) {
			t.Fatalf("Biz runtime duplicates generated structural assembly with %q", forbidden)
		}
	}

	if strings.Contains(main, "kernel.New") || strings.Contains(main, "modulecatalog.Default") {
		t.Fatal("cmd/biz still owns kernel/catalog assembly")
	}
	if !strings.Contains(main, "bizruntime.Bootstrap") {
		t.Fatal("cmd/biz does not enter generated C10 runtime path")
	}

	for _, stale := range []string{
		"../../internal/deviceops/transport/rest/zz_yunka_device_management_rest_adapter_gen.go",
		"../../internal/deviceops/transport/rpc/zz_yunka_device_management_rpc_adapter_gen.go",
		"../../internal/deviceops/transport/rest/zz_yunka_device_transfer_rest_adapter_gen.go",
		"../../internal/deviceops/transport/rpc/zz_yunka_device_transfer_rpc_adapter_gen.go",
		"../../internal/deviceops/transport/rest/zz_yunka_site_management_operation_executor_gen.go",
		"../../internal/deviceops/transport/rpc/zz_yunka_site_management_operation_executor_gen.go",
	} {
		if _, err := os.Stat(filepath.Clean(stale)); err == nil {
			t.Fatalf("stale or forbidden generated transport remains: %s", stale)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}
