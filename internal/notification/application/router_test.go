package application

import (
	"context"
	"errors"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accessports "github.com/hvritual/biz/internal/access/ports"
	deviceports "github.com/hvritual/biz/internal/deviceops/ports"
	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
)

type routeQueueFake struct {
	claim domain.RoutingClaim
	plan  domain.RoutePlan
}

func (f *routeQueueFake) ClaimNextBusinessEvent(context.Context, string, time.Duration) (domain.RoutingClaim, error) {
	return f.claim, nil
}
func (f *routeQueueFake) CompleteBusinessEventRoute(_ context.Context, _ domain.RoutingClaim, p domain.RoutePlan) (domain.RoutingResult, error) {
	f.plan = p
	return domain.RoutingResult{EventID: f.claim.Event.EventID, State: domain.RoutingStateRouted}, nil
}

type routeConfigFake struct {
	config domain.Configuration
	found  bool
}

func (f routeConfigFake) FindRoutingConfiguration(context.Context, string, string, domain.MessageLevel) (domain.Configuration, bool, error) {
	return f.config, f.found, nil
}

type routeGroupsFake struct{ active bool }

func (f routeGroupsFake) NotificationRouteGroupActive(context.Context, string, string) (bool, error) {
	return f.active, nil
}

type routeRecipientsFake struct{ active map[string]bool }

func (f routeRecipientsFake) ResolveNotificationRouteRecipients(_ context.Context, _ string, ids []string) ([]accessports.NotificationRouteRecipient, error) {
	out := make([]accessports.NotificationRouteRecipient, 0, len(ids))
	for _, id := range ids {
		out = append(out, accessports.NotificationRouteRecipient{UserID: id, Active: f.active[id]})
	}
	return out, nil
}

type routePreferenceFake struct {
	states map[string]accessdomain.NotificationPreferenceState
}

func (f routePreferenceFake) ReadNotificationPreference(_ context.Context, owner accessdomain.NotificationPreferenceOwner, channel accessdomain.NotificationPreferenceChannel) (accessdomain.NotificationPreference, error) {
	state := f.states[owner.UserID+"/"+string(channel)]
	if state == "" {
		state = accessdomain.NotificationPreferenceDefault
	}
	if state == accessdomain.NotificationPreferenceDefault {
		return accessdomain.NotificationPreference{Channel: channel, State: state}, nil
	}
	now := time.Now().UTC()
	return accessdomain.NotificationPreference{Channel: channel, State: state, Version: 1, UpdatedAt: &now}, nil
}

type routePreferenceErrorFake struct{}

func (routePreferenceErrorFake) ReadNotificationPreference(context.Context, accessdomain.NotificationPreferenceOwner, accessdomain.NotificationPreferenceChannel) (accessdomain.NotificationPreference, error) {
	return accessdomain.NotificationPreference{}, accessdomain.ErrNotificationPreferenceUnavailable
}

