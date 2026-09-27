//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	"gorm.io/gorm"
	"yunka.io/gateway/authz"
)

type notificationBrowserFixture struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	ReaderEmail    string `json:"reader_email"`
	ReaderPassword string `json:"reader_password"`
	TenantA        string `json:"tenant_a"`
	TenantB        string `json:"tenant_b"`
	SiteID         string `json:"site_id"`
	SiteBID        string `json:"site_b_id"`
	OwnerID        string `json:"owner_id"`
	ReaderID       string `json:"reader_id"`
	ContactID      string `json:"contact_id"`
}

// Seed only the existing disposable CE12 test database, through its original
// seed entry. Never run from production startup or create another test DB.
// Separate identities/tenants preserve all historical browser test outcomes.
func seedNotificationBrowser(t *testing.T, db *gorm.DB, store *accesspersistence.Store, tenants accessv1.TenantLifecycleApplicationClient, entitlements commercialv1.EntitlementManagementApplicationClient, platformToken string) *notificationBrowserFixture {
	t.Helper()
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	owner, reader, contact := "n184-browser-owner", "n184-browser-reader", "n184-browser-contact"
	email, password := "n184.owner@example.invalid", "NotifyOwner9A"
	readerEmail, readerPassword := reader+"@example.invalid", "NotifyRead9A"
	a := ce12CreateActiveTenant(t, tenants, platformToken, "消息验收企业 A", owner, email)
	b := ce12CreateActiveTenant(t, tenants, platformToken, "消息验收企业 B", owner, email)
	permissions := []authz.PermissionKey{"tenant.notification.read", "tenant.notification.create", "tenant.notification.update", "tenant.notification.delete"}
	for _, tenant := range []string{a, b} {
		must(store.Bootstrap(ctx, accesspersistence.Bootstrap{TenantID: tenant, TenantName: tenant, UserID: owner, Email: email, Token: "n184-setup-" + tenant}, permissions))
		must(db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", tenant, owner).Update("name", "通知管理员").Error)
		var version uint64
		must(db.Table("biz_commercial_entitlement_state").Select("version").Where("tenant_id = ?", tenant).Scan(&version).Error)
		if version == 0 {
			t.Fatal("notification browser tenant has no entitlement state")
		}
		key := "n184-access-" + ce04Random(t)
		_, err := entitlements.CreateEntitlementOverride(ce04Context(platformToken, key), &commercialv1.CreateEntitlementOverrideRequest{
			RequestId: key, TenantId: tenant, ExpectedVersion: version, ModuleCode: "access-management",
			Target: commercialv1.EntitlementTarget_ENTITLEMENT_TARGET_CAPABILITY, Key: "tenant.lifecycle",
			Effect:      commercialv1.EntitlementEffect_ENTITLEMENT_EFFECT_GRANT,
			EffectiveAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), Reason: "Enterprise184 isolated live browser acceptance",
		})
		must(err)
	}
	must(store.SetUserPassword(ctx, owner, password))
	now := time.Now().UTC()
	siteID := ""
	for i := 1; i <= 103; i++ {
		id := fmt.Sprintf("n184-site-%03d", i)
		must(db.Create(&devicepersistence.SitePORecord{SitePO: devicepersistence.SitePO{Name: fmt.Sprintf("消息点位 %03d", i)}, SitePOBase: devicepersistence.SitePOBase{ID: id, TenantID: a, Version: 1, CreatedAt: now, UpdatedAt: now}}).Error)
		must(db.Exec("INSERT INTO biz_member_sites (tenant_id,user_id,site_id) VALUES (?,?,?)", a, owner, id).Error)
		if i == 103 {
			siteID = id
		}
	}
	siteB := "n184-site-b"
	must(db.Create(&devicepersistence.SitePORecord{SitePO: devicepersistence.SitePO{Name: "另一企业点位"}, SitePOBase: devicepersistence.SitePOBase{ID: siteB, TenantID: b, Version: 1, CreatedAt: now, UpdatedAt: now}}).Error)
	must(db.Exec("INSERT INTO biz_member_sites (tenant_id,user_id,site_id) VALUES (?,?,?)", b, owner, siteB).Error)
	seedReader(t, db, a, reader, "n184-reader-seed", siteID, a+":notification-reader", "Notification reader", "tenant.notification.read", "sites")
	seedReader(t, db, a, contact, "n184-contact-seed", siteID, a+":notification-contact", "Notification contact", "tenant.notification.read", "sites")
	must(store.SetUserPassword(ctx, reader, readerPassword))
	must(db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", a, reader).Update("name", "通知只读成员").Error)
	must(db.Table("biz_memberships").Where("tenant_id = ? AND user_id = ?", a, contact).Update("name", "通知接收人").Error)
	return &notificationBrowserFixture{Email: email, Password: password, ReaderEmail: readerEmail, ReaderPassword: readerPassword, TenantA: a, TenantB: b, SiteID: siteID, SiteBID: siteB, OwnerID: owner, ReaderID: reader, ContactID: contact}
}
