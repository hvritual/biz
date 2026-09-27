//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/bizruntime"
	notificationdomain "github.com/hvritual/biz/internal/notification/domain"
	notificationpersistence "github.com/hvritual/biz/internal/notification/infrastructure/persistence"
	"github.com/hvritual/biz/modules/deviceops"
	"gorm.io/gorm"
	"yunka.io/framework/platform"
	"yunka.io/gateway/authz"
	"yunka.io/pkg/logExt"
)

func TestEnterprise185NotificationRuntimeComponentRoutesDeliversAndCallbacks(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	stamp := fmt.Sprint(time.Now().UnixNano())
	tenant, user, site := "n185-runtime-"+stamp, "n185-owner-"+stamp, "n185-site-"+stamp
	token := "n185-runtime-token-" + stamp
	email := "runtime-" + stamp + "@example.invalid"
	callbackSecret := []byte(strings.Repeat("H", 32))

	var providerMu sync.Mutex
	var providerCalls int
	var providerPayload map[string]string
	var providerIdempotency, providerAuthorization string
	providerServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		providerMu.Lock()
		defer providerMu.Unlock()
		providerCalls++
		providerIdempotency = request.Header.Get("Idempotency-Key")
		providerAuthorization = request.Header.Get("Authorization")
		if err := json.NewDecoder(request.Body).Decode(&providerPayload); err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte("{"receipt_id":"runtime-provider-receipt","status":"accepted"}"))
	}))
	defer providerServer.Close()

	config := deviceops.DefaultConfig()
	config.HTTPListenAddress = "127.0.0.1:0"
	config.GRPCListenAddress = "127.0.0.1:0"
	config.AutoMigrate = true
	config.Bootstrap = deviceops.BootstrapConfig{
		TenantID: tenant, TenantName: tenant, UserID: user, Email: email,
		Token: token, SiteID: site, SiteName: site,
	}
	protection, err := accesspersistence.NewContactProtection(accesspersistence.ContactProtectionConfig{
		ActiveVersion: "v1",
		Keys:          map[string][]byte{"v1": []byte(strings.Repeat("C", 32))},
		LookupKey:     []byte(strings.Repeat("L", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	platformProvider, err := platform.New(platform.Options{
		Config: bizruntime.ConfigProvider{DeviceOps: config},
		Logger: logExt.NewBaseLogger(),
		Databases: map[string]platform.DatabaseFactory{
			"primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
				return platform.BorrowedDatabase(db), nil
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	started, err := bizruntime.BootstrapWithOptionsAndContactProtection(runtimeCtx, platformProvider, bizruntime.Options{
		DeviceOps: config,
		NotificationRuntime: bizruntime.NotificationRuntimeOptions{
			ProviderEndpoint: providerServer.URL, ProviderBearerToken: "runtime-provider-token",
			ProviderIdempotent: true, Channels: []string{"email"}, CallbackHMACSecret: callbackSecret,
			PollInterval: 10 * time.Millisecond, RoutingLeaseDuration: 5 * time.Second,
		},
	}, protection)
	if err != nil {
		runtimeCancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		runtimeCancel()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = started.App.Shutdown(shutdown)
	})
	ce05LegacyFixtureGrants(t, db, tenant, []string{"access-management", "device-operations"})

	store, err := accesspersistence.NewWithContactProtection(db, protection)
	if err != nil {
		t.Fatal(err)
	}
	permissions := []authz.PermissionKey{
		"tenant.notification.read", "tenant.notification.create", "tenant.notification.update", "tenant.notification.delete",
	}
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenant, TenantName: tenant, UserID: user, Email: email, Token: token,
	}, permissions); err != nil {
		t.Fatal(err)
	}
	enterprise182SetTenantContacts(t, db, protection, tenant, user, email, "+491701234567")

	base := "http://" + started.HTTPAddress()
	t.Run("configured-provider-makes-only-enabled-channel-selectable", func(t *testing.T) {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/tenant/notification/channels", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("channel directory status=%d", response.StatusCode)
		}
		var payload struct {
			Items []struct {
				Code         string `json:"code"`
				Configurable bool   `json:"configurable"`
			} `json:"items"`
		}
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		state := map[string]bool{}
		for _, item := range payload.Items {
			state[item.Code] = item.Configurable
		}
		if !state["in_app"] || !state["email"] || state["sms"] {
			t.Fatalf("channel availability=%v", state)
		}
	})

	t.Run("configuration-api-and-runtime-share-the-same-channel-catalog", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"groupId": site, "levels": []string{"urgent"}, "channels": []string{"in_app", "email"},
			"primaryUserId": user, "notes": "runtime component qualification",
		})
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/tenant/notification/configurations", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Idempotency-Key", "runtime-config-"+stamp)
		request.Header.Set("Content-Type", "application/json")
		response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("configuration status=%d", response.StatusCode)
		}
	})

	event := notificationdomain.BusinessEvent{
		EventID: "runtime-event-" + stamp, TenantID: tenant, GroupID: site,
		TypeCode: "device.fault", Level: notificationdomain.LevelUrgent,
		TraceID: "runtime-trace-" + stamp, ReferenceKind: "device", ReferenceID: "machine-" + stamp,
		OccurredAt: time.Now().UTC(),
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		publisher, err := notificationpersistence.NewEventPublisher(tx)
		if err != nil {
			return err
		}
		return publisher.AppendBusinessEvent(ctx, event)
	}); err != nil {
		t.Fatal(err)
	}

	var taskID, state, receipt string
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var row struct {
			TaskID, State, ProviderReceipt string
		}
		err := db.Table("biz_notification_external_tasks").
			Select("task_id,state,provider_receipt").Where("event_id=?", event.EventID).Take(&row).Error
		if err == nil && row.State == notificationdomain.ExternalTaskStateProviderAccepted {
			taskID, state, receipt = row.TaskID, row.State, row.ProviderReceipt
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if state != notificationdomain.ExternalTaskStateProviderAccepted || taskID == "" || receipt != "runtime-provider-receipt" {
		t.Fatalf("automatic runtime task=%s state=%s receipt=%s", taskID, state, receipt)
	}
	var inAppCount int64
	if err := db.Table("biz_notification_in_app").Where("event_id=? AND tenant_id=? AND user_id=?", event.EventID, tenant, user).Count(&inAppCount).Error; err != nil {
		t.Fatal(err)
	}
	if inAppCount != 1 {
		t.Fatalf("automatic runtime in-app count=%d", inAppCount)
	}

	providerMu.Lock()
	calls, idempotency, authorization := providerCalls, providerIdempotency, providerAuthorization
	payload := providerPayload
	providerMu.Unlock()
	if calls != 1 || idempotency != taskID || authorization != "Bearer runtime-provider-token" ||
		payload["task_id"] != taskID || payload["destination"] != email {
		t.Fatalf("provider calls=%d idempotency=%s auth=%s payload=%v", calls, idempotency, authorization, payload)
	}
	if _, ok := payload["tenant_id"]; ok {
		t.Fatal("runtime provider leaked tenant id")
	}
	if _, ok := payload["user_id"]; ok {
		t.Fatal("runtime provider leaked user id")
	}

	callbackBody, err := json.Marshal(map[string]string{
		"task_id": taskID, "receipt_id": receipt, "status": "delivered", "failure_code": "",
	})
	if err != nil {
		t.Fatal(err)
	}
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	mac := hmac.New(sha256.New, callbackSecret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(callbackBody)
	callbackRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/callbacks/notification/provider", bytes.NewReader(callbackBody))
	if err != nil {
		t.Fatal(err)
	}
	callbackRequest.Header.Set("Content-Type", "application/json")
	callbackRequest.Header.Set("X-Yunka-Notification-Timestamp", timestamp)
	callbackRequest.Header.Set("X-Yunka-Notification-Signature", "v1="+hex.EncodeToString(mac.Sum(nil)))
	callbackResponse, err := (&http.Client{Timeout: 5 * time.Second}).Do(callbackRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer callbackResponse.Body.Close()
	if callbackResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("callback status=%d", callbackResponse.StatusCode)
	}
	if err := db.Table("biz_notification_external_tasks").Select("state").Where("task_id=?", taskID).Scan(&state).Error; err != nil {
		t.Fatal(err)
	}
	if state != notificationdomain.ExternalTaskStateDelivered {
		t.Fatalf("callback terminal state=%s", state)
	}

	healthResponse, err := (&http.Client{Timeout: 5 * time.Second}).Get(base + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer healthResponse.Body.Close()
	if healthResponse.StatusCode != http.StatusOK {
		t.Fatalf("runtime health status=%d", healthResponse.StatusCode)
	}
}
