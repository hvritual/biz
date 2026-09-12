//go:build integration

package integration

import (
	"context"
	"encoding/json"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/bizruntime"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"github.com/hvritual/biz/internal/commercial/ports"
	"github.com/hvritual/biz/modules/deviceops"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"sync/atomic"
	"testing"
	"time"
	"yunka.io/framework/execution"
	"yunka.io/framework/platform"
	"yunka.io/pkg/logExt"
)

// Synthetic local policy evidence validates only the typed adapter contract;
// it is explicitly not CE-19 real device/member quota consumption.
type ce09TestQuotaPolicy struct {
	used  atomic.Uint64
	delay atomic.Int64
	calls atomic.Int64
}

func (p *ce09TestQuotaPolicy) Evaluate(ctx context.Context, in ports.QuotaChangeInput) ([]change.QuotaImpact, error) {
	if _, ok := execution.Current(ctx); !ok {
		return nil, change.ErrCorrupt
	}
	p.calls.Add(1)
	if d := time.Duration(p.delay.Load()); d > 0 {
		select {
		case <-time.After(d):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	out := change.DefaultQuotaImpacts(in.Changes)
	for i := range out {
		if change.Less(out[i].After, out[i].Before) {
			out[i].UsageKnown = true
			out[i].Used = p.used.Load()
			out[i].OverLimit = !out[i].After.Unlimited && out[i].Used > out[i].After.Value
			out[i].Policy = "SYNTHETIC_LOCAL_POLICY_FIXTURE"
			out[i].Evidence = "CE09 adapter contract test; not real resource usage"
		}
	}
	return out, nil
}
func ce09WithPolicy(t *testing.T, policy ports.QuotaChangePolicy) *ce09Environment {
	t.Helper()
	db := ce08FreshFixtureDB(t)
	token := ce04Random(t)
	config := deviceops.DefaultConfig()
	config.HTTPListenAddress = "127.0.0.1:0"
	config.GRPCListenAddress = "127.0.0.1:0"
	config.AutoMigrate = true
	provider, err := platform.New(platform.Options{Config: bizruntime.ConfigProvider{DeviceOps: config}, Logger: logExt.NewBaseLogger(), Databases: map[string]platform.DatabaseFactory{"primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
		return platform.BorrowedDatabase(db), nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	started, err := bizruntime.BootstrapWithOptions(ctx, provider, bizruntime.Options{DeviceOps: config, QuotaChangePolicy: policy, PlatformBootstrap: bizruntime.PlatformBootstrap{Subject: "ce09-policy:" + token, Token: token, Permissions: ce09Permissions()}})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		ctx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = started.App.Shutdown(ctx)
	})
	seedB122DefaultSubscription(t, started, token)
	conn, err := grpc.DialContext(context.Background(), started.GRPCAddress(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	base := &ce08Environment{t: t, db: db, token: token, base: "http://" + started.HTTPAddress(), grpcAddress: started.GRPCAddress(), plans: v1.NewPlanManagementApplicationClient(conn), subscriptions: v1.NewSubscriptionManagementApplicationClient(conn)}
	e := &ce09Environment{ce08Environment: base, changes: v1.NewSubscriptionChangesApplicationClient(conn), entitlements: v1.NewEntitlementManagementApplicationClient(conn), catalog: v1.NewModuleCatalogApplicationClient(conn)}
	e.old = e.plan(ce09Terms(10, 30))
	e.putRule("ce09", 100, e.old)
	id := ce04Random(t)
	e.tenant = ce08Tenant(t, e.createTenant(id, id, "o-"+id, "ce09")).Id
	return e
}
func TestCE09MySQLQuotaEvidenceIsRecheckedDuringConfirmation(t *testing.T) {
	policy := &ce09TestQuotaPolicy{}
	policy.used.Store(1)
	e := ce09WithPolicy(t, policy)
	p := e.preview(e.plan(ce09Terms(2, 30)))
	if p.QuotaValidationRequired || len(p.QuotaImpacts) != 1 || !p.QuotaImpacts[0].UsageKnown || p.QuotaImpacts[0].Used != 1 {
		t.Fatal("typed policy evidence not shown")
	}
	policy.used.Store(3)
	r, err := e.confirm(e.confirmation(p))
	if err != nil {
		t.Fatal(err)
	}
	if !r.QuotaValidationRequired || !r.QuotaImpacts[0].OverLimit || r.QuotaImpacts[0].Used != 3 || r.Status != "SCHEDULED" || policy.calls.Load() != 2 || ce09Quota(t, e.view()) != 10 {
		t.Fatal("confirmation trusted obsolete usage or applied overage")
	}
}
func TestCE09MySQLPreviewExpiresDuringLocalPolicyBeforeMutation(t *testing.T) {
	policy := &ce09TestQuotaPolicy{}
	e := ce09WithPolicy(t, policy)
	p := e.preview(e.plan(ce09Terms(20, 30)))
	var row struct{ Payload string }
	if err := e.db.Table("biz_commercial_change_previews").Where("change_id=?", p.ChangeId).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	var stored change.Preview
	if err := json.Unmarshal([]byte(row.Payload), &stored); err != nil {
		t.Fatal(err)
	}
	var now time.Time
	if err := e.db.Raw("SELECT UTC_TIMESTAMP(6)").Scan(&now).Error; err != nil {
		t.Fatal(err)
	}
	stored.ExpiresAt = now.Add(1500 * time.Millisecond)
	stored = stored.Seal()
	raw, _ := json.Marshal(stored)
	if err := e.db.Table("biz_commercial_change_previews").Where("change_id=?", p.ChangeId).Updates(map[string]any{"payload": string(raw), "payload_sha256": stored.Hash, "expires_at": stored.ExpiresAt}).Error; err != nil {
		t.Fatal(err)
	}
	p, err := e.changes.GetSubscriptionChangePreview(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: e.tenant, ChangeId: p.ChangeId})
	if err != nil {
		t.Fatal(err)
	}
	before := ce09State(t, e)
	policy.delay.Store(int64(2 * time.Second))
	_, err = e.confirm(e.confirmation(p))
	ce09Code(t, err, codes.FailedPrecondition)
	ce09EqualState(t, before, ce09State(t, e))
}
