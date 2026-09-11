from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

source = ROOT / "internal/bizruntime/first_party_idp.go"
text = source.read_text()
old = '\twriter.Header().Set("Referrer-Policy", "no-referrer")'
new = (
    '\t// A navigate-mode HTML form POST needs a non-null Origin so the IdP\n'
    '\t// transaction cookie remains same-site. `origin` still strips the OIDC\n'
    '\t// authorization path/query from Referer, avoiding state/nonce leakage.\n'
    '\twriter.Header().Set("Referrer-Policy", "origin")'
)
if old not in text:
    raise SystemExit("first_party_idp.go referrer-policy anchor missing")
source.write_text(text.replace(old, new, 1))

test = ROOT / "internal/bizruntime/first_party_idp_hardening_test.go"
text = test.read_text()
anchor = "\nfunc testFirstPartyIdPConfig(active string, previous []FirstPartyIdPVerificationKey) FirstPartyIdPConfig {"
addition = r'''
func TestCE12FirstPartyIdPLoginReferrerPolicyPreservesFormOrigin(t *testing.T) {
	idp, err := newRuntimeFirstPartyIdP(testFirstPartyIdPConfig(testIDPPrivateKeyPEM(t), nil))
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	idp.renderLogin(recorder, http.StatusOK, "request", "csrf", "", "")
	if got := recorder.Header().Get("Referrer-Policy"); got != "origin" {
		t.Fatalf("login Referrer-Policy=%q want origin", got)
	}
	if got := recorder.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatal("login page must retain Content-Security-Policy")
	}
}
'''
if anchor not in text:
    raise SystemExit("hardening test insertion anchor missing")
if "TestCE12FirstPartyIdPLoginReferrerPolicyPreservesFormOrigin" not in text:
    test.write_text(text.replace(anchor, addition + anchor, 1))

print("CE12_LOGIN_ORIGIN_PATCHED=1")
