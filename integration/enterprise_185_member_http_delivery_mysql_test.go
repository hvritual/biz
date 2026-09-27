//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

func TestEnterprise185MemberHTTPActionsPublishOnlyLifecycleEvents(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	started, protection := startB123Enterprise176Runtime(t, db)
	base := "http://" + started.HTTPAddress()
	f := newEnterprise185HTTPDelivery(t, db, protection)
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenantA, tenantB := "e185-ma-"+stamp, "e185-mb-"+stamp
	admin, tokenA, tokenB := "e185-admin-"+stamp, "e185-token-a-"+stamp, "e185-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, admin, admin+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, "e185-b-admin-"+stamp, "e185-b-"+stamp+"@example.invalid", tokenB)
	if err := db.Exec("INSERT INTO biz_permission_grants(tenant_id,role_id,permission,scope) VALUES(?,?,?,?)", tenantA, tenantA+":member-admin", "tenant.role.manage", "all").Error; err != nil {
		t.Fatal(err)
	}
	user := "e185-member-" + stamp
	email := user + "@example.invalid"
	seedReader(t, db, tenantA, user, "e185-reader-"+stamp, "", tenantA+":operator", "operator", "tenant.member.read", "all")
	if err := db.Table("biz_users").Where("id=?", user).Update("email", email).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_memberships").Where("tenant_id=? AND user_id=?", tenantA, user).Updates(map[string]any{"email": email, "version": 1}).Error; err != nil {
		t.Fatal(err)
	}
	// The same global user also exists in B; operating A must not affect B.
	if err := db.Exec("INSERT INTO biz_memberships(tenant_id,user_id,status,version,created_at,updated_at) VALUES(?,?,?,1,NOW(6),NOW(6))", tenantB, user, accessdomain.TenantMemberStatusActive).Error; err != nil {
		t.Fatal(err)
	}

	if !t.Run("profile-update-is-not-a-security-lifecycle-event", func(t *testing.T) {
		payload, err := protojson.Marshal(&accessv1.UpdateTenantMemberProfileRequest{UserId: user, Version: 1, Name: "Profile corrected"})
		if err != nil {
			t.Fatal(err)
		}
		request, err := http.NewRequest(http.MethodPatch, base+"/v1/tenant/members/"+url.PathEscape(user)+"/profile", bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+tokenA)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", "profile-"+stamp)
		response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("profile HTTP status=%d", response.StatusCode)
		}
		if f.count(t) != 0 || f.sender.Count() != 0 {
			t.Fatal("ordinary profile edit emitted a security lifecycle notification")
		}
		current, code, _ := getB123HTTP(t, base, tokenA, user)
		if code != http.StatusOK || current.GetVersion() != 2 {
			t.Fatal("profile update was not persisted")
		}
	}) {
		return
	}

	if !t.Run("failed-outbox-insert-rolls-back-real-suspend-action", func(t *testing.T) {
		const callback = "enterprise185:reject-lifecycle-outbox"
		if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
			if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "biz_security_notification_outbox" {
				tx.AddError(errors.New("injected outbox insert failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = db.Callback().Create().Remove(callback) })
		code, _ := enterprise177Post(t, base, tokenA, "reject-suspend-"+stamp, "/v1/tenant/members/"+url.PathEscape(user)+"/suspend", &accessv1.SuspendTenantMemberRequest{UserId: user, Version: 2, Reason: "not committed"})
		if code == http.StatusOK {
			t.Fatal("suspend succeeded despite outbox insertion failure")
		}
		current, status, _ := getB123HTTP(t, base, tokenA, user)
		if status != http.StatusOK || current.GetVersion() != 2 || current.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_ACTIVE || f.count(t) != 0 {
			t.Fatal("failed suspend left partial member/outbox state")
		}
	}) {
		return
	}

	steps := []struct {
		name, path, status string
		version            uint64
		input              proto.Message
	}{
		{"suspend", "suspend", accessdomain.TenantMemberStatusSuspended, 3, &accessv1.SuspendTenantMemberRequest{UserId: user, Version: 2, Reason: "approved suspension"}},
		{"activate", "activate", accessdomain.TenantMemberStatusActive, 4, &accessv1.ActivateTenantMemberRequest{UserId: user, Version: 3, Reason: "approved activation"}},
		{"remove", "remove", accessdomain.TenantMemberStatusRemoved, 5, &accessv1.RemoveTenantMemberRequest{UserId: user, Version: 4, Reason: "approved removal"}},
		{"restore", "restore", accessdomain.TenantMemberStatusActive, 6, &accessv1.RestoreTenantMemberRequest{UserId: user, Version: 5, Reason: "approved restoration"}},
	}
	for _, step := range steps {
		if !t.Run(step.name+"-http-outbox-worker", func(t *testing.T) {
			before := f.count(t)
			key := "e185-" + step.name + "-" + stamp
			path := "/v1/tenant/members/" + url.PathEscape(user) + "/" + step.path
			code, body := enterprise177Post(t, base, tokenA, key, path, step.input)
			if code != http.StatusOK {
				t.Fatalf("%s HTTP status=%d", step.name, code)
			}
			member := enterprise177DecodeMember(t, body)
			if member.GetVersion() != step.version || f.count(t) != before+1 {
				t.Fatal("real action/outbox did not commit exactly once")
			}
			eventID := f.event(t, fmt.Sprintf("member-lifecycle/%s/%s/%s/%d", tenantA, user, step.status, step.version), tenantA, user, accessdomain.SecurityNotificationMemberLifecycle)
			message := f.deliver(t, eventID)
			if message.TenantID != tenantA || message.UserID != user || message.Destination != email || !strings.Contains(message.Secret, "status="+step.status) {
				t.Fatal("lifecycle delivery identity or content mismatch")
			}
			code, _ = enterprise177Post(t, base, tokenA, key, path, step.input)
			if code != http.StatusConflict || f.count(t) != before+1 {
				t.Fatal("completed action replay duplicated its event")
			}
			var other struct {
				Status  string
				Version uint64
			}
			if err := db.Table("biz_memberships").Where("tenant_id=? AND user_id=?", tenantB, user).Take(&other).Error; err != nil {
				t.Fatal(err)
			}
			if other.Status != accessdomain.TenantMemberStatusActive || other.Version != 1 {
				t.Fatal("A action affected B membership")
			}
			f.idle(t)
		}) {
			return
		}
	}
	t.Run("foreign-tenant-action-cannot-address-a-only-member", func(t *testing.T) {
		before := f.count(t)
		code, _ := enterprise177Post(t, base, tokenB, "foreign-"+stamp, "/v1/tenant/members/"+url.PathEscape(admin)+"/suspend", &accessv1.SuspendTenantMemberRequest{UserId: admin, Version: 1, Reason: "foreign request"})
		if code == http.StatusOK || f.count(t) != before {
			t.Fatal("foreign tenant action created lifecycle output")
		}
		f.idle(t)
	})
}

func TestEnterprise185InitialCredentialHTTPRequestsReachReliableWorker(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	started, protection := startB123Enterprise176Runtime(t, db)
	base := "http://" + started.HTTPAddress()
	f := newEnterprise185HTTPDelivery(t, db, protection)
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	idp, client := enterprise185IDPServer(t, store, f, protection)
	stamp := fmt.Sprint(time.Now().UnixNano())
	tenant, token := "e185-create-"+stamp, "e185-create-token-"+stamp
	seedB123TenantAdmin(t, db, tenant, "e185-admin-"+stamp, "e185-admin-"+stamp+"@example.invalid", token)
	if err := db.Exec("INSERT INTO biz_permission_grants(tenant_id,role_id,permission,scope) VALUES(?,?,?,?)", tenant, tenant+":member-admin", "tenant.role.manage", "all").Error; err != nil {
		t.Fatal(err)
	}
	role := tenant + ":operator"
	if err := db.Exec("INSERT INTO biz_roles(id,tenant_id,name,status,version) VALUES(?,?,?,?,1)", role, tenant, "operator", "active").Error; err != nil {
		t.Fatal(err)
	}
	suffix := stamp[len(stamp)-8:]
	for _, mode := range []struct {
		name  string
		value accessv1.TenantMemberActivationMode
	}{
		{"link", accessv1.TenantMemberActivationMode_TENANT_MEMBER_ACTIVATION_MODE_ACTIVATION_LINK},
		{"sms", accessv1.TenantMemberActivationMode_TENANT_MEMBER_ACTIVATION_MODE_SMS_INITIAL_PASSWORD},
	} {
		if !t.Run(mode.name+"-create-http-worker", func(t *testing.T) {
			before := f.sender.Count()
			username := mode.name + "185" + suffix
			request := &accessv1.CreateTenantMemberRequest{Username: username, Name: "Delivery member", RoleIds: []string{role}, ActivationMode: mode.value}
			if mode.name == "link" {
				request.Email = username + "@example.invalid"
			} else {
				request.Phone = "+49176" + suffix
			}
			receipt, status, body := createB123MemberHTTP(t, base, token, "create-"+mode.name+stamp, request)
			if status != http.StatusOK || receipt.GetNotificationEventId() == "" || receipt.GetDeliveryState() != accessdomain.NotificationStatePending || f.sender.Count() != before {
				t.Fatalf("initial credential request status=%d or queued receipt invalid", status)
			}
			if strings.Contains(string(body), "password=") || strings.Contains(string(body), "token=") {
				t.Fatal("creation response disclosed credential material")
			}
			message := f.deliver(t, receipt.GetNotificationEventId())
			if message.TenantID != tenant || message.UserID != receipt.GetMember().GetUserId() || message.Kind != accessdomain.SecurityNotificationInitialCredential {
				t.Fatal("initial credential targeted wrong principal")
			}
			if mode.name == "link" {
				link, err := url.Parse(message.Secret)
				if err != nil || link.Query().Get("token") == "" {
					t.Fatal("test transport did not receive a usable activation link")
				}
				values := url.Values{"token": {link.Query().Get("token")}, "new_password": {"MemberPass9A"}, "confirm_password": {"MemberPass9A"}}
				status, _ = enterprise185Form(t, client, http.MethodPost, idp.URL+"/idp/member/activate", values)
				if status != http.StatusOK {
					t.Fatalf("activation HTTP status=%d", status)
				}
				member, code, _ := getB123HTTP(t, base, token, receipt.GetMember().GetUserId())
				if code != http.StatusOK || member.GetStatus() != accessv1.TenantMemberStatus_TENANT_MEMBER_STATUS_ACTIVE {
					t.Fatal("activation HTTP handler did not activate membership")
				}
				status, _ = enterprise185Form(t, client, http.MethodPost, idp.URL+"/idp/member/activate", values)
				if status == http.StatusOK {
					t.Fatal("activation token replay accepted")
				}
			} else {
				parts := strings.Split(message.Secret, "\n")
				if len(parts) != 2 || parts[0] != "username="+username || !strings.HasPrefix(parts[1], "password=") {
					t.Fatal("initial credential transport shape invalid")
				}
				identity, err := store.AuthenticateUserPassword(context.Background(), username, strings.TrimPrefix(parts[1], "password="))
				if err != nil || !identity.PasswordChangeRequired {
					t.Fatal("initial password did not retain forced-change restriction")
				}
			}
			f.idle(t)
		}) {
			return
		}
	}
}
