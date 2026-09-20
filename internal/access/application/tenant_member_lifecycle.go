package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/requestscope"
)

var (
	ErrInvalidTenantMemberRequest = errors.New("access: invalid tenant member request")
	ErrTenantContextRequired      = errors.New("access: trusted tenant context is required")
)

type tenantMemberConflictError struct {
	cause error
}

func (err *tenantMemberConflictError) Error() string {
	return err.cause.Error()
}

func (err *tenantMemberConflictError) Unwrap() error {
	return err.cause
}

func (err *tenantMemberConflictError) GRPCStatus() *status.Status {
	return status.New(codes.Aborted, err.cause.Error())
}

type TenantMemberLifecycleService struct {
	repositories  requestscope.RepositoryFactory[ports.TenantMemberRepositories]
	capabilities  TenantMemberLifecycleCapabilities
	activationTTL time.Duration
	activationURL string
}

func NewTenantMemberLifecycleService(repositories requestscope.RepositoryFactory[ports.TenantMemberRepositories], capabilities TenantMemberLifecycleCapabilities) (*TenantMemberLifecycleService, error) {
	return newTenantMemberLifecycleService(repositories, capabilities, 0, "")
}

func NewTenantMemberLifecycleServiceWithActivation(
	repositories requestscope.RepositoryFactory[ports.TenantMemberRepositories],
	capabilities TenantMemberLifecycleCapabilities,
	activationTTL time.Duration,
	activationURL string,
) (*TenantMemberLifecycleService, error) {
	return newTenantMemberLifecycleService(repositories, capabilities, activationTTL, activationURL)
}

func newTenantMemberLifecycleService(
	repositories requestscope.RepositoryFactory[ports.TenantMemberRepositories],
	capabilities TenantMemberLifecycleCapabilities,
	activationTTL time.Duration,
	activationURL string,
) (*TenantMemberLifecycleService, error) {
	if repositories == nil {
		return nil, errors.New("access: tenant member repository factory is required")
	}
	if capabilities == nil || capabilities.AccessTenantRolePermission() == nil || capabilities.AccessTenantDepartmentManagement() == nil {
		return nil, errors.New("access: tenant member role and department capabilities are required")
	}
	activationURL = strings.TrimSpace(activationURL)
	if activationTTL < 0 {
		return nil, errors.New("access: tenant member activation TTL must not be negative")
	}
	if activationTTL > 0 && activationURL == "" {
		return nil, errors.New("access: tenant member activation URL is required")
	}
	return &TenantMemberLifecycleService{
		repositories:  repositories,
		capabilities:  capabilities,
		activationTTL: activationTTL,
		activationURL: activationURL,
	}, nil
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

func (service *TenantMemberLifecycleService) CreateTenantMember(ctx context.Context, request *accessv1.CreateTenantMemberRequest) (*accessv1.TenantMemberCreationReceipt, error) {
	if request == nil || service.activationTTL <= 0 {
		return nil, ports.ErrTenantMemberActivationUnavailable
	}
	username, err := normalizeTenantMemberUsername(request.GetUsername())
	if err != nil {
		return nil, err
	}
	email := strings.TrimSpace(request.GetEmail())
	phone := strings.TrimSpace(request.GetPhone())
	if email == "" && phone == "" {
		return nil, ErrInvalidTenantMemberRequest
	}
	roleIDs, err := normalizeTenantMemberRoleIDs(request.GetRoleIds())
	if err != nil {
		return nil, err
	}
	mode, err := tenantMemberActivationMode(request.GetActivationMode())
	if err != nil {
		return nil, err
	}
	if mode == "sms_initial_password" && phone == "" {
		return nil, ErrInvalidTenantMemberRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := service.capabilities.AccessTenantDepartmentManagement().AssertTenantMemberDepartmentAssignmentAllowed(ctx, &accessv1.AssertTenantMemberDepartmentAssignmentAllowedRequest{
		DepartmentId: strings.TrimSpace(request.GetDepartmentId()),
	}); err != nil {
		return nil, err
	}

	userID := newMemberUserID()
	created, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (struct {
		Member         domain.Membership
		AccountCreated bool
	}, error) {
		member, isNew, createErr := scope.Repositories().Member.Create(scope.Context(), tenantID, ports.TenantMemberCreateInput{
			UserID:       userID,
			Username:     username,
			Email:        email,
			Phone:        phone,
			Name:         request.GetName(),
			EmployeeID:   request.GetEmployeeId(),
			Position:     request.GetPosition(),
			DepartmentID: request.GetDepartmentId(),
		}, time.Now().UTC())
		return struct {
			Member         domain.Membership
			AccountCreated bool
		}{Member: member, AccountCreated: isNew}, createErr
	})
	if err != nil {
		return nil, wrapTenantMemberConflict(err)
	}

	for _, roleID := range roleIDs {
		if _, err := service.capabilities.AccessTenantRolePermission().AssignTenantRoleMember(ctx, &accessv1.AssignTenantRoleMemberRequest{
			RoleId: roleID,
			UserId: created.Member.UserID,
		}); err != nil {
			return nil, err
		}
	}

	secret := newMemberActivationSecret()
	notificationSecret := service.activationURL + "?token=" + url.QueryEscape(secret)
	if mode == "sms_initial_password" {
		secret = newMemberInitialPassword()
		notificationSecret = "username=" + username + "\npassword=" + secret
	}
	expiresAt := time.Now().UTC().Add(service.activationTTL)
	activation, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (ports.TenantMemberActivationReceipt, error) {
		return scope.Repositories().Activation.Stage(scope.Context(), ports.TenantMemberActivationInput{
			TenantID:           tenantID,
			UserID:             created.Member.UserID,
			Username:           username,
			Email:              email,
			Phone:              phone,
			Mode:               mode,
			Secret:             secret,
			NotificationSecret: notificationSecret,
			NewAccount:         created.AccountCreated,
			ExpiresAt:          expiresAt,
		})
	})
	if err != nil {
		return nil, wrapTenantMemberConflict(err)
	}

	readback, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.Membership, error) {
		return scope.Repositories().Member.Get(scope.Context(), tenantID, created.Member.UserID)
	})
	if err != nil {
		return nil, err
	}
	return &accessv1.TenantMemberCreationReceipt{
		Member:              tenantMemberDTO(readback),
		ActivationMode:      request.GetActivationMode(),
		NotificationEventId: activation.NotificationEventID,
		DeliveryState:       activation.DeliveryState,
		MaskedDestination:   activation.MaskedDestination,
	}, nil
}

