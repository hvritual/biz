// Package featurecatalog owns customer-facing commercial features and their
// lifecycle. It deliberately references technical modules by stable code and
// never duplicates capability definitions or entitlement resolution.
package featurecatalog

import (
	"errors"
	"sort"
	"strings"

	"github.com/hvritual/biz/internal/commercial/domain/plan"
)

type ProductState string

const (
	ProductDraft      ProductState = "DRAFT"
	ProductPilot      ProductState = "PILOT"
	ProductPublished  ProductState = "PUBLISHED"
	ProductDeprecated ProductState = "DEPRECATED"
	ProductEOL        ProductState = "EOL"
)

type SalesState string

const (
	SalesStopped  SalesState = "STOP_SELL"
	SalesSellable SalesState = "SELLABLE"
)

type RuntimeState string

const (
	RuntimeContinuing RuntimeState = "CONTINUING"
	RuntimeRestricted RuntimeState = "RESTRICTED"
	RuntimeStopped    RuntimeState = "STOPPED"
)

type MigrationState string

const (
	MigrationNone        MigrationState = "NONE"
	MigrationRecommended MigrationState = "RECOMMENDED"
	MigrationRequired    MigrationState = "REQUIRED"
	MigrationComplete    MigrationState = "COMPLETE"
)

var (
	ErrInvalid       = errors.New("commercial feature: invalid definition")
	ErrTransition    = errors.New("commercial feature: invalid lifecycle transition")
	ErrReferences    = errors.New("commercial feature: active references prevent retirement")
	ErrMigrationOpen = errors.New("commercial feature: migration is incomplete")
)

type ModuleReference struct {
	ModuleCode      string
	CapabilityCodes []string
}

type Definition struct {
	Code       string
	Name       string
	ModuleRefs []ModuleReference
}

type SunsetPlan struct {
	ReplacementCode string
	Migration       MigrationState
}

// ReferenceImpact is supplied by the persistence/query adapter at retirement
// time. It keeps Plan, subscription and add-on facts in their own authorities.
type ReferenceImpact struct {
	PublishedPlans      uint64
	AddOns              uint64
	ActiveSubscriptions uint64
	EntitlementSources  uint64
}

func (r ReferenceImpact) HasActiveReferences() bool {
	return r.PublishedPlans != 0 || r.AddOns != 0 || r.ActiveSubscriptions != 0 || r.EntitlementSources != 0
}

type Feature struct {
	Code            string
	Name            string
	Version         uint64
	ModuleRefs      []ModuleReference
	Product         ProductState
	Sales           SalesState
	Runtime         RuntimeState
	Migration       MigrationState
	ReplacementCode string
}

func New(definition Definition) (Feature, error) {
	feature := Feature{
		Code:       strings.TrimSpace(definition.Code),
		Name:       strings.TrimSpace(definition.Name),
		ModuleRefs: canonicalReferences(definition.ModuleRefs),
		Product:    ProductDraft,
		Sales:      SalesStopped,
		Runtime:    RuntimeContinuing,
		Migration:  MigrationNone,
	}
	if err := feature.Validate(); err != nil {
		return Feature{}, err
	}
	return feature, nil
}

func (f Feature) Validate() error {
	if !plan.Code(f.Code) || f.Name == "" || len(f.Name) > 160 || len(f.ModuleRefs) == 0 {
		return ErrInvalid
	}
	seenModules := map[string]bool{}
	for _, reference := range f.ModuleRefs {
		if !plan.Code(reference.ModuleCode) || seenModules[reference.ModuleCode] || len(reference.CapabilityCodes) == 0 {
			return ErrInvalid
		}
		seenModules[reference.ModuleCode] = true
		seenCapabilities := map[string]bool{}
		for _, capability := range reference.CapabilityCodes {
			if !plan.Code(capability) || seenCapabilities[capability] {
				return ErrInvalid
			}
			seenCapabilities[capability] = true
		}
	}
	if !validProductState(f.Product) || !validSalesState(f.Sales) || !validRuntimeState(f.Runtime) || !validMigrationState(f.Migration) {
		return ErrInvalid
	}
	if f.Product == ProductEOL && f.Runtime != RuntimeStopped {
		return ErrInvalid
	}
	if f.Migration == MigrationComplete && f.ReplacementCode == "" {
		return ErrInvalid
	}
	return nil
}

