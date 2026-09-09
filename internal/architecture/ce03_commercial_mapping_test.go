package architecture_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/hvritual/biz/internal/commercial/capabilitymap"
)

func TestCE03ProductionInventoryAndArtifacts(t *testing.T) {
	root := filepath.Join("..", "..")
	document, err := capabilitymap.BuildRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := capabilitymap.SyncArtifacts(root, document, false); err != nil {
		t.Fatal(err)
	}
	entries := map[string]capabilitymap.CompiledOperation{}
	for _, entry := range document.Operations {
		entries[entry.OperationID] = entry
	}
	device := entries["device.create"]
	if device.ModuleCode != "device-operations" || !slices.Equal(device.CapabilityCodes, []string{"device.lifecycle"}) || device.ConsumerType != "tenant" || !device.TenantRequired {
		t.Fatalf("real DeviceOps trace lost: %+v", device)
	}
	transfer := entries["device.transfer"]
	if !slices.Equal(transfer.AlwaysRequiredCapabilityCodes, []string{"device.lifecycle", "device.transfer"}) || len(transfer.Children) != 2 {
		t.Fatalf("real transfer dependencies lost: %+v", transfer)
	}
	private := entries["site.validate_transfer_target"]
	if private.ConsumerType != "internal_child" || private.RPC != "" || !private.TenantRequired {
		t.Fatalf("internal child boundary changed: %+v", private)
	}
	for _, id := range []string{"commercial.module.create", "tenant.activate", "tenant.role.assert_member_deactivation_allowed"} {
		if entry := entries[id]; entry.OperationID == "" || entry.ExemptionReason == "" || len(entry.RequiredCapabilityCodes) != 0 {
			t.Fatalf("explicit non-commercial policy missing for %s: %+v", id, entry)
		}
	}
}
