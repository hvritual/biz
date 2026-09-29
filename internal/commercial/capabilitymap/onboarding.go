package capabilitymap

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

const OnboardingIndexPath = "contracts/commercial/onboarding.v1.json"

// OnboardingIndex contains references only. Capabilities, IAM requirements,
// sales state and subscriptions are never editable in this index.
type OnboardingIndex struct {
	SchemaVersion int              `json:"schema_version"`
	Modules       []OnboardingLink `json:"modules"`
}

type OnboardingLink struct {
	ModuleCode string                    `json:"module_code"`
	UIRoutes   []string                  `json:"ui_routes"`
	Acceptance []OnboardingTestReference `json:"acceptance"`
}

type OnboardingTestReference struct {
	Scenario string `json:"scenario"`
	Kind     string `json:"kind"`
	Source   string `json:"source"`
	Name     string `json:"name"`
}

// OnboardingDefinition is an in-memory projection, constructed only from the
// actual ProductionRegistry by BuildOnboardingRepository. It is not decoded
// from an editable second registry.
type OnboardingDefinition struct {
	Code                  string   `json:"module_code"`
	CapabilityCodes       []string `json:"capability_codes"`
	QuotaSchemaKeys       []string `json:"quota_schema_keys"`
	FieldPolicySchemaKeys []string `json:"field_policy_schema_keys"`
	Dependencies          []string `json:"dependencies"`
	ImplementationReady   bool     `json:"implementation_ready"`
}

type OnboardingOperation struct {
	OperationID     string   `json:"operation_id"`
	ConsumerType    string   `json:"consumer_type"`
	Authentication  []string `json:"authentication"`
	IAMPermissions  []string `json:"iam_permissions"`
	PermissionMode  string   `json:"permission_mode"`
	CapabilityCodes []string `json:"capability_codes"`
	AlwaysRequired  []string `json:"always_required_capabilities"`
}

type OnboardingModuleReport struct {
	OnboardingDefinition
	Operations []OnboardingOperation     `json:"operations"`
	UIRoutes   []string                  `json:"ui_routes"`
	Acceptance []OnboardingTestReference `json:"acceptance_sources"`
}

type OnboardingFinding struct {
	Code    string `json:"code"`
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
}

type OnboardingReport struct {
	SchemaVersion       int                      `json:"schema_version"`
	Check               string                   `json:"check"`
	Result              string                   `json:"result"`
	RuntimeVerification string                   `json:"runtime_verification"`
	SalesAdmission      string                   `json:"sales_admission"`
	UIVerification      string                   `json:"ui_consumer_verification"`
	InputsSHA256        map[string]string        `json:"inputs_sha256"`
	Modules             []OnboardingModuleReport `json:"modules"`
	Findings            []OnboardingFinding      `json:"findings"`
}

func newOnboardingReport() OnboardingReport {
	return OnboardingReport{
		SchemaVersion: 1, Check: "commercial_onboarding_references", Result: "BLOCKED",
		RuntimeVerification: "NOT_PERFORMED", SalesAdmission: "NOT_PERFORMED",
		UIVerification: "REQUIRES_WEB_SOURCE_CHECK", InputsSHA256: map[string]string{},
		Modules: []OnboardingModuleReport{}, Findings: []OnboardingFinding{},
	}
}

func (r *OnboardingReport) block(code, subject, detail string) error {
	r.Result = "BLOCKED"
	r.Findings = append(r.Findings, OnboardingFinding{Code: code, Subject: subject, Detail: detail})
	return fmt.Errorf("ONBOARDING-%s %s: %s", code, subject, detail)
}