func routeFixture(t *testing.T, channels []domain.Channel, preferences map[string]accessdomain.NotificationPreferenceState, active map[string]bool) (*BusinessEventRouter, *routeQueueFake) {
	t.Helper()
	types, err := domain.NewMessageTypeCatalog([]domain.MessageType{{Code: "device.fault", Name: "设备故障", Level: domain.LevelUrgent}})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := domain.NewChannelRegistry(channels)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	queue := &routeQueueFake{claim: domain.RoutingClaim{Event: domain.BusinessEvent{EventID: "event-185", TenantID: "tenant-185", GroupID: "site-185", TypeCode: "device.fault", Level: domain.LevelUrgent, TraceID: "trace-185", ReferenceKind: "device", ReferenceID: "device-185", OccurredAt: now}, WorkerID: "router-185", LeaseToken: 1, LeaseUntil: now.Add(time.Minute), Attempt: 1}}
	config := domain.Configuration{ID: "config-185", TenantID: "tenant-185", GroupID: "site-185", Level: domain.LevelUrgent, Channels: []string{}, PrimaryUserID: "user-a", SecondaryUserID: "user-b", AdditionalUserIDs: []string{"user-c"}, Version: 1, CreatedAt: now, UpdatedAt: now}
	for _, c := range channels {
		config.Channels = append(config.Channels, c.Code)
	}
	router, err := NewBusinessEventRouter(ports.RoutingDependencies{Queue: queue, Configurations: routeConfigFake{config: config, found: true}, Recipients: routeRecipientsFake{active: active}, Groups: routeGroupsFake{active: true}, Preferences: routePreferenceFake{states: preferences}}, types, registry, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return router, queue
}

func decisionsBy(plan domain.RoutePlan) map[string]string {
	out := map[string]string{}
	for _, d := range plan.Decisions {
		out[d.UserID+"/"+d.Channel] = d.Outcome
	}
	return out
}

func TestEnterprise185RouterAdmitsInAppAndAllowedOptionalButNotDenied(t *testing.T) {
	router, queue := routeFixture(t, []domain.Channel{{Code: "in_app", Name: "站内", Availability: domain.ChannelConfigurable}, {Code: "email", Name: "邮件", Availability: domain.ChannelConfigurable}}, map[string]accessdomain.NotificationPreferenceState{"user-a/email": accessdomain.NotificationPreferenceDeny, "user-b/email": accessdomain.NotificationPreferenceAllow}, map[string]bool{"user-a": true, "user-b": true, "user-c": true})
	if _, err := router.RouteOnce(context.Background(), "router-185"); err != nil {
		t.Fatal(err)
	}
	got := decisionsBy(queue.plan)
	if got["user-a/in_app"] != domain.RouteOutcomeInAppCreated || got["user-a/email"] != domain.RouteOutcomePreferenceDenied {
		t.Fatalf("user-a=%v", got)
	}
	if got["user-b/email"] != domain.RouteOutcomeExternalTask || got["user-c/email"] != domain.RouteOutcomeExternalTask {
		t.Fatalf("default/allow not admitted: %v", got)
	}
}

func TestEnterprise185RouterRejectsUnavailableChannelAndInactiveRecipient(t *testing.T) {
	router, queue := routeFixture(t, []domain.Channel{{Code: "in_app", Name: "站内", Availability: domain.ChannelConfigurable}, {Code: "sms", Name: "短信", Availability: domain.ChannelNotConfigurable, UnavailableReason: "disabled"}}, nil, map[string]bool{"user-a": true, "user-b": false, "user-c": true})
	if _, err := router.RouteOnce(context.Background(), "router-185"); err != nil {
		t.Fatal(err)
	}
	got := decisionsBy(queue.plan)
	if got["user-a/sms"] != domain.RouteOutcomeChannelUnavailable || got["user-b/in_app"] != domain.RouteOutcomeRecipientInactive || got["user-b/sms"] != domain.RouteOutcomeRecipientInactive {
		t.Fatalf("routing decisions=%v", got)
	}
}

func TestEnterprise185RouterNoConfigurationIsExplicit(t *testing.T) {
	types, _ := domain.NewMessageTypeCatalog([]domain.MessageType{{Code: "device.fault", Name: "设备故障", Level: domain.LevelUrgent}})
	channels, _ := domain.NewChannelRegistry([]domain.Channel{{Code: "in_app", Name: "站内", Availability: domain.ChannelConfigurable}})
	now := time.Now().UTC()
	queue := &routeQueueFake{claim: domain.RoutingClaim{Event: domain.BusinessEvent{EventID: "event-185", TenantID: "tenant-185", GroupID: "site-185", TypeCode: "device.fault", Level: domain.LevelUrgent, TraceID: "trace-185", ReferenceKind: "device", ReferenceID: "device-185", OccurredAt: now}, WorkerID: "router-185", LeaseToken: 1, LeaseUntil: now.Add(time.Minute), Attempt: 1}}
	router, err := NewBusinessEventRouter(ports.RoutingDependencies{Queue: queue, Configurations: routeConfigFake{found: false}, Recipients: routeRecipientsFake{}, Groups: routeGroupsFake{active: true}, Preferences: routePreferenceFake{}}, types, channels, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = router.RouteOnce(context.Background(), "router-185"); err != nil {
		t.Fatal(err)
	}
	if len(queue.plan.Decisions) != 1 || queue.plan.Decisions[0].Outcome != domain.RouteOutcomeNoConfiguration {
		t.Fatalf("plan=%+v", queue.plan)
	}
}

func TestEnterprise185RouterPropagatesPreferenceFailureWithoutCompletingRoute(t *testing.T) {
	router, queue := routeFixture(t, []domain.Channel{{Code: "email", Name: "邮件", Availability: domain.ChannelConfigurable}}, nil, map[string]bool{"user-a": true, "user-b": true, "user-c": true})
	router.deps.Preferences = routePreferenceErrorFake{}
	if _, err := router.RouteOnce(context.Background(), "router-185"); !errors.Is(err, accessdomain.ErrNotificationPreferenceUnavailable) {
		t.Fatalf("error=%v", err)
	}
	if len(queue.plan.Decisions) != 0 {
		t.Fatalf("route completed despite preference failure: %+v", queue.plan)
	}
}

func TestEnterprise185RouterRejectsRetiredGroupBeforeConfiguration(t *testing.T) {
	types, _ := domain.NewMessageTypeCatalog([]domain.MessageType{{Code: "device.fault", Name: "设备故障", Level: domain.LevelUrgent}})
	channels, _ := domain.NewChannelRegistry([]domain.Channel{{Code: "in_app", Name: "站内", Availability: domain.ChannelConfigurable}})
	now := time.Now().UTC()
	queue := &routeQueueFake{claim: domain.RoutingClaim{Event: domain.BusinessEvent{EventID: "event-retired", TenantID: "tenant-185", GroupID: "site-retired", TypeCode: "device.fault", Level: domain.LevelUrgent, TraceID: "trace-retired", ReferenceKind: "device", ReferenceID: "device-retired", OccurredAt: now}, WorkerID: "router-185", LeaseToken: 1, LeaseUntil: now.Add(time.Minute), Attempt: 1}}
	router, err := NewBusinessEventRouter(ports.RoutingDependencies{Queue: queue, Configurations: routeConfigFake{found: true}, Recipients: routeRecipientsFake{}, Groups: routeGroupsFake{active: false}, Preferences: routePreferenceFake{}}, types, channels, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = router.RouteOnce(context.Background(), "router-185"); err != nil {
		t.Fatal(err)
	}
	if len(queue.plan.Decisions) != 1 || queue.plan.Decisions[0].Outcome != domain.RouteOutcomeGroupUnavailable {
		t.Fatalf("plan=%+v", queue.plan)
	}
}

var _ ports.RoutingQueue = (*routeQueueFake)(nil)
var _ ports.RoutingConfigurationReader = routeConfigFake{}
var _ accessports.NotificationRouteRecipients = routeRecipientsFake{}
var _ accessports.OptionalNotificationPreferenceReader = routePreferenceFake{}
var _ deviceports.NotificationRouteGroups = routeGroupsFake{}