func (service *TenantMemberLifecycleService) BootstrapTenantOwnerMember(ctx context.Context, request *accessv1.BootstrapTenantOwnerMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetTenantId()) == "" || strings.TrimSpace(request.GetUserId()) == "" || strings.TrimSpace(request.GetEmail()) == "" {
		return nil, ErrInvalidTenantMemberRequest
	}
	member, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.Membership, error) {
		return scope.Repositories().Member.Bootstrap(scope.Context(), strings.TrimSpace(request.GetTenantId()), strings.TrimSpace(request.GetUserId()), strings.TrimSpace(strings.ToLower(request.GetEmail())), time.Now().UTC())
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

func (service *TenantMemberLifecycleService) ListTenantMembers(ctx context.Context, request *accessv1.ListTenantMembersRequest) (*accessv1.ListTenantMembersResponse, error) {
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	query, err := tenantMemberListQuery(request)
	if err != nil {
		return nil, err
	}
	page, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (ports.TenantMemberListPage, error) {
		return scope.Repositories().Member.List(scope.Context(), tenantID, query)
	})
	if err != nil {
		return nil, err
	}
	response := &accessv1.ListTenantMembersResponse{
		Members: make([]*accessv1.TenantMemberDTO, 0, len(page.Members)),
		Total:   page.Total,
	}
	for _, member := range page.Members {
		response.Members = append(response.Members, tenantMemberDTO(member))
	}
	return response, nil
}

func (service *TenantMemberLifecycleService) ListRemovedTenantMembers(ctx context.Context, request *accessv1.ListRemovedTenantMembersRequest) (*accessv1.ListTenantMembersResponse, error) {
	if request == nil {
		request = &accessv1.ListRemovedTenantMembersRequest{}
	}
	page := request.GetPage()
	if page == 0 {
		page = 1
	}
	pageSize := request.GetPageSize()
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		return nil, ErrInvalidTenantMemberRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	result, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (ports.TenantMemberListPage, error) {
		return scope.Repositories().Member.ListRemoved(scope.Context(), tenantID, page, pageSize)
	})
	if err != nil {
		return nil, err
	}
	response := &accessv1.ListTenantMembersResponse{Members: make([]*accessv1.TenantMemberDTO, 0, len(result.Members)), Total: result.Total}
	for _, member := range result.Members {
		response.Members = append(response.Members, tenantMemberDTO(member))
	}
	return response, nil
}

