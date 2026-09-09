package capabilitymap

import (
	"fmt"
	"slices"
)

// Cross-check both external RPCs and application-private operations. Comparing
// only public methods would silently omit the four current internal children.
func validateManifest(manifest Manifest, ops map[string]Operation) error {
	seen := map[string]bool{}
	check := func(domain, application string, m ManifestOperation, rpc string, http []HTTPBinding) error {
		op, exists := ops[m.ID]
		if !exists || seen[m.ID] {
			return problem("RPC_INVENTORY", m.ID, "missing from OperationPlan or duplicated in manifest")
		}
		seen[m.ID] = true
		if op.Domain != domain || op.Application != application || op.Security.TenantRequired != m.TenantRequired || op.Bindings.RPC != rpc || !slices.Equal(op.Bindings.HTTP, http) || !slices.Equal(sorted(op.Composition.Requires), sorted(m.Requires)) {
			return problem("RPC_DRIFT", m.ID, "manifest/OperationPlan identity, security, transport or child graph differs")
		}
		return nil
	}
	for _, service := range manifest.Services {
		if service.FullName == "" || service.Domain == "" || service.Application.Name == "" {
			return problem("RPC_INVENTORY", service.FullName, "service identity required")
		}
		for _, method := range service.Methods {
			if method.Name == "" {
				return problem("RPC_INVENTORY", service.FullName, "method name required")
			}
			if err := check(service.Domain, service.Application.Name, method.Operation, "/"+service.FullName+"/"+method.Name, method.HTTP); err != nil {
				return err
			}
		}
		for _, internal := range service.Application.Operations {
			if err := check(service.Domain, service.Application.Name, internal, "", nil); err != nil {
				return err
			}
		}
	}
	if len(seen) != len(ops) {
		ids := []string{}
		for id := range ops {
			if !seen[id] {
				ids = append(ids, id)
			}
		}
		return problem("RPC_INVENTORY", "operations", fmt.Sprintf("absent from manifest: %v", sorted(ids)))
	}
	return nil
}

func validateGraph(ids []string, entries map[string]CompiledOperation) error {
	state := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		entry, exists := entries[id]
		if !exists {
			return problem("CHILD_MAPPING", id, "child mapping/capability is missing")
		}
		if state[id] == 1 {
			return problem("CHILD_CYCLE", id, "cycle in root/child graph")
		}
		if state[id] == 2 {
			return nil
		}
		state[id] = 1
		for _, child := range entry.Children {
			if err := visit(child.OperationID); err != nil {
				return err
			}
		}
		state[id] = 2
		return nil
	}
	for _, id := range ids {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

// Conditional edges remain in Document. They never get flattened into a root's
// mandatory purchase set. A consumer traverses them only on the actual branch;
// this compiler does not execute or authorize that branch.
func alwaysClosure(id string, entries map[string]CompiledOperation, memo map[string][]string) []string {
	if previous, exists := memo[id]; exists {
		return append([]string{}, previous...)
	}
	set := map[string]bool{}
	entry := entries[id]
	for _, code := range entry.RequiredCapabilityCodes {
		set[code] = true
	}
	for _, child := range entry.Children {
		if child.Mode == "always" {
			for _, code := range alwaysClosure(child.OperationID, entries, memo) {
				set[code] = true
			}
		}
	}
	values := []string{}
	for code := range set {
		values = append(values, code)
	}
	memo[id] = sorted(values)
	return append([]string{}, memo[id]...)
}
