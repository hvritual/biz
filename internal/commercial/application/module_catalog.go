package application

import (
	"context"
	"errors"
	"strings"

	commercialv1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	"github.com/hvritual/biz/internal/commercial/featurecatalog"
	"github.com/hvritual/biz/internal/commercial/modulecatalog"
	"yunka.io/framework/core/identity"
)

type ModuleCatalogService struct {
	catalog  *modulecatalog.Service
	features *featurecatalog.Service
}

func NewModuleCatalogService(catalog *modulecatalog.Service, features *featurecatalog.Service) (*ModuleCatalogService, error) {
	if catalog == nil || features == nil {
		return nil, errors.New("commercial application: module catalog and feature lifecycle required")
	}
	return &ModuleCatalogService{catalog: catalog, features: features}, nil
}

func requirePlatform(ctx context.Context) error {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.Subject == "" || p.TenantID != "" {
		return modulecatalog.ErrPlatformPrincipalRequired
	}
	return nil
}

func (s *ModuleCatalogService) CreateModule(ctx context.Context, req *commercialv1.CreateModuleRequest) (*commercialv1.ModuleDTO, error) {
	if req == nil {
		return nil, modulecatalog.ErrInvalidRequest
	}
	m, err := s.catalog.Create(ctx, modulecatalog.CreateCommand{RequestID: req.RequestId, Code: req.ModuleCode, Name: req.Name, Category: req.Category, SalesScope: req.SalesScope, Reason: req.Reason})
	if err != nil {
		return nil, err
	}
	return toDTO(m), nil
}
func (s *ModuleCatalogService) GetModule(ctx context.Context, req *commercialv1.GetModuleRequest) (*commercialv1.ModuleDTO, error) {
	if req == nil {
		return nil, modulecatalog.ErrInvalidRequest
	}
	if err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	m, err := s.catalog.Get(ctx, req.ModuleCode)
	if err != nil {
		return nil, err
	}
	return toDTO(m), nil
}
func (s *ModuleCatalogService) ListModules(ctx context.Context, _ *commercialv1.ListModulesRequest) (*commercialv1.ListModulesResponse, error) {
	if err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	items, err := s.catalog.List(ctx)
	if err != nil {
		return nil, err
	}
	out := &commercialv1.ListModulesResponse{Modules: make([]*commercialv1.ModuleDTO, 0, len(items))}
	for _, m := range items {
		out.Modules = append(out.Modules, toDTO(m))
	}
	return out, nil
}
func (s *ModuleCatalogService) UpdateModule(ctx context.Context, req *commercialv1.UpdateModuleRequest) (*commercialv1.ModuleDTO, error) {
	if req == nil {
		return nil, modulecatalog.ErrInvalidRequest
	}
	m, err := s.catalog.Update(ctx, modulecatalog.UpdateCommand{RequestID: req.RequestId, Code: req.ModuleCode, Name: req.Name, Category: req.Category, SalesScope: req.SalesScope, Version: req.Version, Reason: req.Reason})
	if err != nil {
		return nil, err
	}
	return toDTO(m), nil
}
func (s *ModuleCatalogService) SetModuleSalesStatus(ctx context.Context, req *commercialv1.SetModuleSalesStatusRequest) (*commercialv1.ModuleDTO, error) {
	if req == nil {
		return nil, modulecatalog.ErrInvalidRequest
	}
	status, err := salesFromPB(req.SalesStatus)
	if err != nil {
		return nil, err
	}
	m, err := s.catalog.SetSalesStatus(ctx, modulecatalog.StatusCommand{RequestID: req.RequestId, Code: req.ModuleCode, Version: req.Version, Reason: req.Reason, Sales: status})
	if err != nil {
		return nil, err
	}
	return toDTO(m), nil
}
func (s *ModuleCatalogService) SetModuleTechnicalStatus(ctx context.Context, req *commercialv1.SetModuleTechnicalStatusRequest) (*commercialv1.ModuleDTO, error) {
	if req == nil {
		return nil, modulecatalog.ErrInvalidRequest
	}
	status, err := technicalFromPB(req.TechnicalStatus)
	if err != nil {
		return nil, err
	}
	m, err := s.catalog.SetTechnicalStatus(ctx, modulecatalog.StatusCommand{RequestID: req.RequestId, Code: req.ModuleCode, Version: req.Version, Reason: req.Reason, Technical: status})
	if err != nil {
		return nil, err
	}
	return toDTO(m), nil
}
func (s *ModuleCatalogService) DeleteModule(ctx context.Context, req *commercialv1.DeleteModuleRequest) (*commercialv1.DeleteModuleResponse, error) {
	if req == nil {
		return nil, modulecatalog.ErrInvalidRequest
	}
	if err := s.catalog.Delete(ctx, modulecatalog.DeleteCommand{RequestID: req.RequestId, Code: req.ModuleCode, Version: req.Version, Reason: req.Reason}); err != nil {
		return nil, err
	}
	return &commercialv1.DeleteModuleResponse{ModuleCode: req.ModuleCode, Deleted: true}, nil
}
func (s *ModuleCatalogService) RecordModuleRuntimeVerification(ctx context.Context, req *commercialv1.RecordModuleRuntimeVerificationRequest) (*commercialv1.ModuleDTO, error) {
	if req == nil {
		return nil, modulecatalog.ErrInvalidRequest
	}
	if err := s.catalog.RecordRuntimeVerification(ctx, modulecatalog.RuntimeVerificationCommand{ModuleCode: req.ModuleCode, ModuleVersion: req.ModuleVersion, EvidenceDigest: req.EvidenceDigest, SourceTree: req.SourceTree}); err != nil {
		return nil, err
	}
	m, err := s.catalog.Get(ctx, req.ModuleCode)
	if err != nil {
		return nil, err
	}
	return toDTO(m), nil
}

