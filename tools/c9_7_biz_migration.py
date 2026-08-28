from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def edit(path: str, old: str, new: str) -> None:
    target = ROOT / path
    text = target.read_text()
    if old not in text:
        raise SystemExit(f"expected fragment not found in {path}: {old[:120]!r}")
    target.write_text(text.replace(old, new, 1))

# Repository factories become joinable views over the Executor-owned UoW.
edit(
    "internal/deviceops/infrastructure/persistence/scoped.go",
    '''func NewScopedScopeFactory(database *gorm.DB) (requestscope.ScopeFactory[ports.ScopedRepositories], error) {
\tunit, err := requestscope.NewGORMFactory(database, nil)
\tif err != nil {
\t\treturn nil, err
\t}
\treturn requestscope.NewFactory(requestscope.FactoryOptions[ports.ScopedRepositories]{
\t\tUnitOfWork: unit,
\t\tRepositories: requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.ScopedRepositories, error) {
\t\t\tdevices, err := NewDeviceRepository(transaction)
\t\t\tif err != nil {
\t\t\t\treturn ports.ScopedRepositories{}, err
\t\t\t}
\t\t\tsites, err := NewSiteRepository(transaction)
\t\t\tif err != nil {
\t\t\t\treturn ports.ScopedRepositories{}, err
\t\t\t}
\t\t\treturn ports.ScopedRepositories{Device: devices, Site: sites}, nil
\t\t}),
\t})
}
''',
    '''func NewScopedRepositoryFactory(database *gorm.DB) (requestscope.RepositoryFactory[ports.ScopedRepositories], error) {
\tif database == nil {
\t\treturn nil, errors.New("deviceops persistence: database is required")
\t}
\treturn requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.ScopedRepositories, error) {
\t\tdevices, err := NewDeviceRepository(transaction)
\t\tif err != nil {
\t\t\treturn ports.ScopedRepositories{}, err
\t\t}
\t\tsites, err := NewSiteRepository(transaction)
\t\tif err != nil {
\t\t\treturn ports.ScopedRepositories{}, err
\t\t}
\t\treturn ports.ScopedRepositories{Device: devices, Site: sites}, nil
\t}), nil
}

// NewScopedScopeFactory remains a compatibility seam for pre-C9.7 callers.
func NewScopedScopeFactory(database *gorm.DB) (requestscope.ScopeFactory[ports.ScopedRepositories], error) {
\tunit, err := requestscope.NewGORMFactory(database, nil)
\tif err != nil {
\t\treturn nil, err
\t}
\trepositories, err := NewScopedRepositoryFactory(database)
\tif err != nil {
\t\treturn nil, err
\t}
\treturn requestscope.NewFactory(requestscope.FactoryOptions[ports.ScopedRepositories]{UnitOfWork: unit, Repositories: repositories})
}
''',
)

