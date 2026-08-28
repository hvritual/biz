package policy

import (
	"testing"

	"yunka.io/pkg/operationplan"
)

func TestPressurePlansPreserveCompositionAndPermissionClosure(t *testing.T) {
	set := operationplan.Set{Operations: []operationplan.Plan{
		OperationPlanGetDevice(),
		LocalTransferPressurePlan(),
		RemoteProvisionPressurePlan(),
	}}
	if err := operationplan.Validate(set); err != nil {
		t.Fatal(err)
	}
	local := LocalTransferPressurePlan()
	if local.Composition.Boundary != "local" || len(local.Composition.RequiresOperations) != 1 || local.Composition.RequiresOperations[0] != "device.get" {
		t.Fatalf("unexpected local composition plan: %#v", local.Composition)
	}
	remote := RemoteProvisionPressurePlan()
	if remote.Composition.Boundary != "remote_saga" {
		t.Fatalf("unexpected remote composition plan: %#v", remote.Composition)
	}
}
