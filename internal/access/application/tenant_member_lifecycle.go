package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
)

var (
	ErrInvalidTenantMemberRequest = errors.New("access: invalid tenant member request")
	ErrTenantContextRequired      = errors.New("access: trusted tenant context is required")
)

type TenantMemberLifecycleService struct {
	repositories requestscope.RepositoryFactory[ports.TenantMemberRepositories]
	capabilities TenantMemberLifecycleCapabilities
}

func NewTenantMemberLifecycleService(repositories requestscope.RepositoryFactory[ports.TenantMemberRepositories], capabilities TenantMemberLifecycleCapabilities) (*TenantMemberLifecycleService, error) {
	if repositories == nil {
		return nil, errors.New("access: tenant member repository factory is required")
	}
	if capabilities == nil || capabilities.AccessTenantRolePermission() == nil {
		return nil, errors.New("access: tenant member role capability is required")
	}
	return &TenantMemberLifecycleService{repositories: repositories, capabilities: capabilities}, nil
}

func (service *TenantMemberLifecycleService) InviteTenantMember(ctx context.Context, request *accessv1.InviteTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetEmail()) == "" {
		return nil, ErrInvalidTenantMemberRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	email := strings.TrimSpace(strings.ToLower(request.GetEmail()))
	member, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.Membership, error) {
		return scope.Repositories().Member.Invite(scope.Context(), tenantID, newMemberUserID(), email, time.Now().UTC())
	})
	if err != nil {
		return nil, err
	}
	return tenantMemberDTO(member), nil
}

func (service *TenantMemberLifecycleService) BootstrapTenantOwnerMember(ctx context.Context, request *accessv1.BootstrapTenantOwnerMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetTenantId()) == "" || strings.TrimSpace(request.GetUserId()) == "" || strings.TrimSpace(request.GetEmail()) == "" {
		return nil, ErrInvalidTenantMemberRequest
	}
	member, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.Membership, error) {
		return scope.Repositories().Member.Bootstrap(
			scope.Context(),
			strings.TrimSpace(request.GetTenantId()),
			strings.TrimSpace(request.GetUserId()),
			strings.TrimSpace(strings.ToLower(request.GetEmail())),
			time.Now().UTC(),
		)
	})
	if err != nil {
		return nil, err
	}
	return tenantMemberDTO(member), nil
}

func (service *TenantMemberLifecycleService) GetTenantMember(ctx context.Context, request *accessv1.GetTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" {
		return nil, ErrInvalidTenantMemberRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	member, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.Membership, error) {
		return scope.Repositories().Member.Get(scope.Context(), tenantID, strings.TrimSpace(request.GetUserId()))
	})
	if err != nil {
		return nil, err
	}
	return tenantMemberDTO(member), nil
}

func (service *TenantMemberLifecycleService) ListTenantMembers(ctx context.Context, _ *accessv1.ListTenantMembersRequest) (*accessv1.ListTenantMembersResponse, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	members, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) ([]domain.Membership, error) {
		return scope.Repositories().Member.List(scope.Context(), tenantID)
	})
	if err != nil {
		return nil, err
	}
	response := &accessv1.ListTenantMembersResponse{Members: make([]*accessv1.TenantMemberDTO, 0, len(members))}
	for _, member := range members {
		response.Members = append(response.Members, tenantMemberDTO(member))
	}
	return response, nil
}

func (service *TenantMemberLifecycleService) ActivateTenantMember(ctx context.Context, request *accessv1.ActivateTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantMemberRequest
	}
	return service.mutate(ctx, strings.TrimSpace(request.GetUserId()), request.GetVersion(), nil, func(member *domain.Membership) error {
		return member.Activate(time.Now().UTC())
	})
}

func (service *TenantMemberLifecycleService) SuspendTenantMember(ctx context.Context, request *accessv1.SuspendTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantMemberRequest
	}
	userID := strings.TrimSpace(request.GetUserId())
	return service.mutate(ctx, userID, request.GetVersion(), func(callCtx context.Context) error {
		_, err := service.capabilities.AccessTenantRolePermission().AssertTenantMemberDeactivationAllowed(callCtx, &accessv1.AssertTenantMemberDeactivationAllowedRequest{UserId: userID})
		return err
	}, func(member *domain.Membership) error {
		return member.Suspend(time.Now().UTC())
	})
}

func (service *TenantMemberLifecycleService) RemoveTenantMember(ctx context.Context, request *accessv1.RemoveTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantMemberRequest
	}
	userID := strings.TrimSpace(request.GetUserId())
	return service.mutate(ctx, userID, request.GetVersion(), func(callCtx context.Context) error {
		_, err := service.capabilities.AccessTenantRolePermission().AssertTenantMemberDeactivationAllowed(callCtx, &accessv1.AssertTenantMemberDeactivationAllowedRequest{UserId: userID})
		return err
	}, func(member *domain.Membership) error {
		return member.Remove(time.Now().UTC())
	})
}

func (service *TenantMemberLifecycleService) mutate(ctx context.Context, userID string, expectedVersion uint64, beforeApply func(context.Context) error, apply func(*domain.Membership) error) (*accessv1.TenantMemberDTO, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	member, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.Membership, error) {
		current, err := scope.Repositories().Member.Get(scope.Context(), tenantID, userID)
		if err != nil {
			return domain.Membership{}, err
		}
		if current.Version != expectedVersion {
			return domain.Membership{}, ports.ErrTenantMemberConflict
		}
		if beforeApply != nil {
			if err := beforeApply(scope.Context()); err != nil {
				return domain.Membership{}, err
			}
		}
		if err := apply(&current); err != nil {
			return domain.Membership{}, err
		}
		if err := scope.Repositories().Member.Update(scope.Context(), &current, expectedVersion); err != nil {
			return domain.Membership{}, err
		}
		return current, nil
	})
	if err != nil {
		return nil, err
	}
	return tenantMemberDTO(member), nil
}

func trustedTenantID(ctx context.Context) (string, error) {
	principal, ok := identity.FromContext(ctx)
	if !ok || !principal.Authenticated || strings.TrimSpace(principal.TenantID) == "" {
		return "", ErrTenantContextRequired
	}
	return strings.TrimSpace(principal.TenantID), nil
}

func tenantMemberDTO(member domain.Membership) *accessv1.TenantMemberDTO {
	return &accessv1.TenantMemberDTO{
		UserId: member.UserID,
		Email: member.Email,
		Status: tenantMemberStatusDTO(member.Status),
		Version: member.Version,
	}
}

func tenantMemberStatusDTO(status string) accessv1.TenantMemberStatus {
	switch status {
	case domain.TenantMemberStatusInvited:
		return accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_INVITED
	case domain.TenantMemberStatusActive:
		return accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_ACTIVE
	case domain.TenantMemberStatusSuspended:
		return accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_SUSPENDED
	case domain.TenantMemberStatusRemoved:
		return accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_REMOVED
	default:
		return accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_UNSPECIFIED
	}
}

func newMemberUserID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}