edit(
    "internal/deviceops/infrastructure/persistence/local_composition.go",
    'import (\n\t"context"\n',
    'import (\n\t"context"\n\t"errors"\n',
)
edit(
    "internal/deviceops/infrastructure/persistence/local_composition.go",
    '''// NewLocalTransferScopeFactory proves the C8.7 local-composition invariant:
// heterogeneous repository ports are constructed over exactly one request-owned
// UnitOfWork. The Application receives typed ports and never a second DB handle.
func NewLocalTransferScopeFactory(database *gorm.DB) (requestscope.ScopeFactory[LocalTransferRepositories], error) {
\tunit, err := requestscope.NewGORMFactory(database, nil)
\tif err != nil {
\t\treturn nil, err
\t}
\tdevices := requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.ScopedDeviceRepository, error) {
\t\treturn NewDeviceRepository(transaction)
\t})
\tsites := requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.SiteRepository, error) {
\t\treturn NewSiteRepository(transaction)
\t})
\treturn requestscope.NewFactory(requestscope.FactoryOptions[LocalTransferRepositories]{
\t\tUnitOfWork:   unit,
\t\tRepositories: requestscope.Compose2(devices, sites),
\t})
}
''',
    '''// NewLocalTransferRepositoryFactory composes heterogeneous repository ports over
// the UnitOfWork already owned by the root C9.7 ExecutionScope.
func NewLocalTransferRepositoryFactory(database *gorm.DB) (requestscope.RepositoryFactory[LocalTransferRepositories], error) {
\tif database == nil {
\t\treturn nil, errors.New("deviceops persistence: database is required")
\t}
\tdevices := requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.ScopedDeviceRepository, error) {
\t\treturn NewDeviceRepository(transaction)
\t})
\tsites := requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (ports.SiteRepository, error) {
\t\treturn NewSiteRepository(transaction)
\t})
\treturn requestscope.Compose2(devices, sites), nil
}

// NewLocalTransferScopeFactory remains for pre-C9.7 compatibility tests.
func NewLocalTransferScopeFactory(database *gorm.DB) (requestscope.ScopeFactory[LocalTransferRepositories], error) {
\tunit, err := requestscope.NewGORMFactory(database, nil)
\tif err != nil {
\t\treturn nil, err
\t}
\trepositories, err := NewLocalTransferRepositoryFactory(database)
\tif err != nil {
\t\treturn nil, err
\t}
\treturn requestscope.NewFactory(requestscope.FactoryOptions[LocalTransferRepositories]{UnitOfWork: unit, Repositories: repositories})
}
''',
)

# Application no longer owns transaction lifecycle.
edit(
    "internal/deviceops/application/service.go",
    '''type Service struct {
\tscopes requestscope.ScopeFactory[ports.ScopedRepositories]
}

func NewService(scopes requestscope.ScopeFactory[ports.ScopedRepositories]) (*Service, error) {
\tif scopes == nil {
\t\treturn nil, errors.New("deviceops: request scope factory is required")
\t}
\treturn &Service{scopes: scopes}, nil
}
''',
    '''type Service struct {
\trepositories requestscope.RepositoryFactory[ports.ScopedRepositories]
}

func NewService(repositories requestscope.RepositoryFactory[ports.ScopedRepositories]) (*Service, error) {
\tif repositories == nil {
\t\treturn nil, errors.New("deviceops: repository factory is required")
\t}
\treturn &Service{repositories: repositories}, nil
}
''',
)
p = ROOT / "internal/deviceops/application/service.go"
s = p.read_text()
s = s.replace('requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories])', 'requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.ScopedRepositories])')
s = s.replace('requestscope.Execute(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories])', 'requestscope.JoinDo(ctx, service.repositories, func(scope *requestscope.View[ports.ScopedRepositories])')
p.write_text(s)

edit(
    "internal/deviceops/application/local_composition.go",
    '''type LocalTransferService struct {
\tscopes requestscope.ScopeFactory[localTransferRepositories]
}

func NewLocalTransferService(scopes requestscope.ScopeFactory[localTransferRepositories]) (*LocalTransferService, error) {
\tif scopes == nil {
\t\treturn nil, errors.New("deviceops local transfer: request scope factory is required")
\t}
\treturn &LocalTransferService{scopes: scopes}, nil
}
''',
    '''type LocalTransferService struct {
\trepositories requestscope.RepositoryFactory[localTransferRepositories]
}

func NewLocalTransferService(repositories requestscope.RepositoryFactory[localTransferRepositories]) (*LocalTransferService, error) {
\tif repositories == nil {
\t\treturn nil, errors.New("deviceops local transfer: repository factory is required")
\t}
\treturn &LocalTransferService{repositories: repositories}, nil
}
''',
)
p = ROOT / "internal/deviceops/application/local_composition.go"
s = p.read_text().replace('requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[localTransferRepositories])', 'requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[localTransferRepositories])')
s = s.replace('request-owned UnitOfWork composed by requestscope.Compose2.', 'root ExecutionScope UnitOfWork composed by requestscope.Compose2.')
p.write_text(s)

