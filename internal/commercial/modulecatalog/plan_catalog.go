package modulecatalog

import (
	"context"
	"github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/requestscope"
)

// ListPlanCatalogInScope takes the existing catalog admission lock and current
// reads inside the caller's root UoW. No stale RR image can publish an offer.
func (s *Service) ListPlanCatalogInScope(ctx context.Context) ([]Module, error) {
	factory := requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (catalogReadRepositories, error) {
		return catalogReadRepositories{db: tx}, nil
	})
	return requestscope.JoinValue(ctx, factory, func(scope *requestscope.View[catalogReadRepositories]) ([]Module, error) {
		db := scope.Repositories().db.WithContext(scope.Context())
		if _, err := consistency.LockCatalog(db, false); err != nil {
			return nil, err
		}
		var rows []moduleRow
		if err := db.Clauses(clause.Locking{Strength: "SHARE"}).Order("module_code").Find(&rows).Error; err != nil {
			return nil, err
		}
		out := make([]Module, 0, len(rows))
		for _, r := range rows {
			d, ok := s.registry.Definition(r.Code)
			if !ok {
				return nil, ErrUnknownDefinition
			}
			m := rowToModule(r, d)
			if !d.ImplementationReady {
				m.TechnicalStatus = TechnicalNotReady
			}
			out = append(out, m)
		}
		return out, nil
	})
}
