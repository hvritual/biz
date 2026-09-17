package domain

import "testing"

func TestTenantBrandingNormalizesOnlyBoundedSelections(t *testing.T) {
	for _, preset := range []string{"blue", "emerald", "violet", "amber"} {
		got, primary, err := NormalizeTenantBranding(preset, "")
		if err != nil || got != preset || primary != "" {
			t.Fatalf("preset %s: %s %s %v", preset, got, primary, err)
		}
	}
	preset, primary, err := NormalizeTenantBranding(" CUSTOM ", " #125A75 ")
	if err != nil || preset != "custom" || primary != "#125a75" {
		t.Fatalf("normalization: %s %s %v", preset, primary, err)
	}
}

func TestTenantBrandingRejectsCSSAndUnreadableOrAmbiguousInput(t *testing.T) {
	cases := [][2]string{
		{"", ""}, {"other", ""}, {"blue", "#2563eb"}, {"custom", ""},
		{"custom", "#fff"}, {"custom", "#ffffffff"}, {"custom", "red"},
		{"custom", "url(https://example.invalid/x)"}, {"custom", "#ffffff"},
		{"custom", "#fefefe"}, {"custom", "var(--color-primary)"},
		{"custom", "#000000; background: red"},
	}
	for _, item := range cases {
		if _, _, err := NormalizeTenantBranding(item[0], item[1]); err == nil {
			t.Errorf("accepted invalid selection: %#v", item)
		}
	}
	if _, _, err := NormalizeTenantBranding("custom", "#000000"); err != nil {
		t.Fatal(err)
	}
}
