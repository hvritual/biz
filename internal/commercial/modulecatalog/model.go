package modulecatalog

import (
	"errors"
	"sort"
	"strings"
	"time"
)

type TechnicalStatus string
const (
	TechnicalNotReady TechnicalStatus = "not_ready"
	TechnicalReady TechnicalStatus = "ready"
	TechnicalDisabled TechnicalStatus = "disabled"
)

type SalesStatus string
const (
	SalesSellable SalesStatus = "sellable"
	SalesRetired SalesStatus = "retired"
)

var (
	ErrNotFound = errors.New("module catalog: not found")
	ErrConflict = errors.New("module catalog: version conflict")
	ErrUnknownDefinition = errors.New("module catalog: module is not declared by code")
	ErrImplementationUnavailable = errors.New("module catalog: technical implementation is unavailable")
	ErrDependencyMissing = errors.New("module catalog: dependency missing")
	ErrDependencyCycle = errors.New("module catalog: dependency cycle")
	ErrReferenced = errors.New("module catalog: module is referenced")
	ErrCodeRetired = errors.New("module catalog: retired module code cannot be reused")
	ErrPlatformPrincipalRequired = errors.New("module catalog: tenantless authenticated platform principal required")
	ErrInvalidRequest = errors.New("module catalog: invalid request")
)

type Definition struct {
	Code string
	CapabilityCodes []string
	QuotaSchemaKeys []string
	FieldPolicySchemaKeys []string
	Dependencies []string
	ImplementationReady bool
}

type Registry struct{ definitions map[string]Definition }

func NewRegistry(definitions []Definition) (Registry, error) {
	r := Registry{definitions: map[string]Definition{}}
	for _, d := range definitions {
		d.Code = strings.TrimSpace(d.Code)
		if d.Code == "" || r.definitions[d.Code].Code != "" { return Registry{}, ErrInvalidRequest }
		d.CapabilityCodes = normalized(d.CapabilityCodes)
		d.QuotaSchemaKeys = normalized(d.QuotaSchemaKeys)
		d.FieldPolicySchemaKeys = normalized(d.FieldPolicySchemaKeys)
		d.Dependencies = normalized(d.Dependencies)
		r.definitions[d.Code] = d
	}
	if err := r.Validate(); err != nil { return Registry{}, err }
	return r, nil
}

func ProductionRegistry() Registry {
	r, err := NewRegistry([]Definition{
		{Code:"access-management", CapabilityCodes:[]string{"tenant.lifecycle","tenant.member.lifecycle","tenant.role.permission"}, QuotaSchemaKeys:[]string{"tenant.members"}, FieldPolicySchemaKeys:[]string{"member.profile"}, ImplementationReady:true},
		{Code:"device-operations", CapabilityCodes:[]string{"device.lifecycle","device.transfer"}, QuotaSchemaKeys:[]string{"tenant.devices"}, FieldPolicySchemaKeys:[]string{"device.identity"}, ImplementationReady:true},
	})
	if err != nil { panic(err) }
	return r
}

func (r Registry) Definition(code string) (Definition, bool) { d, ok := r.definitions[code]; return d, ok }
func (r Registry) Validate() error {
	for code, d := range r.definitions {
		for _, dep := range d.Dependencies { if dep == code { return ErrDependencyCycle }; if _, ok := r.definitions[dep]; !ok { return ErrDependencyMissing } }
	}
	state := map[string]uint8{}
	var visit func(string) error
	visit = func(code string) error {
		if state[code] == 1 { return ErrDependencyCycle }
		if state[code] == 2 { return nil }
		state[code] = 1
		for _, dep := range r.definitions[code].Dependencies { if err := visit(dep); err != nil { return err } }
		state[code] = 2; return nil
	}
	for code := range r.definitions { if err := visit(code); err != nil { return err } }
	return nil
}

type Module struct {
	Code string
	Name string
	Category string
	SalesScope []string
	TechnicalStatus TechnicalStatus
	SalesStatus SalesStatus
	CapabilityCodes []string
	QuotaSchemaKeys []string
	FieldPolicySchemaKeys []string
	Dependencies []string
	Version uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func normalized(values []string) []string { set:=map[string]struct{}{}; for _,v:=range values { v=strings.TrimSpace(v); if v!="" { set[v]=struct{}{} } }; out:=make([]string,0,len(set)); for v:=range set { out=append(out,v) }; sort.Strings(out); return out }