func tenantMemberListQuery(request *accessv1.ListTenantMembersRequest) (ports.TenantMemberListQuery, error) {
	if request == nil {
		request = &accessv1.ListTenantMembersRequest{}
	}
	query := strings.TrimSpace(request.GetQuery())
	roleID := strings.TrimSpace(request.GetRoleId())
	departmentID := strings.TrimSpace(request.GetDepartmentId())
	if len([]rune(query)) > 320 || len([]rune(roleID)) > 160 || len([]rune(departmentID)) > 64 {
		return ports.TenantMemberListQuery{}, ErrInvalidTenantMemberRequest
	}
	page := request.GetPage()
	if page == 0 {
		page = 1
	}
	pageSize := request.GetPageSize()
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		return ports.TenantMemberListQuery{}, ErrInvalidTenantMemberRequest
	}
	statusFilter := ""
	switch request.GetStatus() {
	case accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_UNSPECIFIED:
	case accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_INVITED:
		statusFilter = domain.TenantMemberStatusInvited
	case accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_ACTIVE:
		statusFilter = domain.TenantMemberStatusActive
	case accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_SUSPENDED:
		statusFilter = domain.TenantMemberStatusSuspended
	case accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_REMOVED:
		statusFilter = domain.TenantMemberStatusRemoved
	default:
		return ports.TenantMemberListQuery{}, ErrInvalidTenantMemberRequest
	}
	return ports.TenantMemberListQuery{
		Query:        query,
		RoleID:       roleID,
		DepartmentID: departmentID,
		Status:       statusFilter,
		Page:         page,
		PageSize:     pageSize,
	}, nil
}

func (service *TenantMemberLifecycleService) UpdateTenantMember(ctx context.Context, request *accessv1.UpdateTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantMemberRequest
	}
	roleIDs, err := normalizeTenantMemberRoleIDs(request.GetRoleIds())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(request.GetEmail()) == "" && strings.TrimSpace(request.GetPhone()) == "" {
		return nil, ErrInvalidTenantMemberRequest
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(request.GetUserId())
	if _, err := service.capabilities.AccessTenantDepartmentManagement().AssertTenantMemberDepartmentAssignmentAllowed(ctx, &accessv1.AssertTenantMemberDepartmentAssignmentAllowedRequest{
		DepartmentId: strings.TrimSpace(request.GetDepartmentId()),
	}); err != nil {
		return nil, err
	}

	current, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.Membership, error) {
		value, getErr := scope.Repositories().Member.Get(scope.Context(), tenantID, userID)
		if getErr != nil {
			return domain.Membership{}, getErr
		}
		if value.Version != request.GetVersion() {
			return domain.Membership{}, ports.ErrTenantMemberConflict
		}
		value.Email = strings.TrimSpace(request.GetEmail())
		if err := value.UpdateProfile(request.GetName(), request.GetPhone(), request.GetEmployeeId(), request.GetPosition(), request.GetDepartmentId(), time.Now().UTC()); err != nil {
			return domain.Membership{}, err
		}
		if err := scope.Repositories().Member.Update(scope.Context(), &value, request.GetVersion()); err != nil {
			return domain.Membership{}, err
		}
		return value, nil
	})
	if err != nil {
		return nil, wrapTenantMemberConflict(err)
	}

	existingRoles := make(map[string]struct{}, len(current.Roles))
	for _, role := range current.Roles {
		existingRoles[role.ID] = struct{}{}
	}
	targetRoles := make(map[string]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		targetRoles[roleID] = struct{}{}
		if _, exists := existingRoles[roleID]; !exists {
			if _, err := service.capabilities.AccessTenantRolePermission().AssignTenantRoleMember(ctx, &accessv1.AssignTenantRoleMemberRequest{RoleId: roleID, UserId: userID}); err != nil {
				return nil, err
			}
		}
	}
	for roleID := range existingRoles {
		if _, keep := targetRoles[roleID]; keep {
			continue
		}
		if _, err := service.capabilities.AccessTenantRolePermission().RevokeTenantRoleMember(ctx, &accessv1.RevokeTenantRoleMemberRequest{RoleId: roleID, UserId: userID}); err != nil {
			return nil, err
		}
	}

	readback, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.Membership, error) {
		return scope.Repositories().Member.Get(scope.Context(), tenantID, userID)
	})
	if err != nil {
		return nil, err
	}
	return tenantMemberDTO(readback), nil
}

