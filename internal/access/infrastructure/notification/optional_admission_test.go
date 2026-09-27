package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
)

type preferenceReaderFunc func(context.Context, domain.NotificationPreferenceOwner, domain.NotificationPreferenceChannel) (domain.NotificationPreference, error)

func (f preferenceReaderFunc) ReadNotificationPreference(ctx context.Context, owner domain.NotificationPreferenceOwner, channel domain.NotificationPreferenceChannel) (domain.NotificationPreference, error) {
	return f(ctx, owner, channel)
}

func TestOptionalNotificationAdmissionFailsClosed(t *testing.T) {
	owner := domain.NotificationPreferenceOwner{TenantID: "tenant-a", UserID: "user-a"}
	now := time.Now().UTC()
	failure := errors.New("reader unavailable")
	for _, item := range []struct {
		name       string
		preference domain.NotificationPreference
		err        error
		wantCalls  int
	}{
		{"missing", domain.NotificationPreference{Channel: domain.NotificationPreferenceSMS, State: domain.NotificationPreferenceDefault}, nil, 1},
		{"allow", domain.NotificationPreference{Channel: domain.NotificationPreferenceSMS, State: domain.NotificationPreferenceAllow, Version: 1, UpdatedAt: &now}, nil, 1},
		{"deny", domain.NotificationPreference{Channel: domain.NotificationPreferenceSMS, State: domain.NotificationPreferenceDeny, Version: 1, UpdatedAt: &now}, nil, 0},
		{"reader-error", domain.NotificationPreference{}, failure, 0},
		{"corrupt", domain.NotificationPreference{Channel: domain.NotificationPreferenceSMS, State: domain.NotificationPreferenceAllow}, nil, 0},
		{"wrong-channel", domain.NotificationPreference{Channel: domain.NotificationPreferenceEmail, State: domain.NotificationPreferenceDefault}, nil, 0},
	} {
		t.Run(item.name, func(t *testing.T) {
			reader := preferenceReaderFunc(func(_ context.Context, got domain.NotificationPreferenceOwner, channel domain.NotificationPreferenceChannel) (domain.NotificationPreference, error) {
				if got != owner || channel != domain.NotificationPreferenceSMS {
					t.Fatal("recipient binding changed")
				}
				return item.preference, item.err
			})
			calls := 0
			accepted, err := AdmitOptionalNotification(context.Background(), reader, owner, domain.NotificationPreferenceSMS, func(context.Context) error { calls++; return nil })
			if calls != item.wantCalls || accepted != (item.wantCalls == 1) {
				t.Fatalf("accepted=%v enqueue calls=%d", accepted, calls)
			}
			if item.err != nil && !errors.Is(err, item.err) {
				t.Fatalf("reader error lost: %v", err)
			}
			if (item.name == "corrupt" || item.name == "wrong-channel") && err == nil {
				t.Fatal("invalid preference silently treated as refusal/default")
			}
		})
	}
}

func TestOptionalNotificationAdmissionPropagatesCancellationAndEnqueueFailure(t *testing.T) {
	owner := domain.NotificationPreferenceOwner{TenantID: "tenant-a", UserID: "user-a"}
	ctx, cancel := context.WithCancel(context.Background())
	reader := preferenceReaderFunc(func(context.Context, domain.NotificationPreferenceOwner, domain.NotificationPreferenceChannel) (domain.NotificationPreference, error) {
		cancel()
		return domain.NotificationPreference{Channel: domain.NotificationPreferenceSMS, State: domain.NotificationPreferenceDefault}, nil
	})
	calls := 0
	if accepted, err := AdmitOptionalNotification(ctx, reader, owner, domain.NotificationPreferenceSMS, func(context.Context) error { calls++; return nil }); accepted || !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatalf("cancelled admission: %v / %v / %d", accepted, err, calls)
	}
	reader = preferenceReaderFunc(func(context.Context, domain.NotificationPreferenceOwner, domain.NotificationPreferenceChannel) (domain.NotificationPreference, error) {
		return domain.NotificationPreference{Channel: domain.NotificationPreferenceSMS, State: domain.NotificationPreferenceDefault}, nil
	})
	failure := errors.New("outbox unavailable")
	if accepted, err := AdmitOptionalNotification(context.Background(), reader, owner, domain.NotificationPreferenceSMS, func(context.Context) error { return failure }); accepted || !errors.Is(err, failure) {
		t.Fatalf("enqueue failure: %v / %v", accepted, err)
	}
	if _, err := AdmitOptionalNotification(context.Background(), nil, owner, domain.NotificationPreferenceSMS, nil); err == nil {
		t.Fatal("missing ports accepted")
	}
}
