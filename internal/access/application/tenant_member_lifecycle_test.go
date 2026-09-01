package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
	"yunka.io/framework/requestscope"
)

type memoryTenantMemberRepository struct {
	mu     sync.Mutex
	values map[string]domain.Membership
}

func newMemoryTenantMemberRepository() *memoryTenantMemberRepository {
	return &memoryTenantMemberRepository{values: map[string]domain.Membership{}}
}

func memberKey(tenantID, userID string) string { return tenantID + "/" + userID }

func (repository *memoryTenantMemberRepository) Invite(_ context.Context, tenantID, userID, email string, now time.Time) (domain.Membership, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	member := domain.NewInvitedMembership(tenantID, userID, email, now)
	key := memberKey(tenantID, userID)
	if _, exists := repository.values[key]; exists {
		return domain.Membership{}, ports.ErrTenantMemberExists
	}
	repository.values[key] = member
	return member, nil
}

func (repository *memoryTenantMemberRepository) Bootstrap(_ context.Context, tenantID, userID, email string, now time.Time) (domain.Membership, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	member := domain.NewActiveMembership(tenantID, userID, email, now)
	key := memberKey(tenantID, userID)
	if existing, exists := repository.values[key]; exists {
		return existing, nil
	}
	repository.values[key] = member
	return member, nil
}

func (repository *memoryTenantMemberRepository) Get(_ context.Context, tenantID, userID string) (domain.Membership, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	member, ok := repository.values[memberKey(tenantID, userID)]
	if !ok {
		return domain.Membership{}, ports.ErrTenantMemberNotFound
	}
	return member, nil
}

func (repository *memoryTenantMemberRepository) List(_ context.Context, tenantID string) ([]domain.Membership, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	result := make([]domain.Membership, 0)
	for _, member := range repository.values {
		if member.TenantID == tenantID {
			result = append(result, member)
		}
	}
	return result, nil
}

func (repository *memoryTenantMemberRepository) Update(_ context.Context, member *domain.Membership, expectedVersion uint64) error {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	key := memberKey(member.TenantID, member.UserID)
	current, ok := repository.values[key]
	if !ok {
		return ports.ErrTenantMemberNotFound
	}
	if current.Version != expectedVersion {
		return ports.ErrTenantMemberConflict
	}
	member.Version = expectedVersion + 1
	repository.values[key] = *member
	return nil
}

func TestTenantMemberLifecycleRequiresTrustedTenantContext(t *testing.T) {
	repository := newMemoryTenantMemberRepository()
	unit := &tenantTestUnit{}
	factory := requestscope.RepositoryFactory[ports.TenantMemberRepositories](func(_ context.Context, got requestscope.UnitOfWork) (ports.TenantMemberRepositories, error) {
		if got != unit {
			t.Fatalf("joined unit=%T %p want=%p", got, got, unit)
		}
		return ports.TenantMemberRepositories{Member: repository}, nil
	})
	service, err := NewTenantMemberLifecycleService(factory)
	if err != nil {
		t.Fatal(err)
	}
	ctx, _, err := execution.BeginRoot(context.Background(), "tenant.member.invite", execution.TransactionLocal, nil, tenantTestTransactionFactory{unit: unit})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.InviteTenantMember(ctx, &accessv1.InviteTenantMemberRequest{Email: "member@example.com"})
	if !errors.Is(err, ErrTenantContextRequired) {
		t.Fatalf("err=%v", err)
	}
}