func (service *TenantMemberLifecycleService) UpdateTenantMemberProfile(ctx context.Context, request *accessv1.UpdateTenantMemberProfileRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantMemberRequest
	}
	return service.mutate(ctx, strings.TrimSpace(request.GetUserId()), request.GetVersion(), "", false, func(callCtx context.Context, current *domain.Membership) error {
		targetDepartmentID := strings.TrimSpace(request.GetDepartmentId())
		if targetDepartmentID == strings.TrimSpace(current.DepartmentID) {
			return nil
		}
		_, err := service.capabilities.AccessTenantDepartmentManagement().AssertTenantMemberDepartmentAssignmentAllowed(callCtx, &accessv1.AssertTenantMemberDepartmentAssignmentAllowedRequest{DepartmentId: targetDepartmentID})
		return err
	}, func(member *domain.Membership) error {
		return member.UpdateProfile(request.GetName(), request.GetPhone(), request.GetEmployeeId(), request.GetPosition(), request.GetDepartmentId(), time.Now().UTC())
	})
}

func (service *TenantMemberLifecycleService) ActivateTenantMember(ctx context.Context, request *accessv1.ActivateTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantMemberRequest
	}
	reason, err := normalizeMemberLifecycleReason(request.GetReason())
	if err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(request.GetUserId())
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	_, err = requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.Membership, error) {
		current, getErr := scope.Repositories().Member.Get(scope.Context(), tenantID, userID)
		if getErr != nil {
			return domain.Membership{}, getErr
		}
		if current.Status == domain.TenantMemberStatusInvited && scope.Repositories().Activation != nil {
			if guardErr := scope.Repositories().Activation.AssertAdminActivationAllowed(scope.Context(), tenantID, userID); guardErr != nil {
				return domain.Membership{}, guardErr
			}
		}
		return current, nil
	})
	if err != nil {
		return nil, wrapTenantMemberConflict(err)
	}
	return service.mutate(ctx, userID, request.GetVersion(), reason, true, nil, func(member *domain.Membership) error {
		return member.Activate(time.Now().UTC())
	})
}

func (service *TenantMemberLifecycleService) SuspendTenantMember(ctx context.Context, request *accessv1.SuspendTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantMemberRequest
	}
	reason, err := normalizeMemberLifecycleReason(request.GetReason())
	if err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(request.GetUserId())
	return service.mutate(ctx, userID, request.GetVersion(), reason, true, func(callCtx context.Context, member *domain.Membership) error {
		if err := assertTenantMemberDeactivationActor(callCtx, member); err != nil {
			return err
		}
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
	reason, err := normalizeMemberLifecycleReason(request.GetReason())
	if err != nil {
		return nil, err
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(request.GetUserId())
	member, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.Membership, error) {
		current, getErr := scope.Repositories().Member.Get(scope.Context(), tenantID, userID)
		if getErr != nil {
			return domain.Membership{}, getErr
		}
		if current.Version != request.GetVersion() {
			return domain.Membership{}, ports.ErrTenantMemberConflict
		}
		if err := assertTenantMemberDeactivationActor(scope.Context(), &current); err != nil {
			return domain.Membership{}, err
		}
		if _, err := service.capabilities.AccessTenantRolePermission().AssertTenantMemberDeactivationAllowed(scope.Context(), &accessv1.AssertTenantMemberDeactivationAllowedRequest{UserId: userID}); err != nil {
			return domain.Membership{}, err
		}
		if err := current.Remove(time.Now().UTC()); err != nil {
			return domain.Membership{}, err
		}
		if err := scope.Repositories().Member.Remove(scope.Context(), &current, request.GetVersion()); err != nil {
			return domain.Membership{}, err
		}
		if err := notifyTenantMemberLifecycle(scope.Context(), scope.Repositories(), current, reason); err != nil {
			return domain.Membership{}, err
		}
		return current, nil
	})
	if err != nil {
		if errors.Is(err, ports.ErrTenantMemberConflict) {
			return nil, &tenantMemberConflictError{cause: err}
		}
		return nil, err
	}
	return tenantMemberDTO(member), nil
}

