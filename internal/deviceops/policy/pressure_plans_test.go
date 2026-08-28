package policy

import (
	"testing"

	"yunka.io/pkg/operationplan"
)

func TestPressurePlansPreserveCompositionBoundaries(t *testing.T) {
	set := operationplan.Set{Operations: []operationplan.Plan{
		LocalTransferPressurePlan(),
		RemoteProvisionPressurePlan(),
	}}
	if err := operationplan.Validate(set); err != nil {
		t.Fatal(err)
	}
	local := LocalTransferPressurePlan()
	if local.Composition.Boundary != "local" || len(local.Composition.RequiresOperations) != 0 || len(local.Composition.PermissionClosure) != 0 {
		t.Fatalf("repository-level local composition must not invent child operations: %#v", local.Composition)
	}
	remote := RemoteProvisionPressurePlan()
	if remote.Composition.Boundary != "remote_saga" {
		t.Fatalf("unexpected remote composition plan: %#v", remote.Composition)
	}
}
