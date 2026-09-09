package modulecatalog

import (
	"context"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"yunka.io/framework/execution"
)

// ReadCurrentCatalog is the owner-defined preflight projection. Unlike the
// display list it rejects stale registry references and enforces technical readiness.
func (s *Service) ReadCurrentCatalog(ctx context.Context) (entitlement.Catalog, error) {
	if _, ok := execution.Current(ctx); ok {
		modules, err := s.ListInScope(ctx)
		if err != nil {
			return nil, err
		}
		result := make(entitlement.Catalog, 0, len(modules))
		for _, m := range modules {
			result = append(result, entitlement.ModuleDefinition{Code: m.Code, TechnicalStatus: string(m.TechnicalStatus), SalesStatus: string(m.SalesStatus), Version: m.Version, Capabilities: m.CapabilityCodes, QuotaKeys: m.QuotaSchemaKeys, FieldKeys: m.FieldPolicySchemaKeys, Dependencies: m.Dependencies})
		}
		return result, result.Validate()
	}
	var rows []moduleRow
	if err := s.store.db.WithContext(ctx).Order("module_code").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(entitlement.Catalog, 0, len(rows))
	for _, row := range rows {
		def, ok := s.registry.Definition(row.Code)
		if !ok {
			return nil, ErrUnknownDefinition
		}
		status := row.TechnicalStatus
		if !def.ImplementationReady {
			status = string(TechnicalNotReady)
		}
		out = append(out, entitlement.ModuleDefinition{Code: row.Code, TechnicalStatus: status, SalesStatus: row.SalesStatus, Version: row.Version, Capabilities: append([]string(nil), def.CapabilityCodes...), QuotaKeys: append([]string(nil), def.QuotaSchemaKeys...), FieldKeys: append([]string(nil), def.FieldPolicySchemaKeys...), Dependencies: append([]string(nil), def.Dependencies...)})
	}
	return out, out.Validate()
}