# Saga staging joins the exact ExecutionScope transaction.
p = ROOT / "internal/deviceops/application/remote_saga.go"
s = p.read_text().replace('\t"yunka.io/framework/event/outbox"\n', '')
s = s.replace('''type ProvisioningService struct {
\tscopes requestscope.ScopeFactory[ports.ScopedRepositories]
\toutbox outbox.TransactionalStore
}

func NewProvisioningService(scopes requestscope.ScopeFactory[ports.ScopedRepositories], store outbox.TransactionalStore) (*ProvisioningService, error) {
\tif scopes == nil {
\t\treturn nil, errors.New("deviceops provisioning: request scope factory is required")
\t}
\tif store == nil {
\t\treturn nil, errors.New("deviceops provisioning: transactional outbox is required")
\t}
\treturn &ProvisioningService{scopes: scopes, outbox: store}, nil
}
''', '''type ProvisioningService struct {
\trepositories requestscope.RepositoryFactory[ports.ScopedRepositories]
\tstager       saga.Stager
}

func NewProvisioningService(repositories requestscope.RepositoryFactory[ports.ScopedRepositories], stager saga.Stager) (*ProvisioningService, error) {
\tif repositories == nil {
\t\treturn nil, errors.New("deviceops provisioning: repository factory is required")
\t}
\tif stager == nil {
\t\treturn nil, errors.New("deviceops provisioning: saga stager is required")
\t}
\treturn &ProvisioningService{repositories: repositories, stager: stager}, nil
}
''')
s = s.replace('requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[ports.ScopedRepositories])', 'requestscope.JoinValue(ctx, service.repositories, func(scope *requestscope.View[ports.ScopedRepositories])')
s = s.replace('''\t\t// Framework Pressure FP-C9-002: Saga/Outbox atomic staging still leaks
\t\t// the adapter-specific transaction seam into Application composition.
\t\ttransaction, err := requestscope.GORMFrom(scope.UnitOfWork())
\t\tif err != nil {
\t\t\treturn domain.Device{}, err
\t\t}
\t\tif err := saga.EnqueueTx(scope.Context(), service.outbox, transaction, plan); err != nil {
''', '''\t\t// C9.7 closes FP-C9-002: Saga stages against the current ExecutionScope
\t\t// transaction without exposing GORM or a transaction handle to Application.
\t\tif err := service.stager.Stage(scope.Context(), plan); err != nil {
''')
p.write_text(s)

# Explicit internal pressure plans exercise C9.7 transaction/idempotency runtime.
p = ROOT / "internal/deviceops/policy/pressure_plans.go"
s = p.read_text()
s = s.replace('\t\tSecurity: operationplan.Security{', '\t\tExecution: operationplan.Execution{Transaction: "local", Idempotency: "none"},\n\t\tSecurity: operationplan.Security{', 1)
start = s.index('func RemoteProvisionPressurePlan')
pos = s.index('\t\tSecurity: operationplan.Security{', start)
s = s[:pos] + '\t\tExecution: operationplan.Execution{Transaction: "local", Idempotency: "required"},\n' + s[pos:]
p.write_text(s)

# PB owns explicit execution semantics for the real public operations.
p = ROOT / "contracts/proto/deviceops/v1/deviceops.proto"
s = p.read_text()
for op, tx, idem in [
    ('device.list', 'TRANSACTION_READ_ONLY', 'IDEMPOTENCY_NONE'),
    ('device.get', 'TRANSACTION_READ_ONLY', 'IDEMPOTENCY_NONE'),
    ('device.create', 'TRANSACTION_LOCAL', 'IDEMPOTENCY_REQUIRED'),
    ('device.update', 'TRANSACTION_LOCAL', 'IDEMPOTENCY_REQUIRED'),
    ('device.delete', 'TRANSACTION_LOCAL', 'IDEMPOTENCY_REQUIRED'),
]:
    start = s.index(f'id: "{op}"')
    end = s.index('    };', start)
    segment = s[start:end]
    anchor = '      authentication: AUTHENTICATION_API_KEY\n'
    if anchor not in segment:
        raise SystemExit(f'auth anchor missing for {op}')
    segment = segment.replace(anchor, anchor + f'      execution: {{ transaction: {tx} idempotency: {idem} }}\n')
    s = s[:start] + segment + s[end:]
