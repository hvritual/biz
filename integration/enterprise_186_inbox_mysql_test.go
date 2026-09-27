//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	notificationapp "github.com/hvritual/biz/internal/notification/application"
	notificationdomain "github.com/hvritual/biz/internal/notification/domain"
	notificationpersistence "github.com/hvritual/biz/internal/notification/infrastructure/persistence"
	"gorm.io/gorm"
)

func TestEnterprise186InAppUnreadInboxMySQLAndHTTP(t *testing.T) {
	db := openDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := notificationpersistence.MigrateRouting(ctx, db); err != nil {
		t.Fatal(err)
	}
	runtime := startEnterprise182Runtime(t, db)
	stamp := fmt.Sprint(time.Now().UnixNano())
	userID, otherUser := "e186-u-"+stamp, "e186-other-"+stamp
	tenantA, tenantB := "e186-a-"+stamp, "e186-b-"+stamp
	email := "e186-" + stamp + "@example.invalid"
	for _, tenant := range []string{tenantA, tenantB} {
		if err := runtime.store.Bootstrap(ctx, accesspersistence.Bootstrap{
			TenantID: tenant, TenantName: tenant, UserID: userID, Email: email, Token: "e186-token-" + tenant,
		}, nil); err != nil {
			t.Fatal(err)
		}
	}
	enterprise182SeedSecondOwner(t, db, tenantA, otherUser)
	identity, err := runtime.store.ResolveOrBindOIDCIdentity(ctx, runtime.issuer.URL, "e186-sub-"+stamp, email, true)
	if err != nil {
		t.Fatal(err)
	}
	rawSession, _, err := runtime.store.CreateWebSession(ctx, identity, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	sessionA, err := runtime.store.SwitchWebSessionTenant(ctx, rawSession, tenantA)
	if err != nil {
		t.Fatal(err)
	}

	created := time.Now().UTC().Truncate(time.Microsecond)
	readAt := created.Add(time.Second)
	seed := func(tenant, user, event, kind, reference string, at time.Time, read *time.Time) string {
		t.Helper()
		messageID := notificationdomain.StableRoutingID("in_app", tenant, event, user)
		if err := db.Exec(`INSERT INTO biz_notification_in_app
			(message_id,tenant_id,user_id,event_id,configuration_id,configuration_version,group_id,type_code,level,trace_id,reference_kind,reference_id,created_at,read_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			messageID, tenant, user, event, "e186-config-"+stamp, 1, "e186-group-"+stamp,
			kind, string(notificationdomain.LevelImportant), "e186-trace-"+event, "device", reference, at, read,
		).Error; err != nil {
			t.Fatal(err)
		}
		return messageID
	}

	messageA1 := seed(tenantA, userID, "e186-a1-"+stamp, "device.fault", "machine-a1", created, nil)
	messageA2 := seed(tenantA, userID, "e186-a2-"+stamp, "device.offline", "machine-a2", created, nil)
	seed(tenantA, userID, "e186-read-"+stamp, "system.announcement", "read", created.Add(-time.Second), &readAt)
	messageB := seed(tenantB, userID, "e186-b-"+stamp, "device.fault", "machine-b", created, nil)
	seed(tenantA, otherUser, "e186-other-"+stamp, "device.fault", "machine-other", created, nil)

	base := "http://" + runtime.started.HTTPAddress()
	endpoint := "/auth/personal/in-app-notifications"
	headersA := enterprise182SessionHeaders(sessionA, "e186-mark-initial-"+stamp)

	t.Run("current-owner-only-and-stable-order", func(t *testing.T) {
		status, body := notificationInboxHTTP(t, base, http.MethodGet, endpoint, rawSession, headersA, "")
		if status != http.StatusOK {
			t.Fatalf("GET status=%d body=%s", status, body)
		}
		var snapshot notificationdomain.InboxSnapshot
		if err := json.Unmarshal(body, &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.TenantID != tenantA || snapshot.UserID != userID || snapshot.UnreadCount != 2 || len(snapshot.Messages) != 2 {
			t.Fatalf("snapshot=%+v", snapshot)
		}
		expected := []string{messageA1, messageA2}
		sort.Sort(sort.Reverse(sort.StringSlice(expected)))
		if snapshot.Messages[0].MessageID != expected[0] || snapshot.Messages[1].MessageID != expected[1] {
			t.Fatalf("unstable same-time order=%s,%s want=%v", snapshot.Messages[0].MessageID, snapshot.Messages[1].MessageID, expected)
		}
		for _, message := range snapshot.Messages {
			if message.MessageID == messageB {
				t.Fatal("cross-tenant message leaked")
			}
		}
	})

	t.Run("HTTP-session-context-CSRF-and-query-negatives", func(t *testing.T) {
		noContext := cloneHeaders186(headersA)
		delete(noContext, "X-Biz-Session-Context")
		forged := cloneHeaders186(headersA)
		forged["X-Biz-Session-Context"] = `{}`
		noCSRF := cloneHeaders186(headersA)
		delete(noCSRF, "X-CSRF-Token")
		noKey := cloneHeaders186(headersA)
		delete(noKey, "Idempotency-Key")
		cases := []struct {
			name, method, path, session, body string
			headers                           map[string]string
			want                              int
		}{
			{"no-session", http.MethodGet, endpoint, "", "", headersA, 401},
			{"no-context", http.MethodGet, endpoint, rawSession, "", noContext, 409},
			{"forged-context", http.MethodGet, endpoint, rawSession, "", forged, 409},
			{"query-tenant", http.MethodGet, endpoint + "?tenant_id=" + tenantB, rawSession, "", headersA, 400},
			{"mark-no-csrf", http.MethodPost, endpoint + "/read-all", rawSession, `{}`, noCSRF, 403},
			{"mark-no-idempotency", http.MethodPost, endpoint + "/read-all", rawSession, `{}`, noKey, 400},
			{"mark-client-selected-tenant", http.MethodPost, endpoint + "/read-all", rawSession, `{"tenant_id":"` + tenantB + `"}`, headersA, 400},
			{"mark-client-selected-user", http.MethodPost, endpoint + "/read-all", rawSession, `{"user_id":"other"}`, headersA, 400},
		}
		for _, test := range cases {
			t.Run(test.name, func(t *testing.T) {
				status, _ := notificationInboxHTTP(t, base, test.method, test.path, test.session, test.headers, test.body)
				if status != test.want {
					t.Fatalf("status=%d want=%d", status, test.want)
				}
			})
		}
	})

	var initialReceipt notificationdomain.MarkAllReadReceipt
	t.Run("mark-all-read-is-idempotent-and-readback-is-zero", func(t *testing.T) {
		status, body := notificationInboxHTTP(t, base, http.MethodPost, endpoint+"/read-all", rawSession, headersA, `{}`)
		if status != http.StatusOK || json.Unmarshal(body, &initialReceipt) != nil {
			t.Fatalf("mark status=%d body=%s", status, body)
		}
		if initialReceipt.TenantID != tenantA || initialReceipt.UserID != userID || initialReceipt.MarkedCount != 2 {
			t.Fatalf("receipt=%+v", initialReceipt)
		}
		status, replay := notificationInboxHTTP(t, base, http.MethodPost, endpoint+"/read-all", rawSession, headersA, `{}`)
		if status != http.StatusOK || !bytes.Equal(body, replay) {
			t.Fatalf("replay status=%d first=%s replay=%s", status, body, replay)
		}
		status, body = notificationInboxHTTP(t, base, http.MethodGet, endpoint, rawSession, headersA, "")
		var snapshot notificationdomain.InboxSnapshot
		if status != http.StatusOK || json.Unmarshal(body, &snapshot) != nil || snapshot.UnreadCount != 0 || len(snapshot.Messages) != 0 {
			t.Fatalf("readback status=%d snapshot=%+v body=%s", status, snapshot, body)
		}
	})

	t.Run("new-message-after-command-snapshot-survives-old-request-and-replay", func(t *testing.T) {
		oldMessage := seed(tenantA, userID, "e186-race-old-"+stamp, "device.fault", "race-old", created.Add(2*time.Second), nil)
		routing, err := notificationpersistence.NewRoutingRepository(db)
		if err != nil {
			t.Fatal(err)
		}
		service, err := notificationapp.NewInboxService(routing)
		if err != nil {
			t.Fatal(err)
		}
		owner := notificationdomain.InboxOwner{TenantID: tenantA, UserID: userID}
		captured := make(chan struct{})
		release := make(chan struct{})
		var once sync.Once
		callback := "enterprise186:pause-after-unread-snapshot"
		if err := db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
			table := tx.Statement.Table
			if table == "" && tx.Statement.Schema != nil {
				table = tx.Statement.Schema.Table
			}
			if table == "biz_notification_in_app" {
				once.Do(func() {
					close(captured)
					<-release
				})
			}
		}); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if callback != "" {
				_ = db.Callback().Query().Remove(callback)
			}
		}()

		type outcome struct {
			receipt notificationdomain.MarkAllReadReceipt
			err     error
		}
		results := make(chan outcome, 1)
		go func() {
			receipt, err := service.MarkAllRead(ctx, owner, "e186-race-key-"+stamp)
			results <- outcome{receipt: receipt, err: err}
		}()
		select {
		case <-captured:
		case <-time.After(5 * time.Second):
			t.Fatal("mark-all snapshot was not observed")
		}
		newMessage := seed(tenantA, userID, "e186-race-new-"+stamp, "device.offline", "race-new", created.Add(3*time.Second), nil)
		close(release)
		result := <-results
		if result.err != nil || result.receipt.MarkedCount != 1 {
			t.Fatalf("race receipt=%+v err=%v", result.receipt, result.err)
		}
		if err := db.Callback().Query().Remove(callback); err != nil {
			t.Fatal(err)
		}
		callback = ""

		snapshot, err := service.ReadUnread(ctx, owner)
		if err != nil || snapshot.UnreadCount != 1 || len(snapshot.Messages) != 1 || snapshot.Messages[0].MessageID != newMessage {
			t.Fatalf("new message lost: %+v / %v", snapshot, err)
		}
		if snapshot.Messages[0].MessageID == oldMessage {
			t.Fatal("old message was not marked read")
		}
		replay, err := service.MarkAllRead(ctx, owner, "e186-race-key-"+stamp)
		if err != nil || !reflect.DeepEqual(replay, result.receipt) {
			t.Fatalf("race replay=%+v want=%+v err=%v", replay, result.receipt, err)
		}
		snapshot, err = service.ReadUnread(ctx, owner)
		if err != nil || snapshot.UnreadCount != 1 || snapshot.Messages[0].MessageID != newMessage {
			t.Fatalf("replay consumed newer message: %+v / %v", snapshot, err)
		}
	})

	t.Run("prepared-snapshot-survives-interrupted-apply-and-later-message", func(t *testing.T) {
		resumeUser := "e186-resume-" + stamp
		owner := notificationdomain.InboxOwner{TenantID: tenantA, UserID: resumeUser}
		key := "e186-resume-key-" + stamp
		commandID, err := notificationdomain.InboxMarkAllCommandID(owner, key)
		if err != nil {
			t.Fatal(err)
		}
		oldMessage := seed(tenantA, resumeUser, "e186-resume-old-"+stamp, "device.fault", "resume-old", created.Add(4*time.Second), nil)
		preparedAt := created.Add(5 * time.Second)
		if err := db.Exec(`INSERT INTO biz_notification_inbox_mark_all
			(command_id,tenant_id,user_id,state,marked_count,read_at,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?)`,
			commandID, tenantA, resumeUser, "prepared", 0, preparedAt, preparedAt, preparedAt,
		).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(`INSERT INTO biz_notification_inbox_mark_all_items (command_id,message_id) VALUES (?,?)`, commandID, oldMessage).Error; err != nil {
			t.Fatal(err)
		}
		newMessage := seed(tenantA, resumeUser, "e186-resume-new-"+stamp, "device.offline", "resume-new", created.Add(6*time.Second), nil)

		routing, err := notificationpersistence.NewRoutingRepository(db)
		if err != nil {
			t.Fatal(err)
		}
		service, err := notificationapp.NewInboxService(routing)
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := service.MarkAllRead(ctx, owner, key)
		if err != nil || receipt.ReceiptID != commandID || receipt.MarkedCount != 1 {
			t.Fatalf("resume receipt=%+v err=%v", receipt, err)
		}
		snapshot, err := service.ReadUnread(ctx, owner)
		if err != nil || snapshot.UnreadCount != 1 || len(snapshot.Messages) != 1 || snapshot.Messages[0].MessageID != newMessage {
			t.Fatalf("prepared boundary expanded on retry: %+v / %v", snapshot, err)
		}
		var remaining int64
		if err := db.Table("biz_notification_inbox_mark_all_items").Where("command_id=?", commandID).Count(&remaining).Error; err != nil {
			t.Fatal(err)
		}
		if remaining != 0 {
			t.Fatalf("applied snapshot items were not compacted: %d", remaining)
		}
	})

	t.Run("tenant-switch-rejects-old-context-and-reads-new-owner", func(t *testing.T) {
		sessionB, err := runtime.store.SwitchWebSessionTenant(ctx, rawSession, tenantB)
		if err != nil {
			t.Fatal(err)
		}
		status, _ := notificationInboxHTTP(t, base, http.MethodGet, endpoint, rawSession, headersA, "")
		if status != http.StatusConflict {
			t.Fatalf("old context status=%d", status)
		}
		status, body := notificationInboxHTTP(t, base, http.MethodGet, endpoint, rawSession, enterprise182SessionHeaders(sessionB, "unused"), "")
		var snapshot notificationdomain.InboxSnapshot
		if status != http.StatusOK || json.Unmarshal(body, &snapshot) != nil || snapshot.TenantID != tenantB || snapshot.UserID != userID || snapshot.UnreadCount != 1 || snapshot.Messages[0].MessageID != messageB {
			t.Fatalf("tenant B status=%d snapshot=%+v body=%s", status, snapshot, body)
		}
	})
}

func cloneHeaders186(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func notificationInboxHTTP(t *testing.T, base, method, path, rawSession string, headers map[string]string, body string) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if rawSession != "" {
		request.AddCookie(&http.Cookie{Name: "biz_session", Value: rawSession})
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	result, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("cacheable inbox response: %q", response.Header.Get("Cache-Control"))
	}
	return response.StatusCode, result
}
