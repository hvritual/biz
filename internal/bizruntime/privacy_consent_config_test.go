package bizruntime

import "testing"

func TestEnterprise169PrivacyConsentConfigRequiresExplicitPolicyInputs(t *testing.T) {
	valid := FirstPartyPrivacyConsentConfig{
		AgreementVersion: "2026-09-v1",
		PrivacyPolicyURL: "https://example.invalid/privacy",
		TermsURL:         "https://example.invalid/terms",
		ReconsentPolicy:  PrivacyReconsentCurrentVersion,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid privacy config rejected: %v", err)
	}
	cases := []FirstPartyPrivacyConsentConfig{
		{},
		{AgreementVersion: "v1", PrivacyPolicyURL: "https://example.invalid/privacy", TermsURL: "https://example.invalid/terms"},
		{AgreementVersion: "v1", PrivacyPolicyURL: "javascript:alert(1)", TermsURL: "https://example.invalid/terms", ReconsentPolicy: PrivacyReconsentCurrentVersion},
	}
	for index, candidate := range cases {
		if err := candidate.Validate(); err == nil {
			t.Fatalf("invalid privacy config %d accepted: %+v", index, candidate)
		}
	}
}

func TestEnterprise169PrivacyReconsentPolicyIsExplicit(t *testing.T) {
	current := FirstPartyPrivacyConsentConfig{ReconsentPolicy: PrivacyReconsentCurrentVersion}
	if !current.RequireCurrentVersion() {
		t.Fatal("current-version policy must require current agreement")
	}
	grandfather := FirstPartyPrivacyConsentConfig{ReconsentPolicy: PrivacyReconsentAnyActive}
	if grandfather.RequireCurrentVersion() {
		t.Fatal("any-active policy must not require current agreement version")
	}
}
