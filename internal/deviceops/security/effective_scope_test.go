package security

import (
	"context"
	"errors"
	"testing"

	devicev1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/access/domain"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

type currentScopeStub struct {
	scope domain.EffectiveBusinessScope
	err   error
	calls int
}

func (s *currentScopeStub) ResolveBusinessScope(_ context.Context, tenant, user string, permission authz.PermissionKey) (domain.EffectiveBusinessScope, error) {
	s.calls++
	if tenant != "t" || user != "u" || permission != "device.read" {
		return domain.EffectiveBusinessScope{}, errors.New("wrong request binding")
	}
	return s.scope, s.err
}

func TestPolicyGuardReevaluatesAndBoundsAllAndSelf(t *testing.T) {
	source := &currentScopeStub{scope: domain.EffectiveBusinessScope{All: true, Self: true, PolicyBound: true, PolicySiteIDs: []string{"a"}}}
	guard, err := NewGuard(source)
	if err != nil {
		t.Fatal(err)
	}
	authorized := authz.AuthorizedOperation{Principal: identity.Principal{Authenticated: true, TenantID: "t", UserID: "u"}, Policy: authz.Policy{Operation: "device.get"}, Decision: authz.Decision{Allowed: true, Grants: []authz.Grant{{Permission: "device.read", Scope: "all"}}}}
	ctx, err := guard.Prepare(context.Background(), authorized, nil)
	if err != nil {
		t.Fatal(err)
	}
	scope, _ := FromContext(ctx)
	if !scope.AllowsSite("a") || scope.AllowsSite("b") {
		t.Fatalf("ALL/SELF escaped policy ceiling: %+v", scope)
	}
	source.scope.PolicySiteIDs = nil
	ctx, err = guard.Prepare(context.Background(), authorized, nil)
	if err != nil {
		t.Fatal(err)
	}
	scope, _ = FromContext(ctx)
	if scope.AllowsSite("a") || source.calls != 2 {
		t.Fatalf("stale allow survived contraction: %+v calls=%d", scope, source.calls)
	}
	source.err = domain.ErrInvalidTenantDataPolicy
	if _, err = guard.Prepare(context.Background(), authorized, nil); err == nil {
		t.Fatal("invalid reference allowed")
	}
	if _, err = NewGuard(nil); err == nil {
		t.Fatal("production policy resolver optional")
	}
}

func TestPolicyCeilingBlocksWriteTargets(t *testing.T) {
	scope := Scope{All: true, Self: true, PolicyBound: true, PolicySiteIDs: []string{"a"}}
	if scope.AllowsSite("b") {
		t.Fatal("forbidden target accepted")
	}
	// Input request types remain checked in Guard.Prepare; empty ceiling is
	// restrictive, never interpreted as unrestricted.
	request := &devicev1.CreateDeviceRequest{SiteId: "b"}
	if scope.AllowsSite(request.GetSiteId()) {
		t.Fatal("create escaped ceiling")
	}
}
