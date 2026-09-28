//go:build integration && enterprise189fixture

package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	notificationdomain "github.com/hvritual/biz/internal/notification/domain"
	notificationpersistence "github.com/hvritual/biz/internal/notification/infrastructure/persistence"
	"gorm.io/gorm"
)

// TestEnterprise189AppendControlledBusinessEvent is a qualification-only ingress.
// Production intentionally exposes no browser endpoint that can fabricate business
// events. The browser E2E calls this test against the already-running isolated CE12
// database, then the real notification runtime claims and routes the committed event.
func TestEnterprise189AppendControlledBusinessEvent(t *testing.T) {
	tenantID := strings.TrimSpace(os.Getenv("ENTERPRISE189_EVENT_TENANT"))
	groupID := strings.TrimSpace(os.Getenv("ENTERPRISE189_EVENT_GROUP"))
	eventID := strings.TrimSpace(os.Getenv("ENTERPRISE189_EVENT_ID"))
	referenceID := strings.TrimSpace(os.Getenv("ENTERPRISE189_EVENT_REFERENCE"))
	if tenantID == "" || groupID == "" || eventID == "" || referenceID == "" {
		t.Skip("enterprise #189 controlled event fixture is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db := openDB(t)
	if err := notificationpersistence.MigrateRouting(ctx, db); err != nil {
		t.Fatal(err)
	}

	occurredAt := time.Now().UTC().Truncate(time.Microsecond)
	event := notificationdomain.BusinessEvent{
		EventID:       eventID,
		TenantID:      tenantID,
		GroupID:       groupID,
		TypeCode:      "system.announcement",
		Level:         notificationdomain.LevelGeneral,
		TraceID:       "enterprise189-" + eventID,
		ReferenceKind: "site",
		ReferenceID:   referenceID,
		OccurredAt:    occurredAt,
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("controlled event invalid: %v", err)
	}

	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		publisher, err := notificationpersistence.NewEventPublisher(tx)
		if err != nil {
			return err
		}
		return publisher.AppendBusinessEvent(ctx, event)
	}); err != nil {
		t.Fatal(err)
	}

	var state string
	if err := db.WithContext(ctx).Table("biz_notification_events").
		Select("state").
		Where("event_id = ?", eventID).
		Scan(&state).Error; err != nil {
		t.Fatal(err)
	}
	switch state {
	case notificationdomain.RoutingStatePending, notificationdomain.RoutingStateLeased, notificationdomain.RoutingStateRouted:
	default:
		t.Fatalf("unexpected controlled event state %q", state)
	}
	t.Logf("ENTERPRISE189_CONTROLLED_EVENT=%s state=%s", eventID, state)
}
