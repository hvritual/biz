//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	"github.com/hvritual/biz/internal/notification/application"
	"github.com/hvritual/biz/internal/notification/domain"
	notificationpersistence "github.com/hvritual/biz/internal/notification/infrastructure/persistence"
	"github.com/hvritual/biz/internal/notification/ports"
	"gorm.io/gorm"
	"yunka.io/gateway/authz"
)

func TestEnterprise185BusinessEventRoutingUsesConfigurationRecipientsAndPreferences(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = notificationpersistence.MigrateConfigurations(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = notificationpersistence.MigrateRouting(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err = devicepersistence.AutoMigrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	stamp := fmt.Sprint(time.Now().UnixNano())
	tenant, other := "n185-route-"+stamp, "n185-other-"+stamp
	owner, contact, inactive := "n185-owner-"+stamp, "n185-contact-"+stamp, "n185-inactive-"+stamp
	group, otherGroup := "n185-site-"+stamp, "n185-other-site-"+stamp
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	must(store.Bootstrap(ctx, accesspersistence.Bootstrap{TenantID: tenant, TenantName: tenant, UserID: owner, Email: owner + "@example.invalid", Token: "n185-token-" + stamp}, []authz.PermissionKey{"tenant.notification.read"}))
	must(store.Bootstrap(ctx, accesspersistence.Bootstrap{TenantID: other, TenantName: other, UserID: "n185-other-owner-" + stamp, Email: "n185-other-" + stamp + "@example.invalid", Token: "n185-other-token-" + stamp}, []authz.PermissionKey{"tenant.notification.read"}))
	seedReader(t, db, tenant, contact, "n185-contact-token-"+stamp, "", tenant+":contact", "route contact", "tenant.notification.read", "all")
	seedReader(t, db, tenant, inactive, "n185-inactive-token-"+stamp, "", tenant+":inactive", "route inactive", "tenant.notification.read", "all")
	must(db.Table("biz_memberships").Where("tenant_id=? AND user_id=?", tenant, inactive).Update("status", accessdomain.TenantMemberStatusSuspended).Error)

	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, site := range []struct{ tenant, id string }{{tenant, group}, {other, otherGroup}} {
		must(db.Create(&devicepersistence.SitePORecord{SitePO: devicepersistence.SitePO{Name: site.id}, SitePOBase: devicepersistence.SitePOBase{ID: site.id, TenantID: site.tenant, Version: 1, CreatedAt: now, UpdatedAt: now}}).Error)
	}
	configID := "n185-config-" + stamp
	must(db.Exec(`INSERT INTO biz_notification_configurations (id,tenant_id,group_id,level,primary_user_id,secondary_user_id,notes,version,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, configID, tenant, group, string(domain.LevelUrgent), owner, contact, "route fixture", 1, now, now).Error)
	for _, channel := range []string{"in_app", "email"} {
		must(db.Exec(`INSERT INTO biz_notification_configuration_channels (tenant_id,configuration_id,channel) VALUES (?,?,?)`, tenant, configID, channel).Error)
	}
	must(db.Exec(`INSERT INTO biz_notification_configuration_recipients (tenant_id,configuration_id,user_id) VALUES (?,?,?)`, tenant, configID, inactive).Error)
	must(db.Exec(`INSERT INTO biz_notification_preferences (tenant_id,user_id,channel,state,version,updated_at) VALUES (?,?,?,?,?,?)`, tenant, owner, "email", "deny", 1, now).Error)

	routing, err := notificationpersistence.NewRoutingRepository(db)
	must(err)
	recipients, err := accesspersistence.NewNotificationRouteRecipientDirectory(db)
	must(err)
	groups, err := devicepersistence.NewNotificationRouteGroupDirectory(db)
	must(err)
	types, err := domain.NewMessageTypeCatalog([]domain.MessageType{{Code: "device.fault", Name: "设备故障", Level: domain.LevelUrgent}})
	must(err)
	channels, err := domain.NewChannelRegistry([]domain.Channel{{Code: "in_app", Name: "站内消息", Availability: domain.ChannelConfigurable}, {Code: "email", Name: "邮件", Availability: domain.ChannelConfigurable}})
	must(err)
	router, err := application.NewBusinessEventRouter(ports.RoutingDependencies{Queue: routing, Configurations: routing, Recipients: recipients, Groups: groups, Preferences: store}, types, channels, 30*time.Second)
	must(err)

	event := domain.BusinessEvent{EventID: "n185-event-" + stamp, TenantID: tenant, GroupID: group, TypeCode: "device.fault", Level: domain.LevelUrgent, TraceID: "trace-" + stamp, ReferenceKind: "device", ReferenceID: "machine-" + stamp, OccurredAt: now}
	t.Run("source-transaction-rollback-has-no-executable-event", func(t *testing.T) {
		err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			publisher, e := notificationpersistence.NewEventPublisher(tx)
			if e != nil {
				return e
			}
			if e = publisher.AppendBusinessEvent(ctx, event); e != nil {
				return e
			}
			return errors.New("rollback")
		})
		if err == nil {
			t.Fatal("rollback fixture unexpectedly committed")
		}
		var count int64
		must(db.Table("biz_notification_events").Where("event_id=?", event.EventID).Count(&count).Error)
		if count != 0 {
			t.Fatalf("rolled back event count=%d", count)
		}
	})
	appendEvent := func(value domain.BusinessEvent) error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			publisher, e := notificationpersistence.NewEventPublisher(tx)
			if e != nil {
				return e
			}
			return publisher.AppendBusinessEvent(ctx, value)
		})
	}
	must(appendEvent(event))
	must(appendEvent(event))
	changed := event
	changed.ReferenceID = "other-machine-" + stamp
	if err := appendEvent(changed); !errors.Is(err, domain.ErrRoutingConflict) {
		t.Fatalf("changed replay accepted: %v", err)
	}

	t.Run("route-creates-inapp-and-only-allowed-external-task", func(t *testing.T) {
		result, err := router.RouteOnce(ctx, "router-185")
		if err != nil {
			t.Fatal(err)
		}
		if result.State != domain.RoutingStateRouted || result.InAppCreated != 2 || result.ExternalTasks != 1 || result.Denied != 1 || result.Inactive != 2 {
			t.Fatalf("route result=%+v", result)
		}
		var inapp, external, outcomes int64
		must(db.Table("biz_notification_in_app").Where("event_id=?", event.EventID).Count(&inapp).Error)
		must(db.Table("biz_notification_external_tasks").Where("event_id=?", event.EventID).Count(&external).Error)
		must(db.Table("biz_notification_route_outcomes").Where("event_id=?", event.EventID).Count(&outcomes).Error)
		if inapp != 2 || external != 1 || outcomes != 6 {
			t.Fatalf("inapp=%d external=%d outcomes=%d", inapp, external, outcomes)
		}
		var task struct{ UserID, Channel, State string }
		must(db.Table("biz_notification_external_tasks").Where("event_id=?", event.EventID).Take(&task).Error)
		if task.UserID != contact || task.Channel != "email" || task.State != accessdomain.NotificationStatePending {
			t.Fatalf("external task=%+v", task)
		}
		var denied int64
		must(db.Table("biz_notification_route_outcomes").Where("event_id=? AND user_id=? AND channel='email' AND outcome=?", event.EventID, owner, domain.RouteOutcomePreferenceDenied).Count(&denied).Error)
		if denied != 1 {
			t.Fatal("explicit deny did not produce denial outcome")
		}
		var maxLatency int64
		must(db.Raw(`SELECT COALESCE(MAX(TIMESTAMPDIFF(MICROSECOND,e.occurred_at,m.created_at)),0) FROM biz_notification_in_app m JOIN biz_notification_events e ON e.event_id=m.event_id WHERE m.event_id=?`, event.EventID).Scan(&maxLatency).Error)
		if maxLatency < 0 || maxLatency >= int64(60*time.Second/time.Microsecond) {
			t.Fatalf("in-app latency=%dus", maxLatency)
		}
	})

	t.Run("routed-event-is-not-produced-again", func(t *testing.T) {
		result, err := router.RouteOnce(ctx, "router-185-second")
		if err != nil {
			t.Fatal(err)
		}
		if result.EventID != "" {
			t.Fatalf("unexpected second route %+v", result)
		}
		var inapp, external int64
		must(db.Table("biz_notification_in_app").Where("event_id=?", event.EventID).Count(&inapp).Error)
		must(db.Table("biz_notification_external_tasks").Where("event_id=?", event.EventID).Count(&external).Error)
		if inapp != 2 || external != 1 {
			t.Fatalf("replay duplicated outputs inapp=%d external=%d", inapp, external)
		}
	})

	t.Run("other-tenant-without-configuration-is-explicit", func(t *testing.T) {
		otherEvent := event
		otherEvent.EventID = "n185-event-other-" + stamp
		otherEvent.TenantID = other
		otherEvent.GroupID = otherGroup
		otherEvent.TraceID = "trace-other-" + stamp
		must(appendEvent(otherEvent))
		result, err := router.RouteOnce(ctx, "router-185-other")
		if err != nil {
			t.Fatal(err)
		}
		if result.EventID != otherEvent.EventID || result.NoConfiguration != 1 || result.InAppCreated != 0 || result.ExternalTasks != 0 {
			t.Fatalf("other tenant result=%+v", result)
		}
	})
}