func TestTenantMemberLifecycleUsesPrincipalTenantAndJoinedRootUoW(t *testing.T) {
	repository := newMemoryTenantMemberRepository()
	unit := &tenantTestUnit{}
	factoryCalls := 0
	factory := requestscope.RepositoryFactory[ports.TenantMemberRepositories](func(_ context.Context, got requestscope.UnitOfWork) (ports.TenantMemberRepositories, error) {
		factoryCalls++
		if got != unit {
			t.Fatalf("joined unit=%T %p want=%p", got, got, unit)
		}
		return ports.TenantMemberRepositories{Member: repository}, nil
	})
	service, err := NewTenantMemberLifecycleService(factory)
	if err != nil {
		t.Fatal(err)
	}

	root := func(operation, tenantID string) context.Context {
		base := identity.WithPrincipal(context.Background(), identity.Principal{
			Subject: "user:admin-" + tenantID,
			TenantID: tenantID,
			UserID: "admin-" + tenantID,
			Authenticated: true,
			AuthMethod: identity.AuthMethodAPIKey,
		})
		ctx, _, err := execution.BeginRoot(base, operation, execution.TransactionLocal, nil, tenantTestTransactionFactory{unit: unit})
		if err != nil {
			t.Fatal(err)
		}
		return ctx
	}

	invited, err := service.InviteTenantMember(root("tenant.member.invite", "tenant-a"), &accessv1.InviteTenantMemberRequest{Email: "member@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if invited.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_INVITED || invited.GetVersion() != 1 {
		t.Fatalf("invited=%+v", invited)
	}

	// The target tenant is never read from the request. A principal from another
	// tenant cannot address tenant-a's member by knowing its user id.
	_, err = service.GetTenantMember(root("tenant.member.get", "tenant-b"), &accessv1.GetTenantMemberRequest{UserId: invited.GetUserId()})
	if !errors.Is(err, ports.ErrTenantMemberNotFound) {
		t.Fatalf("cross-tenant get err=%v", err)
	}

	active, err := service.ActivateTenantMember(root("tenant.member.activate", "tenant-a"), &accessv1.ActivateTenantMemberRequest{UserId: invited.GetUserId(), Version: invited.GetVersion()})
	if err != nil {
		t.Fatal(err)
	}
	if active.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_ACTIVE || active.GetVersion() != 2 {
		t.Fatalf("active=%+v", active)
	}

	suspended, err := service.SuspendTenantMember(root("tenant.member.suspend", "tenant-a"), &accessv1.SuspendTenantMemberRequest{UserId: invited.GetUserId(), Version: active.GetVersion()})
	if err != nil {
		t.Fatal(err)
	}
	if suspended.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_SUSPENDED || suspended.GetVersion() != 3 {
		t.Fatalf("suspended=%+v", suspended)
	}

	removed, err := service.RemoveTenantMember(root("tenant.member.remove", "tenant-a"), &accessv1.RemoveTenantMemberRequest{UserId: invited.GetUserId(), Version: suspended.GetVersion()})
	if err != nil {
		t.Fatal(err)
	}
	if removed.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_REMOVED || removed.GetVersion() != 4 {
		t.Fatalf("removed=%+v", removed)
	}

	_, err = service.ActivateTenantMember(root("tenant.member.activate", "tenant-a"), &accessv1.ActivateTenantMemberRequest{UserId: invited.GetUserId(), Version: removed.GetVersion()})
	if !errors.Is(err, domain.ErrInvalidTenantMemberTransition) {
		t.Fatalf("removed activate err=%v", err)
	}
	if factoryCalls != 6 {
		t.Fatalf("repository factory calls=%d want=6", factoryCalls)
	}
}

func TestBootstrapTenantOwnerMemberUsesExplicitTenantInsideRootScope(t *testing.T) {
	repository := newMemoryTenantMemberRepository()
	unit := &tenantTestUnit{}
	factory := requestscope.RepositoryFactory[ports.TenantMemberRepositories](func(_ context.Context, got requestscope.UnitOfWork) (ports.TenantMemberRepositories, error) {
		if got != unit {
			t.Fatalf("joined unit=%T %p want=%p", got, got, unit)
		}
		return ports.TenantMemberRepositories{Member: repository}, nil
	})
	service, err := NewTenantMemberLifecycleService(factory)
	if err != nil {
		t.Fatal(err)
	}
	ctx, _, err := execution.BeginRoot(context.Background(), "tenant.create", execution.TransactionLocal, []string{"tenant.member.bootstrap_owner"}, tenantTestTransactionFactory{unit: unit})
	if err != nil {
		t.Fatal(err)
	}
	child, err := execution.JoinChild(ctx, "tenant.member.bootstrap_owner", execution.TransactionLocal, nil)
	if err != nil {
		t.Fatal(err)
	}
	member, err := service.BootstrapTenantOwnerMember(child, &accessv1.BootstrapTenantOwnerMemberRequest{
		TenantId: "tenant-bootstrap", UserId: "owner-bootstrap", Email: "owner@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if member.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_ACTIVE || member.GetVersion() != 1 {
		t.Fatalf("member=%+v", member)
	}
}
