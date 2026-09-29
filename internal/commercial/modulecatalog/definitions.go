package modulecatalog

import "sort"

// Definitions is a deterministic, detached projection of the code-owned
// registry. Inspection cannot mutate runtime definitions or set sales state.
func (r Registry) Definitions() []Definition {
	out := make([]Definition, 0, len(r.definitions))
	for _, d := range r.definitions {
		d.CapabilityCodes = append([]string{}, d.CapabilityCodes...)
		d.QuotaSchemaKeys = append([]string{}, d.QuotaSchemaKeys...)
		d.FieldPolicySchemaKeys = append([]string{}, d.FieldPolicySchemaKeys...)
		d.Dependencies = append([]string{}, d.Dependencies...)
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}
