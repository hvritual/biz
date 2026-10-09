package bizruntime

import (
	"context"
	"errors"
	"fmt"
	"testing"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessports "github.com/hvritual/biz/internal/access/ports"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type subscriptionTenantChildFunc func(context.Context, *accessv1.GetTenantRequest) (*accessv1.TenantDTO, error)

func (f subscriptionTenantChildFunc) GetTenant(ctx context.Context, request *accessv1.GetTenantRequest) (*accessv1.TenantDTO, error) {
	return f(ctx, request)
}

func TestSubscriptionTenantReaderPreservesAbsenceAndFailureBoundaries(t *testing.T) {
	transportNotFound := status.Error(codes.NotFound, "tenant missing")
	denied := status.Error(codes.PermissionDenied, "denied")
	unavailable := errors.New("database unavailable")
	for _, tc := range []struct {
		name  string
		input error
		want  error
	}{
		{"local absence", accessports.ErrTenantNotFound, change.ErrNotFound},
		{"wrapped local absence", fmt.Errorf("tenant.get: %w", accessports.ErrTenantNotFound), change.ErrNotFound},
		{"transport absence", transportNotFound, transportNotFound},
		{"denied", denied, denied},
		{"unavailable", unavailable, unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := &accessv1.GetTenantRequest{Id: "tenant-a"}
			calls := 0
			ctx := context.Background()
			capabilities := subscriptionChangeCapabilities{tenant: subscriptionTenantChildFunc(func(gotCtx context.Context, got *accessv1.GetTenantRequest) (*accessv1.TenantDTO, error) {
				calls++
				if gotCtx != ctx || got != request {
					t.Fatal("child context or request replaced")
				}
				return nil, tc.input
			})}
			value, err := capabilities.AccessTenantLifecycle().GetTenant(ctx, request)
			if value != nil || !errors.Is(err, tc.want) || calls != 1 {
				t.Fatalf("value=%v err=%v calls=%d want=%v", value, err, calls, tc.want)
			}
		})
	}
}

func TestSubscriptionTenantReaderPreservesSuccessfulReadAndMissingWiring(t *testing.T) {
	if (subscriptionChangeCapabilities{}).AccessTenantLifecycle() != nil {
		t.Fatal("missing dependency was hidden by adapter")
	}
	want := &accessv1.TenantDTO{Id: "tenant-a"}
	capabilities := subscriptionChangeCapabilities{tenant: subscriptionTenantChildFunc(func(context.Context, *accessv1.GetTenantRequest) (*accessv1.TenantDTO, error) {
		return want, nil
	})}
	got, err := capabilities.AccessTenantLifecycle().GetTenant(context.Background(), &accessv1.GetTenantRequest{Id: want.Id})
	if got != want || err != nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
}
