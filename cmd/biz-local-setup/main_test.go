package main

import (
	"testing"

	"github.com/hvritual/biz/internal/commercial/application/planprojection"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	"github.com/hvritual/biz/internal/commercial/modulecatalog"
)

func TestValidateLocalDSNRequiresExactEndpointAndDatabase(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want bool
	}{
		{name: "designated database", dsn: "user:password@tcp(127.0.0.1:13316)/biz_evolution?parseTime=true", want: true},
		{name: "database prefix is not enough", dsn: "user:password@tcp(127.0.0.1:13316)/biz_evolution_other", want: false},
		{name: "database in password is not enough", dsn: "user:biz_evolution@tcp(127.0.0.1:13316)/other", want: false},
		{name: "port must match", dsn: "user:password@tcp(127.0.0.1:3306)/biz_evolution", want: false},
		{name: "host must match", dsn: "user:password@tcp(localhost:13316)/biz_evolution", want: false},
		{name: "network must be tcp", dsn: "user:password@unix(/tmp/mysql.sock)/biz_evolution", want: false},
		{name: "malformed", dsn: "not a mysql dsn", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateLocalDSN(test.dsn)
			if (err == nil) != test.want {
				t.Fatalf("validateLocalDSN(%q) error=%v, want success=%v", test.dsn, err, test.want)
			}
		})
	}
}

func TestLocalPlanTermsRespectProductionCatalogFieldFloor(t *testing.T) {
	terms, err := planprojection.Terms(localPlanTerms())
	if err != nil {
		t.Fatal(err)
	}
	if err := terms.Validate(localPlanCatalog()); err != nil {
		t.Fatalf("local setup plan is not eligible: %v", err)
	}
	fields := map[string]string{}
	for _, module := range terms.Modules {
		if module.Code != "access-management" {
			continue
		}
		for _, field := range module.Fields {
			fields[field.Action] = field.Mode
		}
	}
	if fields["read"] != "masked" || fields["write"] != "allow" || fields["export"] != "deny" {
		t.Fatalf("member.profile fields=%v", fields)
	}
}

func localPlanCatalog() plan.Catalog {
	registry := modulecatalog.ProductionRegistry()
	catalog := make(plan.Catalog, 0, 2)
	for _, code := range []string{"access-management", "device-operations"} {
		definition, ok := registry.Definition(code)
		if !ok {
			panic("production module definition missing: " + code)
		}
		catalog = append(catalog, plan.Definition{ModuleDefinition: entitlement.ModuleDefinition{
			Code:            definition.Code,
			Version:         1,
			TechnicalStatus: "ready",
			SalesStatus:     "sellable",
			Capabilities:    definition.CapabilityCodes,
			QuotaKeys:       definition.QuotaSchemaKeys,
			FieldKeys:       definition.FieldPolicySchemaKeys,
			Dependencies:    definition.Dependencies,
		}, SalesScope: []string{"*"}})
	}
	return catalog
}
