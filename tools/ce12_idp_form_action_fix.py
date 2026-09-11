from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

source = ROOT / "internal/bizruntime/first_party_idp.go"
text = source.read_text()
old = '\twriter.Header().Set("Content-Security-Policy", "default-src \'none\'; style-src \'unsafe-inline\'; form-action \'self\'; base-uri \'none\'; frame-ancestors \'none\'")'
new = '''\tformAction := "'self'"
\tif publicURL, err := url.Parse(idp.config.PublicURL); err == nil && publicURL.Scheme != "" && publicURL.Host != "" {
\t\tformAction += " " + publicURL.Scheme + "://" + publicURL.Host
\t}
\twriter.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action "+formAction+"; base-uri 'none'; frame-ancestors 'none'")'''
if old not in text:
    raise SystemExit("first_party_idp.go CSP anchor missing")
source.write_text(text.replace(old, new, 1))

test = ROOT / "internal/bizruntime/first_party_idp_hardening_test.go"
text = test.read_text()
old = '''\tif got := recorder.Header().Get("Content-Security-Policy"); got == "" {
\t\tt.Fatal("login page must retain Content-Security-Policy")
\t}'''
new = '''\tcsp := recorder.Header().Get("Content-Security-Policy")
\tif csp == "" {
\t\tt.Fatal("login page must retain Content-Security-Policy")
\t}
\tif want := "form-action 'self' http://127.0.0.1:18081"; !strings.Contains(csp, want) {
\t\tt.Fatalf("login Content-Security-Policy=%q missing %q", csp, want)
\t}'''
if old not in text:
    raise SystemExit("hardening test CSP anchor missing")
text = text.replace(old, new, 1)
if '\t"strings"\n' not in text:
    text = text.replace('\t"net/url"\n', '\t"net/url"\n\t"strings"\n', 1)
test.write_text(text)

print("CE12_IDP_FORM_ACTION_PATCHED=1")