func (service *TenantMemberLifecycleService) RestoreTenantMember(ctx context.Context, request *accessv1.RestoreTenantMemberRequest) (*accessv1.TenantMemberDTO, error) {
	if request == nil || strings.TrimSpace(request.GetUserId()) == "" || request.GetVersion() == 0 {
		return nil, ErrInvalidTenantMemberRequest
	}
	reason, err := normalizeMemberLifecycleReason(request.GetReason())
	if err != nil {
		return nil, err
	}
	tenantID, err := trustedTenantID(ctx)
	if err != nil {
		return nil, err
	}
	userID := strings.TrimSpace(request.GetUserId())
	restored, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (struct {
		Member  domain.Membership
		RoleIDs []string
	}, error) {
		current, getErr := scope.Repositories().Member.Get(scope.Context(), tenantID, userID)
		if getErr != nil {
			return struct {
				Member  domain.Membership
				RoleIDs []string
			}{}, getErr
		}
		if current.Version != request.GetVersion() || current.Status != domain.TenantMemberStatusRemoved {
			return struct {
				Member  domain.Membership
				RoleIDs []string
			}{}, ports.ErrTenantMemberConflict
		}
		if err := current.Restore(time.Now().UTC()); err != nil {
			return struct {
				Member  domain.Membership
				RoleIDs []string
			}{}, err
		}
		roleIDs, restoreErr := scope.Repositories().Member.Restore(scope.Context(), &current, request.GetVersion())
		return struct {
			Member  domain.Membership
			RoleIDs []string
		}{Member: current, RoleIDs: roleIDs}, restoreErr
	})
	if err != nil {
		if errors.Is(err, ports.ErrTenantMemberConflict) {
			return nil, &tenantMemberConflictError{cause: err}
		}
		return nil, err
	}
	for _, roleID := range restored.RoleIDs {
		if _, err := service.capabilities.AccessTenantRolePermission().AssignTenantRoleMember(ctx, &accessv1.AssignTenantRoleMemberRequest{
			RoleId: roleID,
			UserId: userID,
		}); err != nil {
			return nil, err
		}
	}
	if _, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.NotificationDeliveryReceipt, error) {
		if scope.Repositories().Lifecycle == nil {
			return domain.NotificationDeliveryReceipt{}, nil
		}
		return scope.Repositories().Lifecycle.Notify(scope.Context(), ports.TenantMemberLifecycleNotificationInput{
			TenantID: tenantID, UserID: userID, Status: domain.TenantMemberStatusActive,
			Reason: reason, Version: restored.Member.Version,
		})
	}); err != nil {
		return nil, err
	}
	readback, err := requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.TenantMemberRepositories]) (domain.Membership, error) {
		return scope.Repositories().Member.Get(scope.Context(), tenantID, userID)
	})
	if err != nil {
		return nil, err
	}
	return tenantMemberDTO(readback), nil
}

func (service *TenantMemberLifecycleService) mutate(
	ctx context.Context,
	userID string,
	expectedVersion uint64,
	reason string,
	lifecycleEvent bool,
	beforeApply func(context.Context, *domain.Membership) error,
	apply func(*domain.Membership) error,
) (*accessv1.TenantMemberDTO, error) {
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
			if err := beforeApply(scope.Context(), &current); err != nil {
				return domain.Membership{}, err
			}
		}
		if err := apply(&current); err != nil {
			return domain.Membership{}, err
		}
		if err := scope.Repositories().Member.Update(scope.Context(), &current, expectedVersion); err != nil {
			return domain.Membership{}, err
		}
		if err := notifyTenantMemberLifecycle(scope.Context(), scope.Repositories(), current, reason); err != nil {
			return domain.Membership{}, err
		}
		return current, nil
	})
	if err != nil {
		if errors.Is(err, ports.ErrTenantMemberConflict) {
			return nil, &tenantMemberConflictError{cause: err}
		}
		return nil, err
	}
	return tenantMemberDTO(member), nil
}

func notifyTenantMemberLifecycle(
	ctx context.Context,
	repositories ports.TenantMemberRepositories,
	member domain.Membership,
	reason string,
) error {
	if repositories.Lifecycle == nil {
		return nil
	}
	_, err := repositories.Lifecycle.Notify(ctx, ports.TenantMemberLifecycleNotificationInput{
		TenantID: member.TenantID, UserID: member.UserID, Status: member.Status,
		Reason: reason, Version: member.Version,
	})
	return err
}