// CompileOnboarding joins already validated CE-03 metadata to the complete
// code Registry and reference index. PASS is source coverage, never an access
// decision, live test result, or permission to publish/sell a module.
func CompileOnboarding(plans Plans, doc Document, definitions []OnboardingDefinition, index OnboardingIndex) (OnboardingReport, error) {
	r := newOnboardingReport()
	fail := func(code, subject, detail string) (OnboardingReport, error) {
		err := r.block(code, subject, detail)
		return r, err
	}
	if plans.SchemaVersion != 2 || doc.SchemaVersion != 1 || index.SchemaVersion != 1 || len(definitions) == 0 || len(plans.Operations) == 0 {
		return fail("SCHEMA", "inputs", "nonempty Registry/OperationPlan v2, compiled catalog v1 and reference index v1 required")
	}
	defs := append([]OnboardingDefinition{}, definitions...)
	sort.Slice(defs, func(i, j int) bool { return defs[i].Code < defs[j].Code })
	byModule, owners := map[string]OnboardingDefinition{}, map[string]string{}
	for _, d := range defs {
		if !codePattern.MatchString(d.Code) || byModule[d.Code].Code != "" || len(d.CapabilityCodes) == 0 {
			return fail("REGISTRY", d.Code, "unique module code and nonempty capability declaration required")
		}
		for _, field := range []struct {
			label  string
			values []string
		}{
			{"capabilities", d.CapabilityCodes}, {"quotas", d.QuotaSchemaKeys},
			{"fields", d.FieldPolicySchemaKeys}, {"dependencies", d.Dependencies},
		} {
			if err := unique(field.values, field.label); err != nil {
				return fail("REGISTRY", d.Code, err.Error())
			}
		}
		for _, c := range d.CapabilityCodes {
			if owner, exists := owners[c]; exists {
				return fail("CAPABILITY_OWNER", c, "declared by both "+owner+" and "+d.Code)
			}
			if slices.Contains(doc.RetiredCapabilityCodes, c) {
				return fail("RETIRED_CAPABILITY", c, "retired code remains in Registry")
			}
			owners[c] = d.Code
		}
		byModule[d.Code] = d
	}
	states := map[string]uint8{}
	var visit func(string) error
	visit = func(code string) error {
		if states[code] == 1 {
			return r.block("DEPENDENCY_CYCLE", code, "module dependency cycle")
		}
		if states[code] == 2 {
			return nil
		}
		states[code] = 1
		for _, dep := range sorted(byModule[code].Dependencies) {
			if _, exists := byModule[dep]; !exists {
				return r.block("DEPENDENCY_MISSING", code, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		states[code] = 2
		return nil
	}
	for _, d := range defs {
		if err := visit(d.Code); err != nil {
			return r, err
		}
	}
	links := map[string]OnboardingLink{}
	for _, link := range index.Modules {
		if _, exists := byModule[link.ModuleCode]; !exists {
			return fail("UNKNOWN_MODULE", link.ModuleCode, "index references no Registry definition")
		}
		if _, exists := links[link.ModuleCode]; exists {
			return fail("DUPLICATE_LINK", link.ModuleCode, "exactly one reference entry required")
		}
		seenRoutes := map[string]bool{}
		for _, route := range link.UIRoutes {
			if !strings.HasPrefix(route, "/") || strings.TrimSpace(route) != route || strings.ContainsAny(route, "\r\n\x00?#") || seenRoutes[route] {
				return fail("CONSUMER_REFERENCE", link.ModuleCode, "unique exact route paths required")
			}
			seenRoutes[route] = true
		}
		seenScenarios := map[string]bool{}
		for _, proof := range link.Acceptance {
			if !slices.Contains([]string{"allow", "deny_entitlement", "deny_iam"}, proof.Scenario) || seenScenarios[proof.Scenario] {
				return fail("ACCEPTANCE_REFERENCE", link.ModuleCode, "unique allow/deny_entitlement/deny_iam sources required")
			}
			if proof.Kind != "mysql_source" && proof.Kind != "unit_source" {
				return fail("ACCEPTANCE_REFERENCE", link.ModuleCode, "only test-source kinds allowed; execution cannot be asserted")
			}
			if proof.Source == "" || !strings.HasPrefix(proof.Name, "Test") {
				return fail("ACCEPTANCE_REFERENCE", link.ModuleCode, "Go test path and symbol required")
			}
			seenScenarios[proof.Scenario] = true
		}
		if len(seenScenarios) != 3 {
			return fail("MISSING_EVIDENCE", link.ModuleCode, "allow, deny_entitlement and deny_iam sources required")
		}
		links[link.ModuleCode] = link
	}
	operations := map[string]Operation{}
	for _, op := range plans.Operations {
		if op.ID == "" || operations[op.ID].ID != "" {
			return fail("OPERATION_INVENTORY", op.ID, "invalid/duplicate OperationPlan entry")
		}
		operations[op.ID] = op
	}
	mapped := map[string]bool{}
	used := map[string]bool{}
	moduleOps := map[string][]OnboardingOperation{}
	for _, entry := range doc.Operations {
		op, exists := operations[entry.OperationID]
		if !exists || mapped[entry.OperationID] {
			return fail("OPERATION_INVENTORY", entry.OperationID, "compiled map and OperationPlan differ")
		}
		mapped[entry.OperationID] = true
		if entry.Classification != TenantBusiness {
			continue
		}
		if !op.Security.TenantRequired || len(op.Security.Authentication) == 0 || len(op.Security.Permissions) == 0 || (op.Security.PermissionMode != "all" && op.Security.PermissionMode != "any") {
			return fail("IAM_DECLARATION", op.ID, "tenant business requires explicit authentication, permissions and permission mode")
		}
		if err := unique(op.Security.Permissions, "permissions"); err != nil {
			return fail("IAM_DECLARATION", op.ID, err.Error())
		}
		if err := unique(op.Security.Authentication, "authentication"); err != nil {
			return fail("IAM_DECLARATION", op.ID, err.Error())
		}
		if entry.ModuleCode == "" || len(entry.CapabilityCodes) == 0 {
			return fail("CAPABILITY_MAPPING", op.ID, "business mapping missing module/capability")
		}
		for _, code := range entry.CapabilityCodes {
			if owner, ok := owners[code]; !ok || owner != entry.ModuleCode {
				return fail("CAPABILITY_MAPPING", op.ID, "unknown or foreign capability "+code)
			}
			used[code] = true
		}
		moduleOps[entry.ModuleCode] = append(moduleOps[entry.ModuleCode], OnboardingOperation{
			OperationID: op.ID, ConsumerType: entry.ConsumerType,
			Authentication: sorted(op.Security.Authentication), IAMPermissions: sorted(op.Security.Permissions), PermissionMode: op.Security.PermissionMode,
			CapabilityCodes: sorted(entry.CapabilityCodes), AlwaysRequired: sorted(entry.AlwaysRequiredCapabilityCodes),
		})
	}
	if len(mapped) != len(operations) {
		return fail("OPERATION_INVENTORY", "catalog", "every OperationPlan must have a compiled mapping")
	}
	for _, d := range defs {
		link, ok := links[d.Code]
		if !ok {
			return fail("MISSING_LINK", d.Code, "module has no onboarding references")
		}
		for _, code := range d.CapabilityCodes {
			if !used[code] {
				return fail("UNCONSUMED_CAPABILITY", d.Code, code+" has no tenant-business Operation consumer")
			}
		}
		d.CapabilityCodes, d.QuotaSchemaKeys = sorted(d.CapabilityCodes), sorted(d.QuotaSchemaKeys)
		d.FieldPolicySchemaKeys, d.Dependencies = sorted(d.FieldPolicySchemaKeys), sorted(d.Dependencies)
		ops := moduleOps[d.Code]
		sort.Slice(ops, func(i, j int) bool { return ops[i].OperationID < ops[j].OperationID })
		proofs := append([]OnboardingTestReference{}, link.Acceptance...)
		sort.Slice(proofs, func(i, j int) bool { return proofs[i].Scenario < proofs[j].Scenario })
		r.Modules = append(r.Modules, OnboardingModuleReport{OnboardingDefinition: d, Operations: ops, UIRoutes: sorted(link.UIRoutes), Acceptance: proofs})
	}
	r.Result = "PASS"
	return r, nil
}
