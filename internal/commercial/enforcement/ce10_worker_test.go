package enforcement

import (
	"context"
	"testing"
	"yunka.io/framework/core/identity"
	"yunka.io/gateway/authz"
)

func TestCE10WorkerEntryRequiresProcessCapabilityAndUnchangedPlatform(t *testing.T) {
	g, e := New(allowedReader(), &auditRecorder{})
	if e != nil {
		t.Fatal(e)
	}
	id := "commercial.provisioning.delivery.claim"
	p := testPrincipal()
	p.TenantID = ""
	ctx := identity.WithPrincipal(context.Background(), p)
	a := authorized(id, p)
	a.Policy.Mode = authz.PermissionAll
	a.Policy.Permissions = []authz.PermissionKey{"platform.provisioning.execute"}
	if _, e = g.Prepare(ctx, a, nil); e == nil {
		t.Fatal("direct worker root accepted")
	}
	marked, e := g.WorkerContext(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = g.Prepare(marked, a, nil); e != nil {
		t.Fatal(e)
	}
	b := a
	b.Policy.Permissions = nil
	if _, e = g.Prepare(marked, b, nil); e == nil {
		t.Fatal("missing permission closure accepted")
	}
	q := p
	q.Subject = "other"
	b = authorized(id, q)
	b.Policy = a.Policy
	if _, e = g.Prepare(identity.WithPrincipal(marked, q), b, nil); e == nil {
		t.Fatal("process context rebound to another actor")
	}
	q = p
	q.TenantID = "tenant"
	if _, e = g.WorkerContext(identity.WithPrincipal(ctx, q)); e == nil {
		t.Fatal("tenant worker admitted")
	}
	if _, e = g.Prepare(marked, authorized("commercial.subscription.change.prepared", p), nil); e == nil {
		t.Fatal("process marker exposed child as root")
	}
	other, e := New(allowedReader(), &auditRecorder{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = other.Prepare(marked, a, nil); e == nil {
		t.Fatal("process marker reused across guard instances")
	}
}