func normalizeMemberLifecycleReason(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 500 {
		return "", ErrInvalidTenantMemberRequest
	}
	return value, nil
}

func assertTenantMemberDeactivationActor(ctx context.Context, member *domain.Membership) error {
	if member == nil {
		return ErrInvalidTenantMemberRequest
	}
	principal, ok := identity.FromContext(ctx)
	if !ok || !principal.Authenticated {
		return ErrTenantContextRequired
	}
	if principal.UserID != "" && principal.UserID == member.UserID {
		return ports.ErrTenantMemberSelfDeactivation
	}
	targetOwner := false
	for _, role := range member.Roles {
		if role.Name == domain.TenantOwnerRoleName && role.Status == domain.TenantRoleStatusActive {
			targetOwner = true
			break
		}
	}
	if !targetOwner {
		return nil
	}
	for _, role := range principal.Roles {
		if role == domain.TenantOwnerRoleName {
			return nil
		}
	}
	return ports.ErrTenantMemberProtectedOwner
}

func trustedTenantID(ctx context.Context) (string, error) {
	principal, ok := identity.FromContext(ctx)
	if !ok || !principal.Authenticated || strings.TrimSpace(principal.TenantID) == "" {
		return "", ErrTenantContextRequired
	}
	return strings.TrimSpace(principal.TenantID), nil
}

func tenantMemberDTO(member domain.Membership) *accessv1.TenantMemberDTO {
	roles := make([]*accessv1.TenantMemberRoleDTO, 0, len(member.Roles))
	for _, role := range member.Roles {
		roles = append(roles, &accessv1.TenantMemberRoleDTO{RoleId: role.ID, RoleName: role.Name, RoleStatus: role.Status})
	}
	return &accessv1.TenantMemberDTO{UserId: member.UserID, Email: member.Email, Status: tenantMemberStatusDTO(member.Status), Version: member.Version, Name: member.Name, Phone: member.Phone, EmployeeId: member.EmployeeID, Position: member.Position, DepartmentId: member.DepartmentID, Roles: roles, DerivedDataScope: string(member.DerivedDataScope), Username: member.Username}
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

func normalizeTenantMemberUsername(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	length := utf8.RuneCountInString(value)
	if length < 5 || length > 20 {
		return "", ErrInvalidTenantMemberRequest
	}
	allDigits := true
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", ErrInvalidTenantMemberRequest
		}
		if !unicode.IsDigit(r) {
			allDigits = false
		}
	}
	if allDigits {
		return "", ErrInvalidTenantMemberRequest
	}
	return value, nil
}

func normalizeTenantMemberRoleIDs(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, ErrInvalidTenantMemberRequest
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, ErrInvalidTenantMemberRequest
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, ErrInvalidTenantMemberRequest
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func tenantMemberActivationMode(mode accessv1.TenantMemberActivationMode) (string, error) {
	switch mode {
	case accessv1.TenantMemberActivationMode_TENANT_MEMBER_ACTIVATION_MODE_ACTIVATION_LINK:
		return "activation_link", nil
	case accessv1.TenantMemberActivationMode_TENANT_MEMBER_ACTIVATION_MODE_SMS_INITIAL_PASSWORD:
		return "sms_initial_password", nil
	default:
		return "", ErrInvalidTenantMemberRequest
	}
}

func wrapTenantMemberConflict(err error) error {
	switch {
	case errors.Is(err, ports.ErrTenantMemberConflict),
		errors.Is(err, ports.ErrTenantMemberExists),
		errors.Is(err, ports.ErrTenantMemberUsernameConflict),
		errors.Is(err, ports.ErrTenantMemberContactConflict),
		errors.Is(err, ports.ErrTenantMemberExistingAccountSMS),
		errors.Is(err, ports.ErrTenantMemberActivationPending):
		return &tenantMemberConflictError{cause: err}
	default:
		return err
	}
}

func newMemberActivationSecret() string {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(value[:])
}

func newMemberInitialPassword() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	var raw [10]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	out := make([]byte, 12)
	out[0], out[1] = 'A', '7'
	for i := 0; i < len(raw); i++ {
		out[i+2] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(out)
}
