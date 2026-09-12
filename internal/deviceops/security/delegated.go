package security

import (
	"context"
	"errors"
	"strings"
	"time"

	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"yunka.io/gateway/authz"
)

var ErrDelegatedAccessDenied = errors.New("deviceops: delegated access denied")

type DeviceOwnerResolver interface {
	ResolveDeviceOwner(context.Context, string) (string, error)
}

type DeviceDelegationResolver interface {
	ResolveActiveDeviceDelegation(context.Context, string, string, string, string, time.Time) (string, uint64, bool, error)
}

type DelegatedResourceProof struct {
	OwnerTenantID    string
	GranteeTenantID  string
	ResourceID       string
	Permission       string
	DelegationID     string
	DelegationVersion uint64
}

type delegatedResourceProofKey struct{}

func WithDelegatedResourceProof(ctx context.Context, proof DelegatedResourceProof) context.Context {
	return context.WithValue(ctx, delegatedResourceProofKey{}, proof)
}

func DelegatedResourceProofFromContext(ctx context.Context) (DelegatedResourceProof, bool) {
	if ctx == nil {
		return DelegatedResourceProof{}, false
	}
	proof, ok := ctx.Value(delegatedResourceProofKey{}).(DelegatedResourceProof)
	return proof, ok
}

type DelegatedDeviceGuard struct {
	owners      DeviceOwnerResolver
	delegations DeviceDelegationResolver
	now         func() time.Time
}

func NewDelegatedDeviceGuard(owners DeviceOwnerResolver, delegations DeviceDelegationResolver) (*DelegatedDeviceGuard, error) {
	if owners == nil {
		return nil, errors.New("deviceops security: device owner resolver is required")
	}
	if delegations == nil {
		return nil, errors.New("deviceops security: delegation resolver is required")
	}
	return &DelegatedDeviceGuard{owners: owners, delegations: delegations, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (guard *DelegatedDeviceGuard) Prepare(ctx context.Context, authorized authz.AuthorizedOperation, input any) (context.Context, error) {
	if guard == nil || guard.owners == nil || guard.delegations == nil {
		return nil, ErrDelegatedAccessDenied
	}
	granteeTenantID := strings.TrimSpace(authorized.Principal.TenantID)
	if !authorized.Principal.Authenticated || granteeTenantID == "" {
		return nil, ErrDelegatedAccessDenied
	}
	deviceID, err := delegatedDeviceID(input)
	if err != nil {
		return nil, err
	}
	permission, err := delegatedPermission(authorized.Policy)
	if err != nil {
		return nil, err
	}
	ownerTenantID, err := guard.owners.ResolveDeviceOwner(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	ownerTenantID = strings.TrimSpace(ownerTenantID)
	if ownerTenantID == "" || ownerTenantID == granteeTenantID {
		return nil, ErrDelegatedAccessDenied
	}
	delegationID, delegationVersion, found, err := guard.delegations.ResolveActiveDeviceDelegation(
		ctx, ownerTenantID, granteeTenantID, deviceID, permission, guard.now(),
	)
	if err != nil {
		return nil, err
	}
	if !found || strings.TrimSpace(delegationID) == "" || delegationVersion == 0 {
		return nil, ErrDelegatedAccessDenied
	}
	return WithDelegatedResourceProof(ctx, DelegatedResourceProof{
		OwnerTenantID: ownerTenantID, GranteeTenantID: granteeTenantID, ResourceID: deviceID,
		Permission: permission, DelegationID: delegationID, DelegationVersion: delegationVersion,
	}), nil
}

func delegatedDeviceID(input any) (string, error) {
	var value string
	switch request := input.(type) {
	case *deviceopsv1.GetDelegatedDeviceRequest:
		if request != nil {
			value = request.GetId()
		}
	case *deviceopsv1.UpdateDelegatedDeviceRequest:
		if request != nil {
			value = request.GetId()
		}
	default:
		return "", ErrDelegatedAccessDenied
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", ErrDelegatedAccessDenied
	}
	return value, nil
}

func delegatedPermission(policy authz.Policy) (string, error) {
	if len(policy.Permissions) != 1 {
		return "", ErrDelegatedAccessDenied
	}
	permission := strings.TrimSpace(string(policy.Permissions[0]))
	if permission != "device.read" && permission != "device.update" {
		return "", ErrDelegatedAccessDenied
	}
	return permission, nil
}
