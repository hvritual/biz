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
	"testing"
	"time"

	"github.com/hvritual/biz/internal/notification/domain"
	notificationdelivery "github.com/hvritual/biz/internal/notification/infrastructure/delivery"
	notificationpersistence "github.com/hvritual/biz/internal/notification/infrastructure/persistence"
)

func TestEnterprise185HTTPProviderCallbackPersistsExactTerminalState(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := notificationpersistence.MigrateRouting(ctx, db); err != nil {
		t.Fatal(err)
	}
	repository, err := notificationpersistence.NewRoutingRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte(strings.Repeat("H", 32))
	handler, err := notificationdelivery.NewHTTPProviderCallbackHandler(repository, notificationdelivery.HTTPProviderCallbackConfig{
		Secret: secret, MaxClockSkew: 5 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()

	stamp := fmt.Sprint(time.Now().UnixNano())
	insertAccepted := func(taskID, eventID, receiptID string) {
		t.Helper()
		now := time.Now().UTC()
		if err := db.Exec(`INSERT INTO biz_notification_external_tasks
(task_id,tenant_id,user_id,event_id,channel,configuration_id,configuration_version,group_id,type_code,level,trace_id,reference_kind,reference_id,state,provider_receipt,accepted_at,created_at,updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			taskID, "callback-tenant-"+stamp, "callback-user-"+taskID, eventID, "email", "callback-config-"+stamp, 1,
			"callback-site-"+stamp, "device.fault", string(domain.LevelUrgent), "callback-trace-"+taskID, "device",
			"callback-machine-"+taskID, domain.ExternalTaskStateProviderAccepted, receiptID, now, now, now).Error; err != nil {
			t.Fatal(err)
		}
	}
	sendCallback := func(signingKey []byte, payload map[string]string) int {
		t.Helper()
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
		mac := hmac.New(sha256.New, signingKey)
		_, _ = mac.Write([]byte(timestamp))
		_, _ = mac.Write([]byte("."))
		_, _ = mac.Write(body)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Yunka-Notification-Timestamp", timestamp)
		request.Header.Set("X-Yunka-Notification-Signature", "v1="+hex.EncodeToString(mac.Sum(nil)))
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}
	readState := func(taskID string) (string, string, *time.Time) {
		t.Helper()
		var row struct {
			State, FailureCode string
			DeliveredAt        *time.Time
		}
		if err := db.Table("biz_notification_external_tasks").
			Select("state,failure_code,delivered_at").Where("task_id=?", taskID).Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		return row.State, row.FailureCode, row.DeliveredAt
	}

	t.Run("signed-delivered-callback-is-exact-and-idempotent", func(t *testing.T) {
		taskID := "callback-delivered-" + stamp
		receiptID := "provider-delivered-" + stamp
		insertAccepted(taskID, "callback-event-delivered-"+stamp, receiptID)
		payload := map[string]string{
			"task_id": taskID, "receipt_id": receiptID, "status": "delivered", "failure_code": "",
		}
		if status := sendCallback([]byte(strings.Repeat("X", 32)), payload); status != http.StatusUnauthorized {
			t.Fatalf("forged callback status=%d", status)
		}
		state, code, deliveredAt := readState(taskID)
		if state != domain.ExternalTaskStateProviderAccepted || code != "" || deliveredAt != nil {
			t.Fatalf("forged callback changed state=%s code=%s delivered=%v", state, code, deliveredAt)
		}
		if status := sendCallback(secret, payload); status != http.StatusNoContent {
			t.Fatalf("valid callback status=%d", status)
		}
		if status := sendCallback(secret, payload); status != http.StatusNoContent {
			t.Fatalf("replayed callback status=%d", status)
		}
		state, code, deliveredAt = readState(taskID)
		if state != domain.ExternalTaskStateDelivered || code != "" || deliveredAt == nil {
			t.Fatalf("delivered state=%s code=%s delivered=%v", state, code, deliveredAt)
		}
		conflict := map[string]string{
			"task_id": taskID, "receipt_id": "wrong-receipt-" + stamp, "status": "delivered", "failure_code": "",
		}
		if status := sendCallback(secret, conflict); status != http.StatusConflict {
			t.Fatalf("conflicting receipt status=%d", status)
		}
		state, _, _ = readState(taskID)
		if state != domain.ExternalTaskStateDelivered {
			t.Fatalf("conflicting callback changed terminal state=%s", state)
		}
	})

	t.Run("signed-provider-failure-enters-manual-review", func(t *testing.T) {
		taskID := "callback-failed-" + stamp
		receiptID := "provider-failed-" + stamp
		insertAccepted(taskID, "callback-event-failed-"+stamp, receiptID)
		payload := map[string]string{
			"task_id": taskID, "receipt_id": receiptID, "status": "failed", "failure_code": "PROVIDER_BOUNCED",
		}
		if status := sendCallback(secret, payload); status != http.StatusNoContent {
			t.Fatalf("failed callback status=%d", status)
		}
		if status := sendCallback(secret, payload); status != http.StatusNoContent {
			t.Fatalf("replayed failed callback status=%d", status)
		}
		state, code, deliveredAt := readState(taskID)
		if state != domain.ExternalTaskStateManualReview || code != "PROVIDER_BOUNCED" || deliveredAt != nil {
			t.Fatalf("failed state=%s code=%s delivered=%v", state, code, deliveredAt)
		}
		conflict := map[string]string{
			"task_id": taskID, "receipt_id": receiptID, "status": "failed", "failure_code": "PROVIDER_REJECTED",
		}
		if status := sendCallback(secret, conflict); status != http.StatusConflict {
			t.Fatalf("conflicting terminal failure status=%d", status)
		}
	})
}
