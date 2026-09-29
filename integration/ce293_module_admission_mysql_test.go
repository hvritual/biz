//go:build integration

package integration

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/hvritual/biz/internal/commercial/modulecatalog"
	"yunka.io/framework/core/identity"
)

func ce293Verifier() context.Context {
	return identity.WithPrincipal(context.Background(), identity.Principal{Subject: "ci-verifier:ce293", Authenticated: true, AuthMethod: identity.AuthMethodServiceToken})
}

func TestCE293MySQLSignedRuntimeVerificationIsExactVersionSalesAdmission(t *testing.T) {
	db := ce02DB(t)
	store, err := modulecatalog.NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	service, err := modulecatalog.NewService(store, modulecatalog.ProductionRegistry())
	if err != nil {
		t.Fatal(err)
	}
	feature, err := service.Create(ce02Platform(), modulecatalog.CreateCommand{RequestID: "ce293-create", Code: "device-operations", Name: "Device", Category: "ops", Reason: "qualification module"})
	if err != nil || feature.SalesStatus != modulecatalog.SalesRetired {
		t.Fatalf("create=%+v err=%v", feature, err)
	}
	proof := modulecatalog.RuntimeVerificationCommand{ModuleCode: feature.Code, ModuleVersion: feature.Version, EvidenceDigest: strings.Repeat("a", 64), SourceTree: strings.Repeat("b", 64)}
	if err := service.RecordRuntimeVerification(ce02Platform(), proof); !errors.Is(err, modulecatalog.ErrRuntimeVerifierRequired) {
		t.Fatalf("platform self-verification err=%v", err)
	}
	if _, err := service.SetSalesStatus(ce02Platform(), modulecatalog.StatusCommand{RequestID: "ce293-sell-without-proof", Code: feature.Code, Version: feature.Version, Sales: modulecatalog.SalesSellable, Reason: "must reject"}); !errors.Is(err, modulecatalog.ErrRuntimeAdmissionRequired) {
		t.Fatalf("sell without proof err=%v", err)
	}
	if err := service.RecordRuntimeVerification(ce293Verifier(), proof); err != nil {
		t.Fatal(err)
	}
	feature, err = service.SetSalesStatus(ce02Platform(), modulecatalog.StatusCommand{RequestID: "ce293-sell", Code: feature.Code, Version: feature.Version, Sales: modulecatalog.SalesSellable, Reason: "verified release"})
	if err != nil || feature.SalesStatus != modulecatalog.SalesSellable || feature.Version != proof.ModuleVersion {
		t.Fatalf("sell=%+v err=%v", feature, err)
	}
	feature, err = service.Update(ce02Platform(), modulecatalog.UpdateCommand{RequestID: "ce293-version-change", Code: feature.Code, Name: "Device v2", Category: "ops", Version: feature.Version, Reason: "new runtime candidate"})
	if err != nil || feature.SalesStatus != modulecatalog.SalesRetired {
		t.Fatalf("version change=%+v err=%v", feature, err)
	}
	if _, err := service.SetSalesStatus(ce02Platform(), modulecatalog.StatusCommand{RequestID: "ce293-stale-proof", Code: feature.Code, Version: feature.Version, Sales: modulecatalog.SalesSellable, Reason: "must reverify"}); !errors.Is(err, modulecatalog.ErrRuntimeAdmissionRequired) {
		t.Fatalf("stale proof err=%v", err)
	}
}
