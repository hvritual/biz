package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"google.golang.org/protobuf/proto"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
)

// The original typed payload and trusted owner bind one immutable creation
// result. No caller-controlled tenant or subject is accepted from the body.
func createTenantRoleOnce(ctx context.Context, repository ports.TenantRoleRepository, request *accessv1.CreateTenantRoleRequest, role domain.Role) (domain.Role, error) {
	principal, ok := identity.FromContext(ctx)
	if !ok || !principal.Authenticated || principal.Subject == "" || principal.TenantID != role.TenantID {
		return domain.Role{}, ErrInvalidTenantRoleRequest
	}
	transportKey := execution.IdempotencyKeyFrom(ctx)
	if transportKey == "" {
		return domain.Role{}, execution.ErrIdempotencyKeyRequired
	}
	owner, err := json.Marshal([]string{"tenant.role.create", principal.TenantID, principal.Subject, principal.UserID, transportKey})
	if err != nil {
		return domain.Role{}, err
	}
	payload, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return domain.Role{}, err
	}
	key, fingerprint := sha256.Sum256(owner), sha256.Sum256(payload)
	return repository.CreateOnce(ctx, &role, hex.EncodeToString(key[:]), hex.EncodeToString(fingerprint[:]))
}
