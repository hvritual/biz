from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def edit(path: str, old: str, new: str) -> None:
    p = ROOT / path
    s = p.read_text()
    if old not in s:
        raise SystemExit(f"expected fragment not found in {path}: {old[:120]!r}")
    p.write_text(s.replace(old, new, 1))

edit(
    "modules/deviceops/module.go",
    '\t"yunka.io/framework/execution"\n\t"yunka.io/framework/operation"\n',
    '\t"yunka.io/framework/execution"\n\t"yunka.io/framework/execution/idempotencygorm"\n\t"yunka.io/framework/operation"\n',
)
edit(
    "modules/deviceops/module.go",
    '''\tidempotency, err := execution.NewIdempotencyCoordinator(execution.NewMemoryIdempotencyStore())
\tif err != nil {
\t\treturn err
\t}
''',
    '''\tidempotencyStore, err := idempotencygorm.NewStore(module.dependencies.PrimaryDatabase, idempotencygorm.Options{})
\tif err != nil {
\t\treturn err
\t}
\tif module.dependencies.Config.AutoMigrate {
\t\tif err := idempotencyStore.EnsureSchema(ctx); err != nil {
\t\t\treturn fmt.Errorf("deviceops: idempotency migrate: %w", err)
\t\t}
\t}
\tidempotency, err := execution.NewIdempotencyCoordinator(idempotencyStore)
\tif err != nil {
\t\treturn err
\t}
''',
)

p = ROOT / "integration/c9_pressure_mysql_test.go"
s = p.read_text()
if '"gorm.io/gorm"' not in s:
    s = s.replace(
        '\tdevicesecurity "github.com/hvritual/biz/internal/deviceops/security"\n',
        '\tdevicesecurity "github.com/hvritual/biz/internal/deviceops/security"\n\t"gorm.io/gorm"\n',
        1,
    )
s = s.replace(
    '\t"yunka.io/framework/execution"\n\t"yunka.io/framework/operation"\n',
    '\t"yunka.io/framework/execution"\n\t"yunka.io/framework/execution/idempotencygorm"\n\t"yunka.io/framework/operation"\n',
    1,
)
old = '''\tidempotency, err := execution.NewIdempotencyCoordinator(execution.NewMemoryIdempotencyStore())
\tif err != nil {
\t\tt.Fatal(err)
\t}
'''
new = '''\tidempotencyStore, err := idempotencygorm.NewStore(database, idempotencygorm.Options{})
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tif err := idempotencyStore.EnsureSchema(context.Background()); err != nil {
\t\tt.Fatal(err)
\t}
\tidempotency, err := execution.NewIdempotencyCoordinator(idempotencyStore)
\tif err != nil {
\t\tt.Fatal(err)
\t}
'''
if old not in s:
    raise SystemExit("pressure idempotency anchor missing")
s = s.replace(old, new, 1)
needle = '''\tif observer.starts(devicepolicy.OperationRemoteProvision) != 1 {
\t\tt.Fatalf("remote saga security starts=%d want=1", observer.starts(devicepolicy.OperationRemoteProvision))
\t}
'''
if "durable idempotency duplicate suppression" not in s:
    if needle not in s:
        raise SystemExit("remote provision success anchor missing")
    s = s.replace(needle, needle + '''\t_, err = operation.ExecuteTyped(execution.WithIdempotencyKey(ownerContext, "provision:"+serial), executor, devicepolicy.RemoteProvisionPressurePlan(), &deviceopsv1.CreateDeviceRequest{SiteId: site, Name: "Saga", Serial: serial}, service.Provision)
\tif !errors.Is(err, execution.ErrIdempotencyCompleted) {
\t\tt.Fatalf("expected durable idempotency duplicate suppression, got %v", err)
\t}
''', 1)
p.write_text(s)
print("durable C9.7 biz migration staged")