func validProductState(state ProductState) bool {
	return state == ProductDraft || state == ProductPilot || state == ProductPublished || state == ProductDeprecated || state == ProductEOL
}

func validSalesState(state SalesState) bool {
	return state == SalesStopped || state == SalesSellable
}

func validRuntimeState(state RuntimeState) bool {
	return state == RuntimeContinuing || state == RuntimeRestricted || state == RuntimeStopped
}

func validMigrationState(state MigrationState) bool {
	return state == MigrationNone || state == MigrationRecommended || state == MigrationRequired || state == MigrationComplete
}

func (f Feature) Publish() (Feature, error) {
	if f.Product != ProductDraft && f.Product != ProductPilot {
		return Feature{}, ErrTransition
	}
	f.Product = ProductPublished
	f.Sales = SalesSellable
	return f, f.Validate()
}

func (f Feature) StopSell() (Feature, error) {
	if f.Product != ProductPublished && f.Product != ProductDeprecated {
		return Feature{}, ErrTransition
	}
	f.Sales = SalesStopped
	// Stop-sell intentionally leaves Runtime unchanged: it cannot revoke an
	// already-issued entitlement or silently disable active tenants.
	return f, f.Validate()
}

func (f Feature) PlanSunset(sunset SunsetPlan) (Feature, error) {
	if f.Product != ProductPublished && f.Product != ProductDeprecated {
		return Feature{}, ErrTransition
	}
	sunset.ReplacementCode = strings.TrimSpace(sunset.ReplacementCode)
	if f.Sales != SalesStopped || !plan.Code(sunset.ReplacementCode) || sunset.ReplacementCode == f.Code || (sunset.Migration != MigrationRecommended && sunset.Migration != MigrationRequired) {
		return Feature{}, ErrTransition
	}
	f.Product = ProductDeprecated
	f.Migration = sunset.Migration
	f.ReplacementCode = sunset.ReplacementCode
	return f, f.Validate()
}

func (f Feature) CompleteMigration() (Feature, error) {
	if f.Product != ProductDeprecated || (f.Migration != MigrationRecommended && f.Migration != MigrationRequired) || f.ReplacementCode == "" {
		return Feature{}, ErrTransition
	}
	f.Migration = MigrationComplete
	return f, f.Validate()
}

func (f Feature) Retire(references ReferenceImpact) (Feature, error) {
	if f.Product != ProductDeprecated || f.Migration != MigrationComplete {
		return Feature{}, ErrMigrationOpen
	}
	if references.HasActiveReferences() {
		return Feature{}, ErrReferences
	}
	f.Product = ProductEOL
	f.Sales = SalesStopped
	f.Runtime = RuntimeStopped
	return f, f.Validate()
}

func canonicalReferences(input []ModuleReference) []ModuleReference {
	output := make([]ModuleReference, 0, len(input))
	for _, reference := range input {
		copy := ModuleReference{ModuleCode: strings.TrimSpace(reference.ModuleCode), CapabilityCodes: append([]string(nil), reference.CapabilityCodes...)}
		for index := range copy.CapabilityCodes {
			copy.CapabilityCodes[index] = strings.TrimSpace(copy.CapabilityCodes[index])
		}
		sort.Strings(copy.CapabilityCodes)
		output = append(output, copy)
	}
	sort.Slice(output, func(left, right int) bool { return output[left].ModuleCode < output[right].ModuleCode })
	return output
}
