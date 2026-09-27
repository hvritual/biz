//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	notificationv1 "github.com/hvritual/biz/contracts/gen/notification/v1"
	ap "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	dp "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"yunka.io/gateway/authz"
)

// Real generated REST/gRPC -> unified Executor -> current IAM -> borrowed root
// transaction -> MySQL. No HTTP-success mocks or alternate application server.
func TestEnterprise184NotificationConfigurationRealRuntime(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenant, other, site := "nc184-a-"+stamp, "nc184-b-"+stamp, "nc184-site-"+stamp
	token, otherToken := "nc184-token-"+stamp, "nc184-other-token-"+stamp
	user, otherUser := tenant+"-owner", other+"-owner"
	module := startModule(t, db, tenant, token, site)
	base := "http://" + module.HTTPAddress() + "/v1/tenant/notification"
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	store, err := ap.New(db)
	must(err)
	permissions := []authz.PermissionKey{"tenant.notification.read", "tenant.notification.create", "tenant.notification.update", "tenant.notification.delete"}
	for _, fixture := range []struct{ tenant, user, token string }{{tenant, user, token}, {other, otherUser, otherToken}} {
		must(store.Bootstrap(ctx, ap.Bootstrap{TenantID: fixture.tenant, TenantName: fixture.tenant, UserID: fixture.user, Email: fixture.user + "@example.invalid", Token: fixture.token}, permissions))
	}
	ce05LegacyFixtureGrants(t, db, other, []string{"access-management"})
	must(db.Exec("INSERT IGNORE INTO biz_member_sites (tenant_id,user_id,site_id) VALUES (?,?,?)", tenant, user, site).Error)
	otherSite := "nc184-foreign-" + stamp
	now := time.Now().UTC()
	must(db.Create(&dp.SitePORecord{SitePO: dp.SitePO{Name: "Other business site"}, SitePOBase: dp.SitePOBase{ID: otherSite, TenantID: other, Version: 1, CreatedAt: now, UpdatedAt: now}}).Error)
	must(db.Exec("INSERT IGNORE INTO biz_member_sites (tenant_id,user_id,site_id) VALUES (?,?,?)", other, otherUser, otherSite).Error)
	reader, readerToken, secondary, additional := "nc184-reader-"+stamp, "nc184-reader-token-"+stamp, "nc184-second-"+stamp, "nc184-extra-"+stamp
	seedReader(t, db, tenant, reader, readerToken, site, tenant+":reader", "Notification reader", "tenant.notification.read", "sites")
	seedReader(t, db, tenant, secondary, "nc184-second-token-"+stamp, "", tenant+":secondary", "Notification contact", "tenant.notification.read", "self")
	seedReader(t, db, tenant, additional, "nc184-extra-token-"+stamp, "", tenant+":extra", "Notification contact extra", "tenant.notification.read", "self")
	must(db.Exec("UPDATE biz_memberships SET name=? WHERE tenant_id=? AND user_id=?", "Primary Operator", tenant, user).Error)
	must(db.Exec("UPDATE biz_memberships SET name=? WHERE tenant_id=? AND user_id=?", "Second Operator", tenant, secondary).Error)
	must(db.Exec("UPDATE biz_memberships SET name=? WHERE tenant_id=? AND user_id=?", "Extra Operator", tenant, additional).Error)
	httpClient := &http.Client{Timeout: 10 * time.Second}
	call := func(method, path, credential, key string, body any) (int, []byte) {
		t.Helper()
		var payload []byte
		if body != nil {
			var err error
			payload, err = json.Marshal(body)
			must(err)
		}
		request, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(payload))
		must(err)
		if credential != "" {
			request.Header.Set("Authorization", "Bearer "+credential)
		}
		if key != "" {
			request.Header.Set("Idempotency-Key", key)
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := httpClient.Do(request)
		must(err)
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		must(err)
		return response.StatusCode, data
	}
	expect := func(method, path, credential, key string, body any, want int, out proto.Message) []byte {
		t.Helper()
		code, data := call(method, path, credential, key, body)
		if code != want {
			t.Fatalf("%s %s want=%d got=%d body=%s", method, path, want, code, data)
		}
		if out != nil {
			must(protojson.Unmarshal(data, out))
		}
		return data
	}
	input := func(group, owner string, levels ...string) map[string]any {
		return map[string]any{"groupId": group, "levels": levels, "channels": []string{"in_app"}, "primaryUserId": owner, "notes": "Runtime qualification"}
	}
	var created, edited, deleted notificationv1.MessageConfigurationReceipt
	var id string
	createBody := input(site, user, "urgent", "general")
	createBody["secondaryUserId"] = secondary
	createBody["additionalUserIds"] = []string{additional}
	t.Run("metadata-and-directory-are-real-tenant-scoped", func(t *testing.T) {
		var types notificationv1.ListMessageTypesResponse
		expect("GET", "/types?level=important&page_size=1", token, "", nil, 200, &types)
		if types.TenantId != tenant || types.Total != 2 || len(types.Items) != 1 || len(types.Groups) != 3 {
			t.Fatalf("type page=%v", &types)
		}
		var channels notificationv1.ListMessageChannelsResponse
		expect("GET", "/channels", token, "", nil, 200, &channels)
		if len(channels.Items) != 3 {
			t.Fatalf("channels=%v", &channels)
		}
		for _, c := range channels.Items {
			if c.Code != "in_app" && (c.Configurable || c.UnavailableReason == "") {
				t.Fatalf("unconfigured transport selectable: %v", c)
			}
		}
		var groups notificationv1.ListMessageDirectoryResponse
		expect("GET", "/groups", token, "", nil, 200, &groups)
		if groups.Total != 1 || len(groups.Items) != 1 || groups.Items[0].Id != site {
			t.Fatalf("group leak/missing: %v", &groups)
		}
		var contacts notificationv1.ListMessageDirectoryResponse
		raw := expect("GET", "/recipients?query="+url.QueryEscape("Operator"), token, "", nil, 200, &contacts)
		if contacts.Total != 3 || strings.Contains(string(raw), "@example.invalid") || strings.Contains(string(raw), "phone") {
			t.Fatalf("recipient projection=%s", raw)
		}
	})
	t.Run("missing-auth-and-independent-write-permissions", func(t *testing.T) {
		expect("GET", "/configurations", "", "", nil, 401, nil)
		expect("POST", "/configurations", readerToken, "nc-readonly-create", createBody, 403, nil)
		code, _ := call("POST", "/configurations", token, "", createBody)
		if code < 400 || code >= 500 {
			t.Fatalf("missing idempotency key status=%d", code)
		}
	})
	t.Run("invalid-channel-contact-group-and-identity-injection", func(t *testing.T) {
		tests := []struct {
			name string
			body map[string]any
			want int
		}{
			{"foreign-group", input(otherSite, user, "important"), 404},
			{"foreign-contact", input(site, otherUser, "important"), 400},
			{"unknown-channel", input(site, user, "important"), 412},
			{"disabled-channel", input(site, user, "important"), 412},
			{"duplicate-contact", input(site, user, "important"), 400},
			{"tenant-injection", input(site, user, "important"), 400},
		}
		tests[2].body["channels"] = []string{"unknown"}
		tests[3].body["channels"] = []string{"sms"}
		tests[4].body["secondaryUserId"] = user
		tests[5].body["tenantId"] = other
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				expect("POST", "/configurations", token, "nc-invalid-"+tc.name, tc.body, tc.want, nil)
			})
		}
		var n int64
		must(db.Table("biz_notification_configurations").Where("tenant_id=?", tenant).Count(&n).Error)
		if n != 0 {
			t.Fatalf("invalid request persisted %d", n)
		}
	})
	t.Run("multi-level-create-receipt-readback-and-atomic-audit", func(t *testing.T) {
		expect("POST", "/configurations", token, "nc-create", createBody, 200, &created)
		if created.ReceiptId == "" || created.TenantId != tenant || len(created.Configurations) != 2 {
			t.Fatalf("receipt=%v", &created)
		}
		id = created.Configurations[0].Id
		for _, c := range created.Configurations {
			var got notificationv1.MessageConfigurationDTO
			expect("GET", "/configurations/"+c.Id, token, "", nil, 200, &got)
			if got.TenantId != tenant || got.GroupId != site || got.Version != 1 || got.GroupName == "" || len(got.AdditionalUserIds) != 1 || got.AdditionalUserIds[0] != additional {
				t.Fatalf("readback=%v", &got)
			}
		}
		var n int64
		must(db.Table("biz_audit_events").Where("tenant_id=? AND receipt_ref=?", tenant, created.ReceiptId).Count(&n).Error)
		if n != 2 {
			t.Fatalf("transactional audit count=%d", n)
		}
	})
	t.Run("duplicate-batch-has-no-partial-configuration", func(t *testing.T) {
		expect("POST", "/configurations", token, "nc-duplicate", input(site, user, "important", "urgent"), 409, nil)
		var rows notificationv1.ListMessageConfigurationsResponse
		expect("GET", "/configurations?level=important", token, "", nil, 200, &rows)
		if rows.Total != 0 {
			t.Fatalf("partial batch: %v", &rows)
		}
	})
	t.Run("matching-filter-pagination-total-and-B-isolation", func(t *testing.T) {
		for page := 1; page <= 3; page++ {
			var rows notificationv1.ListMessageConfigurationsResponse
			expect("GET", fmt.Sprintf("/configurations?recipient_id=%s&page=%d&page_size=1", url.QueryEscape(additional), page), token, "", nil, 200, &rows)
			want := 1
			if page == 3 {
				want = 0
			}
			if rows.Total != 2 || len(rows.Items) != want {
				t.Fatalf("page=%d response=%v", page, &rows)
			}
		}
		var otherReceipt notificationv1.MessageConfigurationReceipt
		expect("POST", "/configurations", otherToken, "nc-B-create", input(otherSite, otherUser, "urgent"), 200, &otherReceipt)
		expect("GET", "/configurations/"+id, otherToken, "", nil, 404, nil)
		expect("PATCH", "/configurations/"+id, otherToken, "nc-B-update", map[string]any{"expectedVersion": "1", "channels": []string{"in_app"}, "primaryUserId": otherUser}, 404, nil)
		expect("POST", "/configurations/"+id+"/delete", otherToken, "nc-B-delete", map[string]any{"expectedVersion": "1"}, 404, nil)
		expect("GET", "/configurations/"+otherReceipt.Configurations[0].Id, token, "", nil, 404, nil)
	})
	editBody := map[string]any{"expectedVersion": "1", "channels": []string{"in_app"}, "primaryUserId": user, "additionalUserIds": []string{reader}, "notes": "Updated current rule"}
	t.Run("update-replaces-relations-and-stale-CAS-rejects", func(t *testing.T) {
		expect("PATCH", "/configurations/"+id, readerToken, "nc-reader-update", editBody, 403, nil)
		expect("POST", "/configurations/"+id+"/delete", readerToken, "nc-reader-delete", map[string]any{"expectedVersion": "1"}, 403, nil)
		expect("PATCH", "/configurations/"+id, token, "nc-update", editBody, 200, &edited)
		var got notificationv1.MessageConfigurationDTO
		expect("GET", "/configurations/"+id, token, "", nil, 200, &got)
		if got.Version != 2 || got.SecondaryUserId != "" || !reflect.DeepEqual(got.AdditionalUserIds, []string{reader}) || got.Notes != "Updated current rule" {
			t.Fatalf("replacement=%v", &got)
		}
		expect("PATCH", "/configurations/"+id, token, "nc-stale", editBody, 409, nil)
	})
	t.Run("persistent-replay-does-not-revert-newer-state", func(t *testing.T) {
		var replay notificationv1.MessageConfigurationReceipt
		expect("POST", "/configurations", token, "nc-create", createBody, 200, &replay)
		if !proto.Equal(&created, &replay) {
			t.Fatalf("original receipt changed: %v", &replay)
		}
		expect("PATCH", "/configurations/"+id, token, "nc-update", editBody, 200, &replay)
		if !proto.Equal(&edited, &replay) {
			t.Fatalf("update receipt changed: %v", &replay)
		}
		changed := input(site, user, "urgent", "general")
		changed["notes"] = "different"
		expect("POST", "/configurations", token, "nc-create", changed, 409, nil)
		var got notificationv1.MessageConfigurationDTO
		expect("GET", "/configurations/"+id, token, "", nil, 200, &got)
		if got.Version != 2 {
			t.Fatalf("replay overwrote version=%d", got.Version)
		}
	})
	t.Run("gRPC-shares-generated-read-and-write-permissions", func(t *testing.T) {
		conn, err := grpc.DialContext(ctx, module.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		must(err)
		defer conn.Close()
		client := notificationv1.NewMessageConfigurationApplicationClient(conn)
		callContext := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+readerToken)
		response, err := client.ListMessageConfigurations(callContext, &notificationv1.ListMessageConfigurationsRequest{})
		must(err)
		if response.TenantId != tenant || response.Total != 2 {
			t.Fatalf("gRPC read=%v", response)
		}
		_, err = client.DeleteMessageConfiguration(metadata.AppendToOutgoingContext(callContext, "idempotency-key", "nc-grpc-deny"), &notificationv1.DeleteMessageConfigurationRequest{Id: id, ExpectedVersion: 2})
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("gRPC delete should deny, got %v", err)
		}
	})
	t.Run("current-member-and-scope-revocation-apply-next-request", func(t *testing.T) {
		must(db.Exec("DELETE FROM biz_member_sites WHERE tenant_id=? AND user_id=?", tenant, reader).Error)
		var rows notificationv1.ListMessageConfigurationsResponse
		expect("GET", "/configurations", readerToken, "", nil, 200, &rows)
		if rows.Total != 0 {
			t.Fatalf("revoked scope retained: %v", &rows)
		}
		expect("GET", "/configurations/"+id, readerToken, "", nil, 404, nil)
		must(db.Exec("UPDATE biz_memberships SET status='disabled' WHERE tenant_id=? AND user_id=?", tenant, additional).Error)
		body := input(site, user, "important")
		body["additionalUserIds"] = []string{additional}
		expect("POST", "/configurations", token, "nc-disabled-recipient", body, 400, nil)
	})
	t.Run("delete-with-version-cleans-relations-and-replays-original-receipt", func(t *testing.T) {
		expect("POST", "/configurations/"+id+"/delete", token, "nc-delete-stale", map[string]any{"expectedVersion": "1"}, 409, nil)
		expect("POST", "/configurations/"+id+"/delete", token, "nc-delete", map[string]any{"expectedVersion": "2"}, 200, &deleted)
		if len(deleted.Configurations) != 1 || !deleted.Configurations[0].Deleted {
			t.Fatalf("delete receipt=%v", &deleted)
		}
		expect("GET", "/configurations/"+id, token, "", nil, 404, nil)
		for _, table := range []string{"biz_notification_configuration_channels", "biz_notification_configuration_recipients"} {
			var n int64
			must(db.Table(table).Where("tenant_id=? AND configuration_id=?", tenant, id).Count(&n).Error)
			if n != 0 {
				t.Fatalf("orphan %s=%d", table, n)
			}
		}
		var replay notificationv1.MessageConfigurationReceipt
		expect("POST", "/configurations/"+id+"/delete", token, "nc-delete", map[string]any{"expectedVersion": "2"}, 200, &replay)
		if !proto.Equal(&deleted, &replay) {
			t.Fatal("delete replay differs")
		}
	})
	t.Run("action-grant-revocation-denies-next-write", func(t *testing.T) {
		must(db.Exec("DELETE FROM biz_permission_grants WHERE tenant_id=? AND permission=?", tenant, "tenant.notification.create").Error)
		expect("POST", "/configurations", token, "nc-revoked-action", input(site, user, "important"), 403, nil)
	})
}
