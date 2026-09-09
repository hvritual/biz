//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	devicev1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/modulecatalog"
	"github.com/hvritual/biz/internal/deviceops/domain"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/gorm"
	"testing"
	"time"
	"yunka.io/framework/core/identity"
)

// Legacy tests seed IAM by SQL. Their explicit fixture source facts follow the
// same pattern; this is NOT a production default grant or disabled guard. New
// CE-05 behavior tests obtain grants exclusively through real APIs.
func ce05LegacyFixtureGrants(t *testing.T, db *gorm.DB, tenant string, modules []string) {
	t.Helper()
	store, err := modulecatalog.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := modulecatalog.NewService(store, modulecatalog.ProductionRegistry())
	if err != nil {
		t.Fatal(err)
	}
	ctx := identity.WithPrincipal(context.Background(), identity.Principal{Subject: "ce05-legacy-fixture", Authenticated: true, AuthMethod: "api_key"})
	for _, code := range modules {
		_, err = svc.Get(ctx, code)
		if errors.Is(err, modulecatalog.ErrNotFound) {
			_, err = svc.Create(ctx, modulecatalog.CreateCommand{Code: code, Name: code, Reason: "legacy integration fixture", RequestID: ce04Random(t)})
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("INSERT IGNORE INTO biz_commercial_entitlement_state(tenant_id,version) VALUES(?,1)", tenant).Error; err != nil {
			return err
		}
		for _, code := range modules {
			src := entitlement.Source{ID: "ce05-fixture-" + code, TenantID: tenant, ModuleCode: code, Key: code, Kind: entitlement.Module, Effect: entitlement.Grant, SourceKind: entitlement.OverrideSource, EffectiveAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), Reason: "explicit integration fixture grant", ActorID: "ce05-legacy-fixture", Version: 1}
			b, err := json.Marshal(src)
			if err != nil {
				return err
			}
			if err := tx.Exec("INSERT IGNORE INTO biz_commercial_entitlement_sources(tenant_id,source_id,module_code,version,payload) VALUES(?,?,?,?,?)", tenant, src.ID, code, 1, string(b)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

type ce05Environment struct {
	*ce04Environment
	devices      devicev1.DeviceApplicationClient
	transfers    devicev1.DeviceTransferApplicationClient
	siteA, siteB string
}

func ce05New(t *testing.T) *ce05Environment {
	t.Helper()
	e := ce04NewEnvironment(t, "")
	conn, err := grpc.DialContext(context.Background(), e.runtime.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	out := &ce05Environment{ce04Environment: e, devices: devicev1.NewDeviceApplicationClient(conn), transfers: devicev1.NewDeviceTransferApplicationClient(conn), siteA: "a-" + ce04Random(t), siteB: "b-" + ce04Random(t)}
	access, err := accesspersistence.New(e.db)
	if err != nil {
		t.Fatal(err)
	}
	sites, err := devicepersistence.NewSiteRepository(e.db)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct{ tenant, token string }{{e.tenantA, e.tokenA}, {e.tenantB, e.tokenB}} {
		for _, permission := range []string{"device.read", "device.create", "device.update", "device.delete", "site.read"} {
			if err := e.db.Exec("INSERT IGNORE INTO biz_permission_grants(tenant_id,role_id,permission,scope) SELECT tenant_id,role_id,?,'all' FROM biz_member_roles WHERE tenant_id=? AND user_id=?", permission, v.tenant, v.tenant+"-user").Error; err != nil {
				t.Fatal(err)
			}
		}
		p, err := access.Authenticate(context.Background(), v.token)
		if err != nil {
			t.Fatal(err)
		}
		ctx := identity.WithPrincipal(context.Background(), p)
		ids := []string{out.siteA, out.siteB}
		if v.tenant == e.tenantB {
			// Site IDs are globally unique, even across tenant boundaries.
			ids = []string{"b-a-" + ce04Random(t), "b-b-" + ce04Random(t)}
		}
		for _, id := range ids {
			if err := sites.Create(ctx, &domain.Site{ID: id, Name: id}); err != nil {
				t.Fatal(err)
			}
		}
	}
	return out
}
