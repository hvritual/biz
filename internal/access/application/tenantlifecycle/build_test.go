package tenantlifecycle_test

import (
	"context"
	"reflect"
	"testing"
	"unicode"
	"unicode/utf8"

	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/access/application/tenantlifecycle"
	"github.com/hvritual/biz/internal/access/ports"
	"yunka.io/framework/operation"
	"yunka.io/framework/requestscope"
)

type capabilities struct {
	accessapp.TenantLifecycleCapabilities
}
type wideMember struct {
	accessapp.TenantMemberLifecycleApplication
}

func (*wideMember) ExtraAuthority() {}

type wideRole struct {
	accessapp.TenantRolePermissionApplication
}

func (*wideRole) ExtraAuthority() {}

func TestAG02BuildExportsOnlyDeclaredApplication(t *testing.T) {
	calls := 0
	repositories := requestscope.RepositoryFactory[ports.TenantRepositories](func(context.Context, requestscope.UnitOfWork) (ports.TenantRepositories, error) {
		calls++
		return ports.TenantRepositories{}, nil
	})
	app, err := tenantlifecycle.Build(repositories, capabilities{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("composition acquired request repositories")
	}
	actual := reflect.TypeOf(app)
	want := reflect.TypeOf((*accessapp.TenantLifecycleApplication)(nil)).Elem()
	assertExactMethods(t, actual, want)
	hidden := actual.Elem()
	first, _ := utf8.DecodeRuneInString(hidden.Name())
	if unicode.IsUpper(first) || hidden.PkgPath() != "github.com/hvritual/biz/internal/access/application/tenantlifecycle/internal/usecase" {
		t.Fatalf("implementation is not private inside the application: %v", hidden)
	}
	assertPrivateFields(t, hidden)
}

func TestAG02BuildPreservesRequiredDependencies(t *testing.T) {
	repositories := requestscope.RepositoryFactory[ports.TenantRepositories](func(context.Context, requestscope.UnitOfWork) (ports.TenantRepositories, error) {
		t.Fatal("constructor must not acquire request resources")
		return ports.TenantRepositories{}, nil
	})
	if app, err := tenantlifecycle.Build(nil, capabilities{}); err == nil || app != nil {
		t.Fatal("missing repositories accepted")
	}
	if app, err := tenantlifecycle.Build(repositories, nil); err == nil || app != nil {
		t.Fatal("missing capabilities accepted")
	}
}

func TestAG02GeneratedChildrenKeepActualMethodSetNarrow(t *testing.T) {
	// Wide fakes are deliberately supplied at the composition boundary. No
	// business call is made here; real child execution is covered by MySQL B12.5.
	executor := operation.NewExecutor(nil)
	members, err := accessapp.NewTenantLifecycleToAccessTenantMemberLifecycleChildCapability(&wideMember{}, executor)
	if err != nil {
		t.Fatal(err)
	}
	roles, err := accessapp.NewTenantLifecycleToAccessTenantRolePermissionChildCapability(&wideRole{}, executor)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		value    any
		contract reflect.Type
	}{
		{members, reflect.TypeOf((*accessapp.TenantLifecycleToAccessTenantMemberLifecycleChildCapability)(nil)).Elem()},
		{roles, reflect.TypeOf((*accessapp.TenantLifecycleToAccessTenantRolePermissionChildCapability)(nil)).Elem()},
	} {
		actual := reflect.TypeOf(tc.value)
		assertExactMethods(t, actual, tc.contract)
		assertPrivateFields(t, actual.Elem())
		if _, ok := tc.value.(interface{ ExtraAuthority() }); ok {
			t.Fatal("target authority leaked through child wrapper")
		}
	}
}

func assertExactMethods(t *testing.T, actual, contract reflect.Type) {
	t.Helper()
	if actual.NumMethod() != contract.NumMethod() {
		t.Fatalf("%v exposes %d methods; contract has %d", actual, actual.NumMethod(), contract.NumMethod())
	}
	for i := 0; i < actual.NumMethod(); i++ {
		if _, ok := contract.MethodByName(actual.Method(i).Name); !ok {
			t.Fatalf("undeclared method: %s", actual.Method(i).Name)
		}
	}
}
func assertPrivateFields(t *testing.T, actual reflect.Type) {
	t.Helper()
	for i := 0; i < actual.NumField(); i++ {
		field := actual.Field(i)
		if field.IsExported() || field.Anonymous {
			t.Fatalf("implementation field is exposed or embedded: %v", field)
		}
	}
}