func (s *ModuleCatalogService) CreateCommercialFeature(ctx context.Context, req *commercialv1.CreateCommercialFeatureRequest) (*commercialv1.CommercialFeatureDTO, error) {
	if req == nil || strings.TrimSpace(req.FeatureCode) == "" || strings.TrimSpace(req.Name) == "" {
		return nil, featurecatalog.ErrInvalid
	}
	definition := featurecatalog.Definition{Code: req.FeatureCode, Name: req.Name, ModuleRefs: featureRefsFromDTO(req.ModuleRefs)}
	if err := s.validateFeatureReferences(ctx, definition.ModuleRefs); err != nil {
		return nil, err
	}
	feature, err := s.features.Create(ctx, definition, featurecatalog.Command{RequestID: req.RequestId, Reason: req.Reason})
	if err != nil {
		return nil, err
	}
	return s.featureDTO(ctx, feature)
}

func (s *ModuleCatalogService) GetCommercialFeature(ctx context.Context, req *commercialv1.GetCommercialFeatureRequest) (*commercialv1.CommercialFeatureDTO, error) {
	if req == nil || strings.TrimSpace(req.FeatureCode) == "" {
		return nil, featurecatalog.ErrInvalid
	}
	if err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	feature, err := s.features.Get(ctx, req.FeatureCode)
	if err != nil {
		return nil, err
	}
	return s.featureDTO(ctx, feature)
}

func (s *ModuleCatalogService) ListCommercialFeatures(ctx context.Context, _ *commercialv1.ListCommercialFeaturesRequest) (*commercialv1.ListCommercialFeaturesResponse, error) {
	if err := requirePlatform(ctx); err != nil {
		return nil, err
	}
	features, err := s.features.List(ctx)
	if err != nil {
		return nil, err
	}
	result := &commercialv1.ListCommercialFeaturesResponse{Features: make([]*commercialv1.CommercialFeatureDTO, 0, len(features))}
	for _, feature := range features {
		value, err := s.featureDTO(ctx, feature)
		if err != nil {
			return nil, err
		}
		result.Features = append(result.Features, value)
	}
	return result, nil
}

