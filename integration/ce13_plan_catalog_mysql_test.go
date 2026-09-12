//go:build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	access "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"yunka.io/gateway/authz"
)

func ce13CreatePlan(t *testing.T, e *ce07Environment, code, name string) *v1.PlanVersionDTO {
	t.Helper()
	out, err := e.plans.CreatePlanDraft(e.ctx(), &v1.CreatePlanDraftRequest{
		RequestId: ce04Random(t),
		PlanCode:  code,
		Name:      name,
		Terms:     ce07Terms(),
		Reason:    "CE13 plan catalog discovery fixture",
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCE13PlanCatalogMySQLDeterministicPaginationAndAuthority(t *testing.T) {
	e := ce07New(t)
	prefix := "zz-ce13-" + ce04Random(t)
	alpha := ce13CreatePlan(t, e, prefix+"-a", "Alpha")
	beta := ce13CreatePlan(t, e, prefix+"-b", "Beta")
	gamma := ce13CreatePlan(t, e, prefix+"-c", "Gamma")

	beta = e.publish(beta)
	betaV2, err := e.plans.CreatePlanVersion(e.ctx(), &v1.CreatePlanVersionRequest{
		RequestId:            ce04Random(t),
		PlanCode:             beta.PlanCode,
		FromVersion:          beta.Version,
		ExpectedPlanRevision: beta.PlanRevision,
		Reason:               "CE13 latest-version discovery",
	})
	if err != nil {
		t.Fatal(err)
	}
	betaV2, err = e.plans.UpdatePlanDraft(e.ctx(), &v1.UpdatePlanDraftRequest{
		RequestId:        ce04Random(t),
		PlanCode:         betaV2.PlanCode,
		Version:          betaV2.Version,
		ExpectedRevision: betaV2.Revision,
		Name:             "Beta V2",
		Terms:            ce07Terms(),
		Reason:           "CE13 latest-version metadata",
	})
	if err != nil {
		t.Fatal(err)
	}

	page1, err := e.plans.ListPlans(e.ctx(), &v1.ListPlansRequest{AfterPlanCode: prefix, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Plans) != 1 || page1.Plans[0].PlanCode != alpha.PlanCode || page1.NextAfterPlanCode != alpha.PlanCode {
		t.Fatalf("page1=%v", page1)
	}
	page2, err := e.plans.ListPlans(e.ctx(), &v1.ListPlansRequest{AfterPlanCode: page1.NextAfterPlanCode, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Plans) != 1 || page2.Plans[0].PlanCode != beta.PlanCode || page2.NextAfterPlanCode != beta.PlanCode {
		t.Fatalf("page2=%v", page2)
	}
	discovered := page2.Plans[0]
	if discovered.Name != betaV2.Name ||
		discovered.LatestVersion != betaV2.Version ||
		discovered.LatestRevision != betaV2.Revision ||
		discovered.PlanRevision != betaV2.PlanRevision ||
		discovered.State != betaV2.State ||
		!reflect.DeepEqual(discovered.SalesScope, betaV2.Terms.SalesScope) ||
		discovered.CreatedAt == "" ||
		discovered.PublishedAt != "" ||
		discovered.RetiredAt != "" {
		t.Fatalf("latest authoritative summary=%v latest=%v", discovered, betaV2)
	}
	page3, err := e.plans.ListPlans(e.ctx(), &v1.ListPlansRequest{AfterPlanCode: page2.NextAfterPlanCode, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page3.Plans) != 1 || page3.Plans[0].PlanCode != gamma.PlanCode {
		t.Fatalf("page3=%v", page3)
	}

	// Discoverability must not rewrite immutable published history.
	publishedV1 := e.get(&v1.PlanVersionDTO{PlanCode: beta.PlanCode, Version: 1})
	if publishedV1.State != "PUBLISHED" || publishedV1.Name != "Beta" {
		t.Fatalf("published history changed=%v", publishedV1)
	}

	// REST route is the same authority, not a frontend-maintained directory.
	query := url.Values{}
	query.Set("after_plan_code", prefix)
	query.Set("page_size", "2")
	req, err := http.NewRequest(http.MethodGet, "http://"+e.runtime.HTTPAddress()+"/v1/platform/plans?"+query.Encode(), bytes.NewReader(nil))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	payload, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("REST catalog %d %s %v", resp.StatusCode, payload, readErr)
	}
	var restPage v1.ListPlansResponse
	if err := protojson.Unmarshal(payload, &restPage); err != nil {
		t.Fatal(err)
	}
	if len(restPage.Plans) != 2 || restPage.Plans[0].PlanCode != alpha.PlanCode || restPage.Plans[1].PlanCode != beta.PlanCode {
		t.Fatalf("REST page=%v", &restPage)
	}

	store, err := access.New(e.db)
	if err != nil {
		t.Fatal(err)
	}
	deniedToken := ce04Random(t)
	if err := store.BootstrapPlatform(context.Background(), access.PlatformBootstrap{
		Subject:     "ce13-plan-denied-" + deniedToken,
		Token:       deniedToken,
		Permissions: []authz.PermissionKey{"platform.module.read"},
	}); err != nil {
		t.Fatal(err)
	}
	_, err = e.plans.ListPlans(ce04Context(deniedToken, ""), &v1.ListPlansRequest{})
	ce07Error(t, err, codes.PermissionDenied)
	_, err = e.plans.ListPlans(ce04Context(e.tokenA, ""), &v1.ListPlansRequest{})
	ce07Error(t, err, codes.PermissionDenied)
	_, err = e.plans.ListPlans(e.ctx(), &v1.ListPlansRequest{AfterPlanCode: "INVALID", PageSize: 1})
	ce07Error(t, err, codes.InvalidArgument)
}
