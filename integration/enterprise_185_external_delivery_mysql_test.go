//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	notificationapp "github.com/hvritual/biz/internal/notification/application"
	"github.com/hvritual/biz/internal/notification/domain"
	notificationdelivery "github.com/hvritual/biz/internal/notification/infrastructure/delivery"
	notificationpersistence "github.com/hvritual/biz/internal/notification/infrastructure/persistence"
	"github.com/hvritual/biz/internal/notification/ports"
	"yunka.io/gateway/authz"
)

func TestEnterprise185ExternalDeliveryRechecksPreferenceAndProtectedContact(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	protection, err := accesspersistence.NewContactProtection(accesspersistence.ContactProtectionConfig{
		ActiveVersion: "v1",
		Keys:          map[string][]byte{"v1": []byte(strings.Repeat("C", 32))},
		LookupKey:     []byte(strings.Repeat("L", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := accesspersistence.NewWithContactProtection(db, protection)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = notificationpersistence.MigrateRouting(ctx, db); err != nil {
		t.Fatal(err)
	}

	stamp := fmt.Sprint(time.Now().UnixNano())
	tenant, user := "n185-delivery-"+stamp, "n185-user-"+stamp
	email := "delivery-" + stamp + "@example.invalid"
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{TenantID: tenant, TenantName: tenant, UserID: user, Email: email, Token: "n185-token-" + stamp}, []authz.PermissionKey{"tenant.notification.read"}); err != nil {
		t.Fatal(err)
	}
	enterprise182SetTenantContacts(t, db, protection, tenant, user, email, "+491701234567")
	owner := accessdomain.NotificationPreferenceOwner{TenantID: tenant, UserID: user}
	repository, err := notificationpersistence.NewRoutingRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	provider := notificationdelivery.NewMemoryProvider()
	worker, err := notificationapp.NewExternalDeliveryWorker(
		ports.ExternalDeliveryDependencies{Tasks: repository, Admission: store, Provider: provider},
		domain.EnterpriseExternalDeliveryPolicy(),
		"external-worker-185",
	)
	if err != nil {
		t.Fatal(err)
	}

	insertTask := func(id, channel string) {
		t.Helper()
		now := time.Now().UTC()
		if err := db.Exec(`INSERT INTO biz_notification_external_tasks
(task_id,tenant_id,user_id,event_id,channel,configuration_id,configuration_version,group_id,type_code,level,trace_id,reference_kind,reference_id,state,created_at,updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, tenant, user, "event-"+id, channel, "config-"+stamp, 1, "site-"+stamp, "device.fault", string(domain.LevelUrgent),
			"trace-"+id, "device", "machine-"+stamp, domain.ExternalTaskStatePending, now, now).Error; err != nil {
			t.Fatal(err)
		}
	}
	forceDue := func(id string) {
		t.Helper()
		if err := db.Exec("UPDATE biz_notification_external_tasks SET next_attempt_at=DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 SECOND) WHERE task_id=?", id).Error; err != nil {
			t.Fatal(err)
		}
	}

	t.Run("preference-deny-after-enqueue-cancels-before-provider", func(t *testing.T) {
		id := "task-deny-" + stamp
		insertTask(id, "email")
		if _, err := store.ChangeNotificationPreference(ctx, owner, accessdomain.NotificationPreferenceChange{Channel: accessdomain.NotificationPreferenceEmail, Allowed: false, ExpectedVersion: 0, IdempotencyKey: "deny-" + stamp}); err != nil {
			t.Fatal(err)
		}
		receipt, err := worker.RunOnce(ctx)
		if err != nil || receipt.TaskID != id || receipt.State != domain.ExternalTaskStateCancelled || receipt.FailureCode != "PREFERENCE_DENIED" || provider.Count() != 0 {
			t.Fatalf("receipt=%+v provider_count=%d err=%v", receipt, provider.Count(), err)
		}
	})

	t.Run("allowed-delivery-resolves-protected-contact-transiently", func(t *testing.T) {
		if _, err := store.ChangeNotificationPreference(ctx, owner, accessdomain.NotificationPreferenceChange{Channel: accessdomain.NotificationPreferenceEmail, Allowed: true, ExpectedVersion: 1, IdempotencyKey: "allow-" + stamp}); err != nil {
			t.Fatal(err)
		}
		id := "task-allow-" + stamp
		insertTask(id, "email")
		receipt, err := worker.RunOnce(ctx)
		if err != nil || receipt.TaskID != id || receipt.State != domain.ExternalTaskStateProviderAccepted || receipt.ProviderReceipt == "" {
			t.Fatalf("receipt=%+v err=%v", receipt, err)
		}
		request, ok := provider.Request(id)
		if !ok || request.Destination != email || request.TenantID != tenant || request.UserID != user {
			t.Fatalf("provider request=%+v ok=%v", request, ok)
		}
		var member struct{ Email, EmailCiphertext string }
		if err := db.Table("biz_memberships").Select("email,email_ciphertext").Where("tenant_id=? AND user_id=?", tenant, user).Take(&member).Error; err != nil {
			t.Fatal(err)
		}
		if member.Email != "" || member.EmailCiphertext == "" || strings.Contains(member.EmailCiphertext, email) {
			t.Fatalf("membership contact not protected: %+v", member)
		}
		var taskDump string
		if err := db.Raw(`SELECT CONCAT_WS('|',task_id,tenant_id,user_id,event_id,channel,configuration_id,group_id,type_code,trace_id,reference_kind,reference_id,state,failure_code,provider_receipt) FROM biz_notification_external_tasks WHERE task_id=?`, id).Scan(&taskDump).Error; err != nil {
			t.Fatal(err)
		}
		if strings.Contains(taskDump, email) || strings.Contains(taskDump, "+491701234567") {
			t.Fatal("notification task persisted a plaintext contact")
		}
	})

	t.Run("retry-rechecks-new-deny-before-second-provider-call", func(t *testing.T) {
		if _, err := store.ChangeNotificationPreference(ctx, owner, accessdomain.NotificationPreferenceChange{Channel: accessdomain.NotificationPreferenceEmail, Allowed: true, ExpectedVersion: 2, IdempotencyKey: "allow-retry-" + stamp}); err != nil {
			t.Fatal(err)
		}
		id := "task-retry-" + stamp
		insertTask(id, "email")
		provider.SetFailure(context.DeadlineExceeded)
		first, err := worker.RunOnce(ctx)
		if err != nil || first.State != domain.ExternalTaskStateRetryWait || first.Attempt != 1 {
			t.Fatalf("first=%+v err=%v", first, err)
		}
		if _, err := store.ChangeNotificationPreference(ctx, owner, accessdomain.NotificationPreferenceChange{Channel: accessdomain.NotificationPreferenceEmail, Allowed: false, ExpectedVersion: 3, IdempotencyKey: "deny-retry-" + stamp}); err != nil {
			t.Fatal(err)
		}
		forceDue(id)
		provider.SetFailure(nil)
		second, err := worker.RunOnce(ctx)
		if err != nil || second.State != domain.ExternalTaskStateCancelled || second.FailureCode != "PREFERENCE_DENIED" || second.Attempt != 2 {
			t.Fatalf("second=%+v err=%v", second, err)
		}
		if _, ok := provider.Request(id); ok {
			t.Fatal("second attempt reached provider after preference deny")
		}
	})

	t.Run("missing-sms-contact-is-explicit-manual-review", func(t *testing.T) {
		other := "n185-no-phone-" + stamp
		seedReader(t, db, tenant, other, "n185-no-phone-token-"+stamp, "", tenant+":no-phone", "no phone", "tenant.notification.read", "all")
		id := "task-no-phone-" + stamp
		now := time.Now().UTC()
		if err := db.Exec(`INSERT INTO biz_notification_external_tasks
(task_id,tenant_id,user_id,event_id,channel,configuration_id,configuration_version,group_id,type_code,level,trace_id,reference_kind,reference_id,state,created_at,updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, tenant, other, "event-"+id, "sms", "config-"+stamp, 1, "site-"+stamp, "device.fault", string(domain.LevelUrgent),
			"trace-"+id, "device", "machine-"+stamp, domain.ExternalTaskStatePending, now, now).Error; err != nil {
			t.Fatal(err)
		}
		receipt, err := worker.RunOnce(ctx)
		if err != nil || receipt.State != domain.ExternalTaskStateManualReview || receipt.FailureCode != "CONTACT_UNAVAILABLE" {
			t.Fatalf("missing contact receipt=%+v err=%v", receipt, err)
		}
	})
}
