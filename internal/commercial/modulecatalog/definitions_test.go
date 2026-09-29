package modulecatalog

import "testing"

func TestRegistryInspectionIsSortedAndDetached(t *testing.T) {
	r, err := NewRegistry([]Definition{
		{Code: "second", CapabilityCodes: []string{"second.use"}, Dependencies: []string{"first"}},
		{Code: "first", CapabilityCodes: []string{"first.use"}, QuotaSchemaKeys: []string{"first.count"}, FieldPolicySchemaKeys: []string{"first.field"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	values := r.Definitions()
	if len(values) != 2 || values[0].Code != "first" {
		t.Fatal(values)
	}
	values[0].CapabilityCodes[0] = "changed"
	values[0].QuotaSchemaKeys[0] = "changed"
	values[0].FieldPolicySchemaKeys[0] = "changed"
	values[1].Dependencies[0] = "changed"
	values[0].ImplementationReady = true
	fresh := r.Definitions()
	if fresh[0].CapabilityCodes[0] != "first.use" || fresh[0].QuotaSchemaKeys[0] != "first.count" || fresh[0].FieldPolicySchemaKeys[0] != "first.field" || fresh[1].Dependencies[0] != "first" || fresh[0].ImplementationReady {
		t.Fatal("inspection mutated code authority", fresh)
	}
}
