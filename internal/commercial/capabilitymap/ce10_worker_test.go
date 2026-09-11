package capabilitymap

import (
	"testing"
)

func TestCE10WorkerDeclarationRejectsTransportAndMissingAuthority(t *testing.T) {
	p, m, d, lookup := fixture(t)
	op := p.Operations[0]
	mapping := d.Operations[0]
	op.Security.TenantRequired = false
	op.Security.Authentication = []string{"api-key"}
	op.Security.Permissions = []string{"platform.provisioning.execute"}
	op.Security.PermissionMode = "all"
	op.Bindings.RPC = ""
	op.Bindings.HTTP = nil
	op.Composition.Requires = nil
	mapping.Classification = PlatformManagement
	mapping.ExemptionReason = "compiled process worker, live IAM remains mandatory"
	mapping.WorkerPermission = "platform.provisioning.execute"
	mapping.Children = nil
	_ = m
	if e := validateMapping(mapping, op, lookup, map[string]string{}); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"rpc", "http", "tenant", "permission", "authentication", "any"} {
		t.Run(kind, func(t *testing.T) {
			x := op
			switch kind {
			case "rpc":
				x.Bindings.RPC = "/test/Worker"
			case "http":
				x.Bindings.HTTP = []HTTPBinding{{Method: "POST", Path: "/worker"}}
			case "tenant":
				x.Security.TenantRequired = true
			case "permission":
				x.Security.Permissions = nil
			case "authentication":
				x.Security.Authentication = nil
			case "any":
				x.Security.PermissionMode = "any"
			}
			if e := validateMapping(mapping, x, lookup, map[string]string{}); e == nil {
				t.Fatal("invalid worker declaration accepted")
			}
		})
	}
}
