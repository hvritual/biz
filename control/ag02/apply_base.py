from pathlib import Path
import subprocess
root=Path.cwd()
assert subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()=='7d5afbe9cb4b849be462d9a7aed65877ed227700'
assert not subprocess.check_output(['git','status','--porcelain'],text=True).strip()
a=root/'internal/access/application'
owner=a/'tenantlifecycle'; hidden=owner/'internal/usecase'; hidden.mkdir(parents=True)
source=(a/'tenant_lifecycle.go').read_text()
source=source.replace('package application','package usecase',1)
source=source.replace('accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"','accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"\n accessapp "github.com/hvritual/biz/internal/access/application"',1)
source=source.replace('var ErrInvalidTenantRequest = errors.New("access: invalid tenant request")\n','')
source=source.replace('NewTenantLifecycleService','New').replace('TenantLifecycleService','service').replace('TenantLifecycleCapabilities','accessapp.TenantLifecycleCapabilities').replace('ErrInvalidTenantRequest','accessapp.ErrInvalidTenantRequest')
source=source.replace('(*service, error)', '(accessapp.TenantLifecycleApplication, error)')
source=source.replace('type service struct {','// service is hidden behind the canonical generated Application interface.\ntype service struct {',1)
source=source.replace('func New(', '// New is called only by the owning tenantlifecycle factory.\nfunc New(',1)
(hidden/'tenant_lifecycle.go').write_text(source)
(a/'tenant_lifecycle.go').unlink()
(a/'tenant_errors.go').write_text('package application\n\nimport "errors"\n\n// ErrInvalidTenantRequest retains the existing error identity at the contract seam.\nvar ErrInvalidTenantRequest = errors.New("access: invalid tenant request")\n')
(owner/'build.go').write_text('''// Package tenantlifecycle owns the handwritten TenantLifecycle implementation.
// Build is a composition-root entry, not a business-call shortcut.
package tenantlifecycle

import (
 accessapp "github.com/hvritual/biz/internal/access/application"
 "github.com/hvritual/biz/internal/access/application/tenantlifecycle/internal/usecase"
 "github.com/hvritual/biz/internal/access/ports"
 "yunka.io/framework/requestscope"
)

// Build returns only the canonical application port. It does not start a runtime,
// resolve services, construct child wrappers, or own a root transaction.
func Build(repositories requestscope.RepositoryFactory[ports.TenantRepositories], capabilities accessapp.TenantLifecycleCapabilities) (accessapp.TenantLifecycleApplication, error) {
 return usecase.New(repositories, capabilities)
}
''')
test=(a/'tenant_lifecycle_test.go').read_text()
helper=test[test.index('type tenantTestUnit struct{}'):test.index('type memoryTenantRepository struct')]
(a/'tenant_testunit_test.go').write_text('package application\n\nimport ("context"; "yunka.io/framework/execution")\n\n'+helper)
test=test.replace('package application','package tenantlifecycle',1)
test=test.replace('accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"','accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"\n accessapp "github.com/hvritual/biz/internal/access/application"',1)
for name in ['TenantLifecycleToAccessTenantMemberLifecycleChildCapability','TenantLifecycleToAccessTenantRolePermissionChildCapability']:
 test=test.replace(name,'accessapp.'+name)
test=test.replace('NewTenantLifecycleService','Build')
(owner/'tenant_lifecycle_test.go').write_text(test)
(a/'tenant_lifecycle_test.go').unlink()
for rel in ['internal/bizruntime/access_skeleton.go','integration/b12_tenant_lifecycle_mysql_test.go']:
 p=root/rel; text=p.read_text();text=text.replace('accessapp "github.com/hvritual/biz/internal/access/application"','accessapp "github.com/hvritual/biz/internal/access/application"\n "github.com/hvritual/biz/internal/access/application/tenantlifecycle"',1)
 text=text.replace('accessapp.NewTenantLifecycleService','tenantlifecycle.Build').replace('*accessapp.TenantLifecycleService','accessapp.TenantLifecycleApplication')
 p.write_text(text)
