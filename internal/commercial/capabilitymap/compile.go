package capabilitymap

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

var codePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)

func problem(code, at, detail string) error {
	return fmt.Errorf("CE03-%s %s: %s", code, at, detail)
}

func version(value string) (uint64, error) {
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil || n == 0 || strconv.FormatUint(n, 10) != value {
		return 0, problem("VERSION", "mapping_version", "positive canonical decimal string required")
	}
	return n, nil
}

func unique(values []string, at string) error {
	seen := map[string]bool{}
	for _, value := range values {
		if !codePattern.MatchString(value) || seen[value] {
			return problem("DUPLICATE_OR_INVALID", at, value)
		}
		seen[value] = true
	}
	return nil
}

func sorted(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

// Compile validates the complete inventory before emitting anything. Unknown
// operations, ambiguous classifications and incomplete child declarations fail
// closed; there is deliberately no wildcard or implicit exemption.
func Compile(plans Plans, manifest Manifest, declaration Declaration, lookup LookupModule) (Document, error) {
	if plans.SchemaVersion != 2 || manifest.SchemaVersion != 3 || declaration.SchemaVersion != 1 {
		return Document{}, problem("SCHEMA", "inputs", "expected OperationPlan v2, manifest v3, declaration v1")
	}
	if _, err := version(declaration.MappingVersion); err != nil {
		return Document{}, err
	}
	if lookup == nil || len(plans.Operations) == 0 {
		return Document{}, problem("EMPTY", "inputs", "nonempty inventory and Registry lookup required")
	}
	ops := map[string]Operation{}
	for _, op := range plans.Operations {
		if _, exists := ops[op.ID]; exists || !codePattern.MatchString(op.ID) {
			return Document{}, problem("INVENTORY", op.ID, "duplicate or invalid Operation ID")
		}
		if err := unique(op.Composition.Requires, op.ID+".requiresOperations"); err != nil {
			return Document{}, err
		}
		ops[op.ID] = op
	}
	if err := validateManifest(manifest, ops); err != nil {
		return Document{}, err
	}
	if err := unique(declaration.RetiredCapabilityCodes, "retired_capability_codes"); err != nil {
		return Document{}, err
	}
	entries := map[string]CompiledOperation{}
	owners := map[string]string{}
	for _, mapping := range declaration.Operations {
		op, exists := ops[mapping.OperationID]
		if !exists {
			return Document{}, problem("UNKNOWN_OPERATION", mapping.OperationID, "not present in generated OperationPlan")
		}
		if _, exists := entries[op.ID]; exists {
			return Document{}, problem("DUPLICATE_MAPPING", op.ID, "exactly one classification required")
		}
		if err := validateMapping(mapping, op, lookup, owners); err != nil {
			return Document{}, err
		}
		mapping.CapabilityCodes = sorted(mapping.CapabilityCodes)
		mapping.Children = append([]Child{}, mapping.Children...)
		sort.Slice(mapping.Children, func(i, j int) bool { return mapping.Children[i].OperationID < mapping.Children[j].OperationID })
		entry := CompiledOperation{Mapping: mapping, ConsumerType: "internal_child", TenantRequired: op.Security.TenantRequired, RPC: op.Bindings.RPC, RequiredCapabilityCodes: []string{}}
		if op.Bindings.RPC != "" {
			entry.ConsumerType = "platform"
			if op.Security.TenantRequired {
				entry.ConsumerType = "tenant"
			}
		}
		if mapping.Classification == TenantBusiness {
			entry.RequiredCapabilityCodes = sorted(mapping.CapabilityCodes)
		}
		entries[op.ID] = entry
	}
	ids := make([]string, 0, len(ops))
	for id := range ops {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, exists := entries[id]; !exists {
			return Document{}, problem("UNCLASSIFIED", id, "new root or child requires an explicit mapping")
		}
	}
	for _, code := range declaration.RetiredCapabilityCodes {
		if _, exists := owners[code]; exists {
			return Document{}, problem("RETIRED_CAPABILITY", code, "retired capability cannot be reused")
		}
	}
	if err := validateGraph(ids, entries); err != nil {
		return Document{}, err
	}
	out := Document{SchemaVersion: 1, MappingVersion: declaration.MappingVersion, InputsSHA256: map[string]string{}, RetiredCapabilityCodes: sorted(declaration.RetiredCapabilityCodes), Capabilities: []Capability{}, Operations: []CompiledOperation{}}
	for code, module := range owners {
		out.Capabilities = append(out.Capabilities, Capability{Code: code, ModuleCode: module})
	}
	sort.Slice(out.Capabilities, func(i, j int) bool { return out.Capabilities[i].Code < out.Capabilities[j].Code })
	memo := map[string][]string{}
	for _, id := range ids {
		entry := entries[id]
		entry.AlwaysRequiredCapabilityCodes = alwaysClosure(id, entries, memo)
		out.Operations = append(out.Operations, entry)
	}
	return out, nil
}

func validateMapping(m Mapping, op Operation, lookup LookupModule, owners map[string]string) error {
	switch m.Classification {
	case TenantBusiness:
		if !op.Security.TenantRequired || m.ModuleCode == "" || len(m.CapabilityCodes) == 0 || m.ExemptionReason != "" {
			return problem("BUSINESS_MAPPING", op.ID, "tenant business requires module/capabilities and must not be exempt")
		}
	case PlatformManagement:
		if op.Security.TenantRequired {
			return problem("CLASSIFICATION", op.ID, "tenant-bound operation cannot be classified as platform management")
		}
	case Recovery, FoundationExempt:
	default:
		return problem("CLASSIFICATION", op.ID, "unknown or absent classification")
	}
	if m.Classification != TenantBusiness && strings.TrimSpace(m.ExemptionReason) == "" {
		return problem("EXEMPTION_REASON", op.ID, "non-commercial classification requires a reason; IAM is never exempt")
	}
	if err := unique(m.CapabilityCodes, op.ID+".capability_codes"); err != nil {
		return err
	}
	if (m.ModuleCode == "") != (len(m.CapabilityCodes) == 0) {
		return problem("CAPABILITY", op.ID, "module and capability ownership must be specified together")
	}
	if m.ModuleCode != "" {
		available, exists := lookup(m.ModuleCode)
		if !exists {
			return problem("UNKNOWN_MODULE", op.ID, m.ModuleCode)
		}
		for _, code := range m.CapabilityCodes {
			if !slices.Contains(available, code) {
				return problem("UNKNOWN_CAPABILITY", op.ID, m.ModuleCode+"/"+code)
			}
			if previous, exists := owners[code]; exists && previous != m.ModuleCode {
				return problem("CAPABILITY_OWNER", op.ID, code+" has conflicting module owners")
			}
			owners[code] = m.ModuleCode
		}
	}
	children := []string{}
	for _, child := range m.Children {
		children = append(children, child.OperationID)
		if child.Mode != "always" && child.Mode != "conditional" {
			return problem("CHILD_MODE", op.ID+" -> "+child.OperationID, "always or conditional required")
		}
		if (child.Mode == "conditional") != (strings.TrimSpace(child.Condition) != "") {
			return problem("CHILD_CONDITION", op.ID+" -> "+child.OperationID, "only conditional edges require a nonblank condition")
		}
		if strings.TrimSpace(child.Source) == "" {
			return problem("CHILD_SOURCE", op.ID+" -> "+child.OperationID, "implementation source required")
		}
	}
	if err := unique(children, op.ID+".children"); err != nil {
		return err
	}
	if !slices.Equal(sorted(children), sorted(op.Composition.Requires)) {
		return problem("CHILD_COVERAGE", op.ID, fmt.Sprintf("declared=%v generated=%v", sorted(children), sorted(op.Composition.Requires)))
	}
	return nil
}
