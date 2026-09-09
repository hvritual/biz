package entitlement

import "testing"

func TestCE04ExplicitDenialWithoutUsableGrant(t *testing.T) {
 c := catalogFixture()
 c[0].Capabilities = []string{"devices.read"}
 denied := source("deny", Capability, "devices.read", Deny)
 for _, granted := range []bool{false, true} {
  sources := []Source{denied}
  if granted { sources = append(sources, grantModule()) }
  result := resolveFixture(t, epoch, c, sources)
  decision := findDecision(t, result, Capability, "devices.read", "")
  if decision.Allowed || decision.Reason != "CAPABILITY_DISABLED" { t.Fatal(decision) }
 }
 field := source("field-deny", Field, "device.identity", Deny)
 field.Action = "read"
 result := resolveFixture(t, epoch, c, []Source{field})
 if decision := findDecision(t, result, Field, "device.identity", "read"); decision.Allowed || decision.Reason != "FIELD_DISABLED" { t.Fatal(decision) }
 c[0].TechnicalStatus = "disabled"
 result = resolveFixture(t, epoch, c, []Source{field,denied})
 if decision := findDecision(t, result, Capability, "devices.read", ""); decision.Reason != "TECHNICAL_UNAVAILABLE" { t.Fatal(decision) }
 if decision := findDecision(t, result, Field, "device.identity", "read"); decision.Reason != "TECHNICAL_UNAVAILABLE" { t.Fatal(decision) }
}