p.write_text(s)

# Module composes one transaction/idempotency-capable Executor.
p = ROOT / "modules/deviceops/module.go"
s = p.read_text().replace('\t"yunka.io/framework/core/identity"\n\t"yunka.io/framework/operation"\n', '\t"yunka.io/framework/core/identity"\n\t"yunka.io/framework/execution"\n\t"yunka.io/framework/operation"\n\t"yunka.io/framework/requestscope"\n')
old = '''\texecutor := operation.NewExecutor(security)
\tscopes, err := devicepersistence.NewScopedScopeFactory(module.dependencies.PrimaryDatabase)
\tif err != nil {
\t\treturn err
\t}
\tservice, err := deviceapp.NewService(scopes)
'''
new = '''\ttransactions, err := requestscope.NewGORMExecutionFactory(module.dependencies.PrimaryDatabase)
\tif err != nil {
\t\treturn err
\t}
\tidempotency, err := execution.NewIdempotencyCoordinator(execution.NewMemoryIdempotencyStore())
\tif err != nil {
\t\treturn err
\t}
\texecutor := operation.NewExecutorWithOptions(security, operation.ExecutorOptions{Transactions: transactions, Idempotency: idempotency})
\trepositories, err := devicepersistence.NewScopedRepositoryFactory(module.dependencies.PrimaryDatabase)
\tif err != nil {
\t\treturn err
\t}
\tservice, err := deviceapp.NewService(repositories)
'''
if old not in s:
    raise SystemExit('module executor anchor missing')
p.write_text(s.replace(old, new, 1))

# Real MySQL tests exercise transport and internal pressure operations.
p = ROOT / "integration/deviceops_mysql_test.go"
s = p.read_text()
old = 'req.Header.Set("Content-Type", "application/json")\n'
if old not in s:
    raise SystemExit('postDevice header anchor missing')
p.write_text(s.replace(old, old + '\treq.Header.Set("Idempotency-Key", "create:"+serial)\n', 1))

p = ROOT / "integration/c9_pressure_mysql_test.go"
s = p.read_text().replace('\t"yunka.io/framework/event/outbox"\n\t"yunka.io/framework/operation"\n', '\t"yunka.io/framework/event/outbox"\n\t"yunka.io/framework/execution"\n\t"yunka.io/framework/operation"\n\t"yunka.io/framework/requestscope"\n\t"yunka.io/framework/workflow/saga"\n')
s = s.replace('func pressureExecutor(t *testing.T, accessStore *accesspersistence.Store, observer operation.Observer, operationIDs ...string) operation.Executor {', 'func pressureExecutor(t *testing.T, database *gorm.DB, accessStore *accesspersistence.Store, observer operation.Observer, operationIDs ...string) operation.Executor {')
s = s.replace('\treturn operation.NewExecutor(security, observer)\n}', '''\ttransactions, err := requestscope.NewGORMExecutionFactory(database)
\tif err != nil { t.Fatal(err) }
\tidempotency, err := execution.NewIdempotencyCoordinator(execution.NewMemoryIdempotencyStore())
\tif err != nil { t.Fatal(err) }
\treturn operation.NewExecutorWithOptions(security, operation.ExecutorOptions{Transactions: transactions, Idempotency: idempotency}, observer)
}''', 1)
s = s.replace('devicepersistence.NewLocalTransferScopeFactory(db)', 'devicepersistence.NewLocalTransferRepositoryFactory(db)')
s = s.replace('devicepersistence.NewScopedScopeFactory(db)', 'devicepersistence.NewScopedRepositoryFactory(db)')
s = s.replace('pressureExecutor(t, accessStore, observer,', 'pressureExecutor(t, db, accessStore, observer,')
s = s.replace('service, err := deviceapp.NewProvisioningService(scopes, store)', 'stager, err := saga.NewStager(store)\n\tif err != nil { t.Fatal(err) }\n\tservice, err := deviceapp.NewProvisioningService(scopes, stager)')
s = s.replace('failingService, err := deviceapp.NewProvisioningService(scopes, failingTransactionalOutbox{err: failure})', 'failingStager, err := saga.NewStager(failingTransactionalOutbox{err: failure})\n\tif err != nil { t.Fatal(err) }\n\tfailingService, err := deviceapp.NewProvisioningService(scopes, failingStager)')
s = s.replace('response, err := operation.ExecuteTyped(ownerContext, executor, devicepolicy.RemoteProvisionPressurePlan()', 'response, err := operation.ExecuteTyped(execution.WithIdempotencyKey(ownerContext, "provision:"+serial), executor, devicepolicy.RemoteProvisionPressurePlan()')
s = s.replace('_, err = operation.ExecuteTyped(ownerContext, executor, devicepolicy.RemoteProvisionPressurePlan(), &deviceopsv1.CreateDeviceRequest{SiteId: site, Name: "Rollback", Serial: failedSerial}', '_, err = operation.ExecuteTyped(execution.WithIdempotencyKey(ownerContext, "provision:"+failedSerial), executor, devicepolicy.RemoteProvisionPressurePlan(), &deviceopsv1.CreateDeviceRequest{SiteId: site, Name: "Rollback", Serial: failedSerial}')
p.write_text(s)