(owner/'boundary_test.go').write_text('''package tenantlifecycle

import (
 "context"
 "reflect"
 "testing"

 accessapp "github.com/hvritual/biz/internal/access/application"
 "github.com/hvritual/biz/internal/access/ports"
 "yunka.io/framework/operation"
 "yunka.io/framework/requestscope"
)

func TestAG02FactoryKeepsCanonicalSurface(t *testing.T) {
 repositories := requestscope.RepositoryFactory[ports.TenantRepositories](func(context.Context, requestscope.UnitOfWork) (ports.TenantRepositories, error) {
  t.Fatal("construction must not resolve request repositories")
  return ports.TenantRepositories{}, nil
 })
 if app, err := Build(nil, tenantTestCapabilities{}); err == nil || app != nil { t.Fatal("missing repository accepted") }
 if app, err := Build(repositories, nil); err == nil || app != nil { t.Fatal("missing capabilities accepted") }
 app, err := Build(repositories, tenantTestCapabilities{})
 if err != nil { t.Fatal(err) }
 typ := reflect.TypeOf(app)
 if typ.Kind() != reflect.Pointer || typ.Elem().PkgPath() != "github.com/hvritual/biz/internal/access/application/tenantlifecycle/internal/usecase" {
  t.Fatalf("application implementation not hidden: %v", typ)
 }
 assertAG02Methods(t, typ, reflect.TypeOf((*accessapp.TenantLifecycleApplication)(nil)).Elem())
 for i := 0; i < typ.Elem().NumField(); i++ {
  field := typ.Elem().Field(i)
  if field.IsExported() || field.Anonymous { t.Fatalf("implementation field leaks: %v", field) }
 }
}

func TestAG02GeneratedChildrenAreAttenuated(t *testing.T) {
 // The deliberately wide test targets already implement all target methods.
 // The real generated wrapper, not a narrow interface assignment, must remove
 // those extra methods. Runtime invocation is qualified separately on MySQL.
 executor := operation.NewExecutor(nil)
 member, err := accessapp.NewTenantLifecycleToAccessTenantMemberLifecycleChildCapability(tenantTestMemberChild{}, executor)
 if err != nil { t.Fatal(err) }
 role, err := accessapp.NewTenantLifecycleToAccessTenantRolePermissionChildCapability(tenantTestRoleChild{}, executor)
 if err != nil { t.Fatal(err) }
 if _, ok := member.(accessapp.TenantMemberLifecycleApplication); ok { t.Fatal("full member application leaked") }
 if _, ok := role.(accessapp.TenantRolePermissionApplication); ok { t.Fatal("full role application leaked") }
 assertAG02Methods(t, reflect.TypeOf(member), reflect.TypeOf((*accessapp.TenantLifecycleToAccessTenantMemberLifecycleChildCapability)(nil)).Elem())
 assertAG02Methods(t, reflect.TypeOf(role), reflect.TypeOf((*accessapp.TenantLifecycleToAccessTenantRolePermissionChildCapability)(nil)).Elem())
}

func assertAG02Methods(t *testing.T, implementation, contract reflect.Type) {
 t.Helper()
 if implementation.NumMethod() != contract.NumMethod() { t.Fatalf("method set widened: %v (%d), contract %v (%d)", implementation, implementation.NumMethod(), contract, contract.NumMethod()) }
 for i := 0; i < contract.NumMethod(); i++ {
  name := contract.Method(i).Name
  if _, ok := implementation.MethodByName(name); !ok { t.Fatalf("missing canonical method %s", name) }
 }
}
''')
(root/'integration/ag02_tenant_encapsulation_mysql_test.go').write_text('''//go:build integration

package integration

import (
 "reflect"
 "testing"
)

func TestAG02RuntimeAssemblesHiddenTenantImplementation(t *testing.T) {
 started := startB122Runtime(t, openDB(t), "ag02-assembly-test")
 typ := reflect.TypeOf(started.Applications.AccessTenantLifecycle)
 if typ == nil || typ.Kind() != reflect.Pointer || typ.Elem().PkgPath() != "github.com/hvritual/biz/internal/access/application/tenantlifecycle/internal/usecase" {
  t.Fatalf("runtime still uses a non-sealed TenantLifecycle: %v", typ)
 }
}
''')
(root/'scripts/verify-tenant-encapsulation.py').write_text('''#!/usr/bin/env python3
"""AG-02 real-consumer import regression. No source edits or policy exemptions."""
import hashlib
import os
from pathlib import Path
import re
import signal
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
MODULE = "github.com/hvritual/biz"
OWNER = MODULE + "/internal/access/application/tenantlifecycle"
HIDDEN = OWNER + "/internal/usecase"


def run(cwd, env, *args):
    process = subprocess.Popen(args, cwd=cwd, env=env, stdout=subprocess.PIPE,
                               stderr=subprocess.STDOUT, start_new_session=True)
    try:
        output, _ = process.communicate(timeout=120)
        return process.returncode, output.decode("utf-8", errors="strict")
    except subprocess.TimeoutExpired as error:
        raise RuntimeError("AG-02 INCOMPLETE: compiler timeout") from error
    finally:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.communicate()


def main():
    if os.name != "posix":
        raise RuntimeError("AG-02 INCOMPLETE: process cleanup requires a qualified POSIX host")
    names = subprocess.check_output(["git", "ls-files", "-z"], cwd=ROOT).decode().split("\\0")
    sources = {}
    for name in filter(None, names):
        path = ROOT / name
        if path.is_symlink() or not path.is_file():
            raise RuntimeError("AG-02 INCOMPLETE: unsupported tracked entry " + name)
        sources[name] = path.read_bytes()
    original = {name: hashlib.sha256(data).hexdigest() for name, data in sources.items()}
    try:
        with tempfile.TemporaryDirectory(prefix="biz-ag02-") as temp:
            copy = Path(temp) / "biz"
            for name, data in sources.items():
                target = copy / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(data)
            (Path(temp) / "yunka.io").symlink_to((ROOT.parent / "yunka.io").resolve(), target_is_directory=True)
            env = {key: value for key, value in os.environ.items()
                   if not key.startswith(("GO", "CGO_", "YUNKA_", "GH_", "GITHUB_"))}
            env.update(GOWORK=str(copy / "go.work"), GOENV="off", GOTOOLCHAIN="local",
                       GOPROXY="off", GOSUMDB="off", GOVCS="*:off", CGO_ENABLED="0",
                       GOFLAGS="-mod=readonly -buildvcs=false")
            probe = copy / "internal/ag02probe/probe.go"
            probe.parent.mkdir(parents=True, exist_ok=True)
            cases = [
                ("canonical_port", MODULE + "/internal/access/application", "api", "TenantLifecycleApplication", False),
                ("owner_factory", OWNER, "api", "Build", False),
                ("hidden_implementation", HIDDEN, "usecase", "New", True),
                ("aliased_hidden_implementation", HIDDEN, "renamed", "New", True),
            ]
            for name, target, alias, symbol, reject in cases:
                declaration = f"var _ {alias}.{symbol}" if name == "canonical_port" else f"var _ = {alias}.{symbol}"
                probe.write_text(f'package ag02probe\\nimport {alias} "{target}"\\n{declaration}\\n')
                code, output = run(copy, env, "go", "build", "./internal/ag02probe")
                if reject:
                    diagnostic = re.compile(r"^internal/ag02probe/probe.go:\\d+:\\d+: use of internal package " + re.escape(HIDDEN) + r" not allowed$")
                    lines = [line.strip() for line in output.splitlines() if line.strip()]
                    errors = [line for line in lines if not line.startswith(("package " + MODULE, "# " + MODULE, "imports " + MODULE))]
                    if code != 1 or len(errors) != 1 or not diagnostic.fullmatch(errors[0]):
                        raise RuntimeError(f"{name}: wrong rejection, code={code}:\\n{output}")
                elif code != 0 or output.strip():
                    raise RuntimeError(f"{name}: legal control failed, code={code}:\\n{output}")
                print(f"AG02_IMPORT_PASS {name}", flush=True)
    finally:
        after = {name: hashlib.sha256((ROOT / name).read_bytes()).hexdigest() for name in sources}
        if after != original:
            raise RuntimeError("AG-02 source changed during import regression")


if __name__ == "__main__":
    main()
''')
p=root/'Makefile';s=p.read_text().replace('consumer-certify\n','consumer-certify architecture-check\n',1).replace('test:\n', 'architecture-check:\n\t@python3 scripts/verify-tenant-encapsulation.py\n\ntest:\n',1).replace('verify: workspace-check check test','verify: workspace-check check architecture-check test');p.write_text(s)
doc=root/'docs/architecture/AG-02-tenant-encapsulation.md';doc.parent.mkdir(parents=True,exist_ok=True)
doc.write_text('''# AG-02 — TenantLifecycle encapsulation pilot

> Document class: **CURRENT**
> Scope: one consumer Application; not a general architecture analyzer or a framework upgrade
> Framework plan: `hvritual/yunka.io/docs/architecture/APPLICATION-GOVERNANCE-PLAN.md`
> Exact qualification/integration identity: the AG-02 PR and its machine receipt

## Delivered structure

```text
internal/access/application/
    zz_yunka_tenant_lifecycle_*_gen.go   canonical generated ports/wrappers, unchanged
    tenant_errors.go                  existing sentinel identity
    tenantlifecycle/
        build.go                      composition-only factory
        tenant_lifecycle_test.go      original behavior assertions via Build
        boundary_test.go              actual method-set checks
        internal/usecase/
            tenant_lifecycle.go       unexported implementation
```

The owner is below the existing canonical `application` root instead of a new
sibling convention. This preserves current Change Plan/Ownership placement and
Audit source coverage without broadening a matcher or hand-editing generated
files. The illustrative layout in the framework plan is not a second source of
Application identity.

`bizruntime.applicationFactories` calls `tenantlifecycle.Build`. The factory
returns the generated `TenantLifecycleApplication`, not the concrete implementation.
Its seven business methods, validation, ID generation, DTO mapping, CAS and
requestscope joins retain the old behavior. `ErrInvalidTenantRequest` remains at
the old import path with the same sentinel identity within the process.

Generated source-edge child wrappers stay canonical. This pilot moves handwritten
TenantLifecycle code outside their package-private scope; it does not replace
`ExecuteChildTyped`, create an Executor, resolve services, or finalize root UoWs.
The existing Member and Role implementations have NOT yet been similarly moved.

## Compatibility and limits

PB source, Operation IDs, permission/tenant/transaction/idempotency declarations,
all generated bytes, database schemas and the pinned Yunka source remain unchanged.
The old handwritten concrete constructor/type is intentionally removed from this
repository-internal API; all repository call sites are migrated in the same task.
Shared member-test UnitOfWork helpers remain test-only in the original package.

This pilot establishes implementation encapsulation, not complete use-case/port
splitting, universal factory-call enforcement or a same-process security sandbox.
Other code can still import the owner's public factory. Generic composition-only
factory checks belong to AG-04; this task does not claim they already exist.

The tests use deterministic doubles only for unit assertions. Real SQL, tenant
isolation, authorization, CAS, rollback and child-Operation effects are separately
qualified with the existing MySQL integration suite and actual runtime assembly.

## Reproducible verification

With the exact Yunka checkout declared by `.yunka/source.env` as the sibling
`../yunka.io`, and its locked Go/protoc installed:

```sh
make yunka-source-check
make generate
make check
make consumer-certify
make verify
# The following requires a disposable MySQL 8.4 database; never point at production.
go test -count=1 -tags=integration ./integration
go test -race -count=1 ./internal/access/application/... ./internal/bizruntime/...
```

`make architecture-check` runs the dedicated real-project import regression in a
disposable tracked-source copy. Both the canonical port and owning factory must
build before ordinary/aliased imports of the hidden implementation are counted as
rejections. Exactly the expected `internal ... not allowed` diagnostic is required;
missing dependencies, syntax errors and timeouts are failures, not successful
negative cases. All original tracked bytes are checked afterward. Subprocess
cleanup is POSIX-only; unsupported hosts report INCOMPLETE explicitly.

The original lifecycle tests are retained, not replaced by structural assertions.
New tests also require the actual runtime Assembly to produce the hidden concrete
implementation and compare real generated child-wrapper method sets with their
canonical interfaces. A small interface assignment alone is not accepted as
attenuation evidence.

Qualification records must name the candidate SHA/tree, runtime framework pin,
CLI probe pin when different, exact commands, pass/fail/skip counts, generated
zero-drift evidence, independent-review disposition and remote readback. Candidate
qualification is not automatically main integration.

## Rollback

Revert the complete AG-02 commit(s), including factory bindings and relocated tests.
No schema or data migration is included, so no production-data rollback is implied.
''')
p=root/'README.md';p.write_text(p.read_text()+'''
## TenantLifecycle implementation encapsulation

The TenantLifecycle handwritten implementation is owned by
`internal/access/application/tenantlifecycle/internal/usecase`; runtime assembly
uses `tenantlifecycle.Build` and the unchanged generated Application/child ports.
`make architecture-check` validates legal public imports and rejected hidden imports
in a disposable copy; `make verify` includes that check. Member and Role have not
yet been migrated. This is an implementation-layout pilot, not a Yunka version
upgrade. See [AG-02](docs/architecture/AG-02-tenant-encapsulation.md) for guarantees,
limits, qualification commands and rollback.
''')
subprocess.run(['gofmt','-w','internal/access/application/tenant_errors.go','internal/access/application/tenant_testunit_test.go','internal/access/application/tenantlifecycle','internal/bizruntime/access_skeleton.go','integration/b12_tenant_lifecycle_mysql_test.go','integration/ag02_tenant_encapsulation_mysql_test.go'],check=True)
subprocess.run(['git','add','Makefile','README.md','docs/architecture/AG-02-tenant-encapsulation.md','internal/access/application','internal/bizruntime/access_skeleton.go','integration/b12_tenant_lifecycle_mysql_test.go','integration/ag02_tenant_encapsulation_mysql_test.go','scripts/verify-tenant-encapsulation.py'],check=True)
assert subprocess.check_output(['git','write-tree'],text=True).strip()=='14bb42f90cbb2bc109dc065d69ace8ac11cb9ed7', 'candidate tree differs from inspected local implementation'
subprocess.run(['git','diff','--cached','--check'],check=True)
