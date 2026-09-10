//go:build integration

package integration

import (
	"encoding/json"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"os"
	"testing"
)

type ce09RestartData struct {
	Token       string `json:"synthetic_platform_token"`
	Tenant      string `json:"synthetic_tenant"`
	RequestJSON string `json:"confirm_request"`
	ReceiptJSON string `json:"receipt"`
	PreviewJSON string `json:"pending_preview"`
}

func TestCE09PersistenceBeforeRestart(t *testing.T) {
	path := os.Getenv("CE09_RESTART_RECEIPT")
	if path == "" {
		t.Fatal("CE09_RESTART_RECEIPT required")
	}
	e := ce09OnDB(t, openDB(t), ce04Random(t))
	e.old = e.plan(ce09Terms(10, 30))
	e.putRule("ce09", 100, e.old)
	k := ce04Random(t)
	e.tenant = ce08Tenant(t, e.createTenant(k, k, "o-"+k, "ce09")).Id
	p := e.preview(e.plan(ce09Terms(20, 30)))
	req := e.confirmation(p)
	r, err := e.confirm(req)
	if err != nil {
		t.Fatal(err)
	}
	pending := e.preview(e.plan(ce09Terms(30, 30)))
	rq, err := protojson.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	rp, err := protojson.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	pp, err := protojson.Marshal(pending)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(ce09RestartData{e.token, e.tenant, string(rq), string(rp), string(pp)})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
func TestCE09PersistenceAfterRestart(t *testing.T) {
	raw, err := os.ReadFile(os.Getenv("CE09_RESTART_RECEIPT"))
	if err != nil {
		t.Fatal(err)
	}
	var saved ce09RestartData
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	e := ce09OnDB(t, openDB(t), saved.Token)
	e.tenant = saved.Tenant
	req := &v1.ConfirmSubscriptionChangeRequest{}
	expected := &v1.SubscriptionChangeReceiptDTO{}
	p := &v1.SubscriptionChangePreviewDTO{}
	for _, v := range []struct {
		s string
		m proto.Message
	}{{saved.RequestJSON, req}, {saved.ReceiptJSON, expected}, {saved.PreviewJSON, p}} {
		if err = protojson.Unmarshal([]byte(v.s), v.m); err != nil {
			t.Fatal(err)
		}
	}
	got, err := e.confirm(req)
	if err != nil || !proto.Equal(got, expected) {
		t.Fatalf("completed durable receipt lost after restart %v %v", got, err)
	}
	read, err := e.changes.GetSubscriptionChangePreview(e.ctx(), &v1.ReadSubscriptionChangeRequest{TenantId: e.tenant, ChangeId: p.ChangeId})
	if err != nil || !proto.Equal(read, p) {
		t.Fatalf("stored preview lost %v %v", read, err)
	}
	applied, err := e.confirm(e.confirmation(p))
	if err != nil {
		t.Fatal(err)
	}
	if applied.After.Revision != expected.After.Revision+1 || applied.AfterSourceVersion != expected.AfterSourceVersion+1 || ce09Quota(t, e.view()) != 30 {
		t.Fatal("restart resumed wrong source generation")
	}
}