# Source conformance follows OperationPlan v2 and bans Application tx plumbing.
p = ROOT / ".github/workflows/c9-pressure.yml"
s = p.read_text().replace("assert data['schemaVersion'] == 1", "assert data['schemaVersion'] == 2")
s = s.replace("              assert item['bindings']['rpc'].startswith('/deviceops.v1.DeviceApplication/')", "              assert item['bindings']['rpc'].startswith('/deviceops.v1.DeviceApplication/')\n              assert item['execution']['transaction'] in ('read_only','local')\n          commands = {item['operationId']: item['execution']['idempotency'] for item in data['operations']}\n          assert commands['device.create'] == 'required'\n          assert commands['device.update'] == 'required'\n          assert commands['device.delete'] == 'required'")
anchor = "          grep -q 'FP-C9-004' docs/pressure/C9-framework-pressure.md\n"
s = s.replace(anchor, anchor + "          ! grep -R -n 'requestscope.Execute' internal/deviceops/application\n          ! grep -R -n 'requestscope.GORMFrom' internal/deviceops/application\n          ! grep -R -n 'gorm.io/gorm' internal/deviceops/application\n          ! grep -R -n 'TransactionalStore' internal/deviceops/application\n          grep -R -q 'requestscope.Join' internal/deviceops/application\n          grep -q 'service.stager.Stage' internal/deviceops/application/remote_saga.go\n")
p.write_text(s)

# Pressure ledger records the escapes actually removed by C9.7.
p = ROOT / "docs/pressure/C9-framework-pressure.md"
s = p.read_text()
s = s.replace('**Classification:** P0 framework-mechanism pressure for the stated Yunka goal that Application should own use-case/business logic rather than transaction plumbing.', '**C9.7 status:** closed by `saga.Stager` joining the root `ExecutionScope`; Application no longer obtains GORM or a transaction handle.\n\n**Classification:** P0 framework-mechanism pressure for the stated Yunka goal that Application should own use-case/business logic rather than transaction plumbing.')
s = s.replace('**Classification:** P0 repeated pressure and direct input to C9.7.', '**C9.7 status:** closed by explicit PB transaction policy + Executor-owned `ExecutionScope`; Application uses `requestscope.Join*` only.\n\n**Classification:** P0 repeated pressure and direct input to C9.7.')
s = s.replace('**Classification:** P0 repeated command-safety pressure and direct C9.7 candidate.', '**C9.7 status:** mechanism closed for duplicate suppression with explicit Operation policy; response replay remains intentionally outside this pressure contract.\n\n**Classification:** P0 repeated command-safety pressure and direct C9.7 candidate.')
p.write_text(s)

print('C9.7 real-biz migration staged')
