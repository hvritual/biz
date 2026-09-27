//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"gorm.io/gorm"
)

func TestEnterprise185LoginAndRecoveryHTTPEntrypointsReachReliableWorker(t *testing.T) {
	fixture := newEnterprise173Fixture(t)
	f := newEnterprise185HTTPDelivery(t, fixture.DB, fixture.Protection)
	idp, client := enterprise185IDPServer(t, fixture.Store, f, fixture.Protection)
	ctx := context.Background()
	stamp := fmt.Sprint(time.Now().UnixNano())
	user, email := "e185-idp-"+stamp, "idp-"+stamp+"@example.invalid"
	oldPassword, newPassword := "CoffeePass9A", "CoffeePass9B"
	if err := fixture.Store.BootstrapGlobalUser(ctx, accesspersistence.GlobalUserBootstrap{ID: user, Email: email}); err != nil {
		t.Fatal(err)
	}
	if err := fixture.Store.SetUserPassword(ctx, user, oldPassword); err != nil {
		t.Fatal(err)
	}

	if !t.Run("login-lock-and-outbox-commit-atomically", func(t *testing.T) {
		query := url.Values{"response_type": {"code"}, "client_id": {"notification-http-185"}, "redirect_uri": {"http://127.0.0.1/auth/callback"}, "state": {"state-185"}, "nonce": {"nonce-185"}, "scope": {"openid email"}, "code_challenge": {strings.Repeat("a", 43)}, "code_challenge_method": {"S256"}}
		status, body := enterprise185Form(t, client, http.MethodGet, idp.URL+"/idp/authorize?"+query.Encode(), nil)
		if status != http.StatusOK {
			t.Fatalf("authorization HTTP status=%d", status)
		}
		values := url.Values{"request_id": {enterprise185Hidden(t, body, "request_id")}, "csrf_token": {enterprise185Hidden(t, body, "csrf_token")}, "identifier": {email}, "password": {"wrong-password"}}
		policy := accesspersistence.DefaultFirstPartyLoginPolicy()
		for attempt := 0; attempt < policy.MaxFailures-1; attempt++ {
			status, _ = enterprise185Form(t, client, http.MethodPost, idp.URL+"/idp/login", values)
			if status != http.StatusUnauthorized {
				t.Fatalf("pre-lock wrong-password HTTP status=%d", status)
			}
		}
		var before struct {
			FailureCount int
			BlockedUntil *time.Time
		}
		if err := fixture.DB.Table("biz_idp_login_throttles").
			Where("identity_hash=?", accesspersistence.LoginIdentifierThrottleHash(email)).
			Take(&before).Error; err != nil {
			t.Fatal(err)
		}
		if before.FailureCount != policy.MaxFailures-1 || before.BlockedUntil != nil || f.count(t) != 0 {
			t.Fatalf("pre-lock state count=%d blocked=%v outbox=%d", before.FailureCount, before.BlockedUntil, f.count(t))
		}

		const callback = "enterprise185:reject-login-lock-outbox"
		if err := fixture.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
			if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "biz_security_notification_outbox" {
				tx.AddError(errors.New("injected login lock outbox failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		status, _ = enterprise185Form(t, client, http.MethodPost, idp.URL+"/idp/login", values)
		if removeErr := fixture.DB.Callback().Create().Remove(callback); removeErr != nil {
			t.Fatal(removeErr)
		}
		if status != http.StatusUnauthorized {
			t.Fatalf("faulted threshold login status=%d", status)
		}
		var rolledBack struct {
			FailureCount int
			BlockedUntil *time.Time
		}
		if err := fixture.DB.Table("biz_idp_login_throttles").
			Where("identity_hash=?", accesspersistence.LoginIdentifierThrottleHash(email)).
			Take(&rolledBack).Error; err != nil {
			t.Fatal(err)
		}
		if rolledBack.FailureCount != policy.MaxFailures-1 || rolledBack.BlockedUntil != nil || f.count(t) != 0 {
			t.Fatalf("outbox failure left partial lock count=%d blocked=%v outbox=%d", rolledBack.FailureCount, rolledBack.BlockedUntil, f.count(t))
		}

		status, _ = enterprise185Form(t, client, http.MethodPost, idp.URL+"/idp/login", values)
		if status != http.StatusUnauthorized {
			t.Fatalf("threshold retry HTTP status=%d", status)
		}
		blocked, _, err := fixture.Store.FirstPartyLoginThrottleState(ctx, email)
		if err != nil || !blocked || f.count(t) != 1 || f.sender.Count() != 0 {
			t.Fatal("successful lock transition did not atomically enqueue one event")
		}
		var row struct{ EventID, UserID, TenantID, Kind, State string }
		if err := fixture.DB.Table("biz_security_notification_outbox").Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.UserID != user || row.TenantID != "" || row.Kind != string(accessdomain.SecurityNotificationLoginLock) || row.State != accessdomain.NotificationStatePending {
			t.Fatal("login lock notification did not retain tenantless global identity")
		}

		status, _ = enterprise185Form(t, client, http.MethodPost, idp.URL+"/idp/login", values)
		if status != http.StatusUnauthorized || f.count(t) != 1 {
			t.Fatal("already-blocked login duplicated lock notification")
		}
		message := f.deliver(t, row.EventID)
		if message.UserID != user || message.Destination != email || message.Secret != "" {
			t.Fatal("login lock delivery content mismatch")
		}
		f.idle(t)
	}) {
		return
	}

	t.Run("recovery-http-cookie-otp-commit-worker", func(t *testing.T) {
		status, body := enterprise185Form(t, client, http.MethodGet, idp.URL+"/idp/password/recovery", nil)
		if status != http.StatusOK {
			t.Fatalf("recovery page HTTP status=%d", status)
		}
		flow := enterprise185Hidden(t, body, "flow_id")
		requestValues := url.Values{"flow_id": {flow}, "request_nonce": {enterprise185Hidden(t, body, "request_nonce")}, "identifier": {email}}
		before := f.sender.Count()
		status, body = enterprise185Form(t, client, http.MethodPost, idp.URL+"/idp/password/recovery/request", requestValues)
		if status != http.StatusOK || f.sender.Count() != before {
			t.Fatal("recovery request performed provider I/O or failed")
		}
		challenge := enterprise185Hidden(t, body, "challenge_id")
		if strings.HasPrefix(challenge, "vch-fake-") {
			t.Fatal("real account did not get a recovery challenge")
		}
		otpEvent := enterprise182ChallengeEventID(t, fixture.DB, challenge)
		otp := f.deliver(t, otpEvent)
		values := url.Values{"flow_id": {flow}, "challenge_id": {challenge}, "identifier": {email}, "otp_code": {otp.Secret}, "new_password": {newPassword}, "confirm_password": {newPassword}}
		// A copied form without the browser cookie is not an authorization.
		uncookied := &http.Client{Timeout: 8 * time.Second}
		status, _ = enterprise185Form(t, uncookied, http.MethodPost, idp.URL+"/idp/password/recovery/complete", values)
		if status != http.StatusUnauthorized {
			t.Fatal("recovery without browser binding was accepted")
		}
		before = f.sender.Count()
		status, body = enterprise185Form(t, client, http.MethodPost, idp.URL+"/idp/password/recovery/complete", values)
		if status != http.StatusOK || !strings.Contains(body, "密码已更新") || f.sender.Count() != before {
			t.Fatalf("recovery completion status=%d or synchronous provider I/O", status)
		}
		if _, err := fixture.Store.AuthenticateUserPassword(ctx, email, newPassword); err != nil {
			t.Fatal("new credential did not take effect")
		}
		if _, err := fixture.Store.AuthenticateUserPassword(ctx, email, oldPassword); err == nil {
			t.Fatal("old credential survived HTTP recovery")
		}
		event := f.event(t, "password-reset-complete/"+challenge, "", user, accessdomain.SecurityNotificationPasswordResetCompleted)
		message := f.deliver(t, event)
		if message.Secret != "" || message.Destination != email {
			t.Fatal("completion notification contained secret material or wrong destination")
		}
		count := f.count(t)
		status, _ = enterprise185Form(t, client, http.MethodPost, idp.URL+"/idp/password/recovery/complete", values)
		if status == http.StatusOK || f.count(t) != count {
			t.Fatal("consumed HTTP recovery replay produced another event")
		}
		f.idle(t)
	})
}
