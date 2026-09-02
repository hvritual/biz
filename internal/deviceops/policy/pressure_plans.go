package policy

import "github.com/hvritual/yunka.io/pkg/operationplan"

const (
	OperationLocalTransfer   = "device.transfer.local"
	OperationRemoteProvision = "device.provision.remote"
)

// LocalTransferPressurePlan is intentionally handwritten for the C9 pressure
// suite. OperationDeclaration currently exists only as a protobuf RPC method
// option, so an internal-only orchestration use case cannot be canonical PB
// without also becoming an RPC method. See Framework Pressure FP-C9-001.
//
// This slice is repository-level local composition, not child-Operation
// composition: requestscope.Compose2 supplies both typed repository ports over
// one UoW, so requires_operations must remain empty.
func LocalTransferPressurePlan() operationplan.Plan {
	return operationplan.Plan{
		OperationID:  OperationLocalTransfer,
		Domain:       "deviceops",
		Application:  "pressure",
		UseCase:      "transfer_device_local",
		RequestType:  "deviceops.v1.UpdateDeviceRequest",
		ResponseType: "deviceops.v1.DeviceDTO",
		Execution:    operationplan.Execution{Transaction: "local", Idempotency: "none"},
		Security: operationplan.Security{
			TenantRequired: true,
			Authentication: []string{"api-key"},
			Permissions:    []string{"device.read", "device.update"},
			PermissionMode: "all",
		},
		Composition: operationplan.Composition{Boundary: "local"},
		Bindings:    operationplan.Bindings{RPC: "internal://device.transfer.local"},
	}
}

// RemoteProvisionPressurePlan drives the real Saga/Outbox pressure slice while
// keeping transport out of the test. See FP-C9-001 for why this plan is not PB.
func RemoteProvisionPressurePlan() operationplan.Plan {
	return operationplan.Plan{
		OperationID:  OperationRemoteProvision,
		Domain:       "deviceops",
		Application:  "pressure",
		UseCase:      "provision_device_remote",
		RequestType:  "deviceops.v1.CreateDeviceRequest",
		ResponseType: "deviceops.v1.DeviceDTO",
		Execution:    operationplan.Execution{Transaction: "local", Idempotency: "required"},
		Security: operationplan.Security{
			TenantRequired: true,
			Authentication: []string{"api-key"},
			Permissions:    []string{"device.create"},
			PermissionMode: "all",
		},
		Composition: operationplan.Composition{Boundary: "remote_saga"},
		Bindings:    operationplan.Bindings{RPC: "internal://device.provision.remote"},
	}
}
