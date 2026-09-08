from pathlib import Path
import runpy,subprocess
runpy.run_path(str(Path(__file__).with_name('apply_base.py')),run_name='__main__')
p=Path('internal/access/application/tenantlifecycle/tenant_lifecycle_test.go')
s=p.read_text();needle='type tenantTestUnit struct{}';assert needle in s
s=s.replace(needle,'''// Complete the deliberately wide target used by the generated-wrapper test.
func (tenantTestRoleChild) AssertTenantMemberDeactivationAllowed(context.Context, *accessv1.AssertTenantMemberDeactivationAllowedRequest) (*accessv1.AssertTenantMemberDeactivationAllowedResponse, error) {
	return nil, errors.New("unexpected AssertTenantMemberDeactivationAllowed")
}

'''+needle,1)
p.write_text(s)
subprocess.run(['gofmt','-w',str(p)],check=True)
subprocess.run(['git','add',str(p)],check=True)
assert subprocess.check_output(['git','write-tree'],text=True).strip()=='cf0e3b817dfd223e0f1533a81f084723925d2883'
subprocess.run(['git','diff','--cached','--check'],check=True)