func (s *ModuleCatalogService) PublishCommercialFeature(ctx context.Context, req *commercialv1.CommercialFeatureLifecycleRequest) (*commercialv1.CommercialFeatureDTO, error) {
	feature, err := s.featureForLifecycle(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := s.validateFeatureReferences(ctx, feature.ModuleRefs); err != nil {
		return nil, err
	}
	feature, err = s.features.Publish(ctx, feature.Code, featureCommand(req))
	if err != nil {
		return nil, err
	}
	return s.featureDTO(ctx, feature)
}

func (s *ModuleCatalogService) StopSellingCommercialFeature(ctx context.Context, req *commercialv1.CommercialFeatureLifecycleRequest) (*commercialv1.CommercialFeatureDTO, error) {
	feature, err := s.featureForLifecycle(ctx, req)
	if err != nil {
		return nil, err
	}
	feature, err = s.features.StopSell(ctx, feature.Code, featureCommand(req))
	if err != nil {
		return nil, err
	}
	return s.featureDTO(ctx, feature)
}

func (s *ModuleCatalogService) PlanCommercialFeatureSunset(ctx context.Context, req *commercialv1.CommercialFeatureLifecycleRequest) (*commercialv1.CommercialFeatureDTO, error) {
	feature, err := s.featureForLifecycle(ctx, req)
	if err != nil {
		return nil, err
	}
	replacement, err := s.features.Get(ctx, req.ReplacementCode)
	if err != nil || replacement.Code == feature.Code {
		return nil, featurecatalog.ErrInvalid
	}
	migration := featurecatalog.MigrationState(strings.TrimSpace(req.MigrationState))
	feature, err = s.features.PlanSunset(ctx, feature.Code, featurecatalog.SunsetPlan{ReplacementCode: replacement.Code, Migration: migration}, featureCommand(req))
	if err != nil {
		return nil, err
	}
	return s.featureDTO(ctx, feature)
}

func (s *ModuleCatalogService) CompleteCommercialFeatureMigration(ctx context.Context, req *commercialv1.CommercialFeatureLifecycleRequest) (*commercialv1.CommercialFeatureDTO, error) {
	feature, err := s.featureForLifecycle(ctx, req)
	if err != nil {
		return nil, err
	}
	feature, err = s.features.CompleteMigration(ctx, feature.Code, featureCommand(req))
	if err != nil {
		return nil, err
	}
	return s.featureDTO(ctx, feature)
}

func (s *ModuleCatalogService) RetireCommercialFeature(ctx context.Context, req *commercialv1.CommercialFeatureLifecycleRequest) (*commercialv1.CommercialFeatureDTO, error) {
	feature, err := s.featureForLifecycle(ctx, req)
	if err != nil {
		return nil, err
	}
	feature, err = s.features.Retire(ctx, feature.Code, featureCommand(req))
	if err != nil {
		return nil, err
	}
	return s.featureDTO(ctx, feature)
}

func featureCommand(req *commercialv1.CommercialFeatureLifecycleRequest) featurecatalog.Command {
	return featurecatalog.Command{RequestID: req.RequestId, Version: req.Version, Reason: req.Reason}
}

func (s *ModuleCatalogService) featureForLifecycle(ctx context.Context, req *commercialv1.CommercialFeatureLifecycleRequest) (featurecatalog.Feature, error) {
	if req == nil || strings.TrimSpace(req.FeatureCode) == "" || req.Version == 0 {
		return featurecatalog.Feature{}, featurecatalog.ErrInvalid
	}
	if err := requirePlatform(ctx); err != nil {
		return featurecatalog.Feature{}, err
	}
	return s.features.Get(ctx, req.FeatureCode)
}

func featureRefsFromDTO(values []*commercialv1.CommercialFeatureModuleReference) []featurecatalog.ModuleReference {
	refs := make([]featurecatalog.ModuleReference, 0, len(values))
	for _, value := range values {
		if value != nil {
			refs = append(refs, featurecatalog.ModuleReference{ModuleCode: value.ModuleCode, CapabilityCodes: append([]string(nil), value.CapabilityCodes...)})
		}
	}
	return refs
}

func (s *ModuleCatalogService) validateFeatureReferences(ctx context.Context, refs []featurecatalog.ModuleReference) error {
	for _, ref := range refs {
		module, err := s.catalog.Get(ctx, ref.ModuleCode)
		if err != nil || module.TechnicalStatus != modulecatalog.TechnicalReady || module.SalesStatus != modulecatalog.SalesSellable {
			return featurecatalog.ErrInvalid
		}
		available := map[string]bool{}
		for _, capability := range module.CapabilityCodes {
			available[capability] = true
		}
		for _, capability := range ref.CapabilityCodes {
			if !available[capability] {
				return featurecatalog.ErrInvalid
			}
		}
	}
	return nil
}

func (s *ModuleCatalogService) featureDTO(ctx context.Context, feature featurecatalog.Feature) (*commercialv1.CommercialFeatureDTO, error) {
	impact, err := s.features.ReferenceImpact(ctx, feature.Code)
	if err != nil {
		return nil, err
	}
	refs := make([]*commercialv1.CommercialFeatureModuleReference, 0, len(feature.ModuleRefs))
	for _, ref := range feature.ModuleRefs {
		refs = append(refs, &commercialv1.CommercialFeatureModuleReference{ModuleCode: ref.ModuleCode, CapabilityCodes: append([]string(nil), ref.CapabilityCodes...)})
	}
	return &commercialv1.CommercialFeatureDTO{FeatureCode: feature.Code, Name: feature.Name, Version: feature.Version, ModuleRefs: refs, ProductState: string(feature.Product), SalesState: string(feature.Sales), RuntimeState: string(feature.Runtime), MigrationState: string(feature.Migration), ReplacementCode: feature.ReplacementCode, ReferenceImpact: &commercialv1.CommercialFeatureReferenceImpact{PublishedPlans: impact.PublishedPlans, AddOns: impact.AddOns, ActiveSubscriptions: impact.ActiveSubscriptions, EntitlementSources: impact.EntitlementSources}}, nil
}

func toDTO(m modulecatalog.Module) *commercialv1.ModuleDTO {
	return &commercialv1.ModuleDTO{ModuleCode: m.Code, Name: m.Name, Category: m.Category, SalesScope: append([]string(nil), m.SalesScope...), TechnicalStatus: technicalToPB(m.TechnicalStatus), SalesStatus: salesToPB(m.SalesStatus), CapabilityCodes: append([]string(nil), m.CapabilityCodes...), QuotaSchemaKeys: append([]string(nil), m.QuotaSchemaKeys...), FieldPolicySchemaKeys: append([]string(nil), m.FieldPolicySchemaKeys...), Dependencies: append([]string(nil), m.Dependencies...), Version: m.Version}
}
func technicalToPB(v modulecatalog.TechnicalStatus) commercialv1.ModuleTechnicalStatus {
	switch v {
	case modulecatalog.TechnicalReady:
		return commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY
	case modulecatalog.TechnicalDisabled:
		return commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_DISABLED
	default:
		return commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_NOT_READY
	}
}
func salesToPB(v modulecatalog.SalesStatus) commercialv1.ModuleSalesStatus {
	if v == modulecatalog.SalesRetired {
		return commercialv1.ModuleSalesStatus_MODULE_SALES_STATUS_RETIRED
	}
	return commercialv1.ModuleSalesStatus_MODULE_SALES_STATUS_SELLABLE
}
func technicalFromPB(v commercialv1.ModuleTechnicalStatus) (modulecatalog.TechnicalStatus, error) {
	switch v {
	case commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_NOT_READY:
		return modulecatalog.TechnicalNotReady, nil
	case commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_READY:
		return modulecatalog.TechnicalReady, nil
	case commercialv1.ModuleTechnicalStatus_MODULE_TECHNICAL_STATUS_DISABLED:
		return modulecatalog.TechnicalDisabled, nil
	default:
		return "", modulecatalog.ErrInvalidRequest
	}
}
func salesFromPB(v commercialv1.ModuleSalesStatus) (modulecatalog.SalesStatus, error) {
	switch v {
	case commercialv1.ModuleSalesStatus_MODULE_SALES_STATUS_SELLABLE:
		return modulecatalog.SalesSellable, nil
	case commercialv1.ModuleSalesStatus_MODULE_SALES_STATUS_RETIRED:
		return modulecatalog.SalesRetired, nil
	default:
		return "", modulecatalog.ErrInvalidRequest
	}
}
