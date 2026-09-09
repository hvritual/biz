package modulecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
	"gorm.io/gorm"
	"yunka.io/framework/core/identity"
)

type Service struct {
	store    *Store
	registry Registry
}

func NewService(store *Store, registry Registry) (*Service, error) {
	if store == nil {
		return nil, errors.New("module catalog: store required")
	}
	if err := registry.Validate(); err != nil {
		return nil, err
	}
	return &Service{store: store, registry: registry}, nil
}

type CreateCommand struct {
	RequestID, Code, Name, Category, Reason string
	SalesScope                              []string
}
type UpdateCommand struct {
	RequestID, Code, Name, Category, Reason string
	SalesScope                              []string
	Version                                 uint64
}
type StatusCommand struct {
	RequestID, Code, Reason string
	Version                 uint64
	Technical               TechnicalStatus
	Sales                   SalesStatus
}
type DeleteCommand struct {
	RequestID, Code, Reason string
	Version                 uint64
}

func platformActor(ctx context.Context) (string, error) {
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.Subject == "" || p.TenantID != "" {
		return "", ErrPlatformPrincipalRequired
	}
	return p.Subject, nil
}
func validWrite(requestID, reason string) bool {
	return strings.TrimSpace(requestID) != "" && strings.TrimSpace(reason) != ""
}

func (s *Service) Create(ctx context.Context, c CreateCommand) (Module, error) {
	actor, err := platformActor(ctx)
	if err != nil {
		return Module{}, err
	}
	if !validWrite(c.RequestID, c.Reason) || strings.TrimSpace(c.Code) == "" || strings.TrimSpace(c.Name) == "" {
		return Module{}, ErrInvalidRequest
	}
	def, ok := s.registry.Definition(c.Code)
	if !ok {
		return Module{}, ErrUnknownDefinition
	}
	var out Module
	err = s.transact(ctx, func(tx *gorm.DB) error {
		if m, ok, err := loadIdempotent(tx, c.RequestID, "create", c.Code); err != nil {
			return err
		} else if ok {
			out = m
			return nil
		}
		var retired int64
		if err := tx.Model(&retiredCodeRow{}).Where("module_code = ?", c.Code).Count(&retired).Error; err != nil {
			return err
		}
		if retired > 0 {
			return ErrCodeRetired
		}
		for _, dep := range def.Dependencies {
			var count int64
			if err := tx.Model(&moduleRow{}).Where("module_code = ?", dep).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrDependencyMissing
			}
		}
		now := time.Now().UTC()
		tech := TechnicalNotReady
		if def.ImplementationReady {
			tech = TechnicalReady
		}
		row := moduleRow{Code: c.Code, Name: c.Name, Category: c.Category, SalesScopeJSON: encode(normalized(c.SalesScope)), TechnicalStatus: string(tech), SalesStatus: string(SalesSellable), Version: 1, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		for _, dep := range def.Dependencies {
			if err := tx.Create(&dependencyRow{ModuleCode: c.Code, DependsOn: dep}).Error; err != nil {
				return err
			}
		}
		out = rowToModule(row, def)
		if err := writeAudit(tx, c.Code, actor, "create", nil, out, c.Reason, c.RequestID); err != nil {
			return err
		}
		return saveIdempotent(tx, c.RequestID, "create", c.Code, out)
	})
	return out, err
}

func (s *Service) Get(ctx context.Context, code string) (Module, error) {
	var row moduleRow
	if err := s.store.db.WithContext(ctx).Where("module_code = ?", code).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Module{}, ErrNotFound
		}
		return Module{}, err
	}
	def, ok := s.registry.Definition(code)
	if !ok {
		return Module{}, ErrUnknownDefinition
	}
	return rowToModule(row, def), nil
}
func (s *Service) List(ctx context.Context) ([]Module, error) {
	var rows []moduleRow
	if err := s.store.db.WithContext(ctx).Order("module_code ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Module, 0, len(rows))
	for _, r := range rows {
		if d, ok := s.registry.Definition(r.Code); ok {
			out = append(out, rowToModule(r, d))
		}
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, c UpdateCommand) (Module, error) {
	return s.mutate(ctx, c.RequestID, c.Code, c.Reason, c.Version, "update", func(row *moduleRow, def Definition) error {
		if strings.TrimSpace(c.Name) == "" {
			return ErrInvalidRequest
		}
		row.Name = c.Name
		row.Category = c.Category
		row.SalesScopeJSON = encode(normalized(c.SalesScope))
		return nil
	})
}
func (s *Service) SetSalesStatus(ctx context.Context, c StatusCommand) (Module, error) {
	if c.Sales != SalesSellable && c.Sales != SalesRetired {
		return Module{}, ErrInvalidRequest
	}
	return s.mutate(ctx, c.RequestID, c.Code, c.Reason, c.Version, "sales_status", func(row *moduleRow, _ Definition) error { row.SalesStatus = string(c.Sales); return nil })
}
func (s *Service) SetTechnicalStatus(ctx context.Context, c StatusCommand) (Module, error) {
	if c.Technical != TechnicalNotReady && c.Technical != TechnicalReady && c.Technical != TechnicalDisabled {
		return Module{}, ErrInvalidRequest
	}
	return s.mutate(ctx, c.RequestID, c.Code, c.Reason, c.Version, "technical_status", func(row *moduleRow, d Definition) error {
		if c.Technical == TechnicalReady && !d.ImplementationReady {
			return ErrImplementationUnavailable
		}
		row.TechnicalStatus = string(c.Technical)
		return nil
	})
}
func (s *Service) mutate(ctx context.Context, requestID, code, reason string, version uint64, op string, change func(*moduleRow, Definition) error) (Module, error) {
	actor, err := platformActor(ctx)
	if err != nil {
		return Module{}, err
	}
	if !validWrite(requestID, reason) || version == 0 {
		return Module{}, ErrInvalidRequest
	}
	def, ok := s.registry.Definition(code)
	if !ok {
		return Module{}, ErrUnknownDefinition
	}
	var out Module
	err = s.transact(ctx, func(tx *gorm.DB) error {
		if m, ok, err := loadIdempotent(tx, requestID, op, code); err != nil {
			return err
		} else if ok {
			out = m
			return nil
		}
		var row moduleRow
		if err := tx.Where("module_code = ?", code).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if row.Version != version {
			return ErrConflict
		}
		before := rowToModule(row, def)
		if err := change(&row, def); err != nil {
			return err
		}
		now := time.Now().UTC()
		res := tx.Model(&moduleRow{}).Where("module_code = ? AND version = ?", code, version).Updates(map[string]any{"name": row.Name, "category": row.Category, "sales_scope_json": row.SalesScopeJSON, "technical_status": row.TechnicalStatus, "sales_status": row.SalesStatus, "version": gorm.Expr("version + 1"), "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrConflict
		}
		row.Version = version + 1
		row.UpdatedAt = now
		out = rowToModule(row, def)
		if err := writeAudit(tx, code, actor, op, before, out, reason, requestID); err != nil {
			return err
		}
		return saveIdempotent(tx, requestID, op, code, out)
	})
	return out, err
}

func (s *Service) Delete(ctx context.Context, c DeleteCommand) error {
	actor, err := platformActor(ctx)
	if err != nil {
		return err
	}
	if !validWrite(c.RequestID, c.Reason) || c.Version == 0 {
		return ErrInvalidRequest
	}
	if _, ok := s.registry.Definition(c.Code); !ok {
		return ErrUnknownDefinition
	}
	return s.transact(ctx, func(tx *gorm.DB) error {
		if _, ok, err := loadDeleteIdempotent(tx, c.RequestID, c.Code); err != nil {
			return err
		} else if ok {
			return nil
		}
		var row moduleRow
		if err := tx.Where("module_code = ?", c.Code).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if row.Version != c.Version {
			return ErrConflict
		}
		var refs int64
		if err := tx.Model(&dependencyRow{}).Where("depends_on = ?", c.Code).Count(&refs).Error; err != nil {
			return err
		}
		if refs > 0 {
			return ErrReferenced
		}
		def, _ := s.registry.Definition(c.Code)
		before := rowToModule(row, def)
		if err := tx.Where("module_code = ?", c.Code).Delete(&dependencyRow{}).Error; err != nil {
			return err
		}
		res := tx.Where("module_code = ? AND version = ?", c.Code, c.Version).Delete(&moduleRow{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrConflict
		}
		if err := tx.Create(&retiredCodeRow{ModuleCode: c.Code, RetiredAt: time.Now().UTC()}).Error; err != nil {
			return err
		}
		if err := writeAudit(tx, c.Code, actor, "delete", before, nil, c.Reason, c.RequestID); err != nil {
			return err
		}
		return saveIdempotent(tx, c.RequestID, "delete", c.Code, map[string]any{"deleted": true, "module_code": c.Code})
	})
}

func writeAudit(tx *gorm.DB, code, actor, action string, before, after any, reason, requestID string) error {
	if err := consistency.AdvanceCatalog(tx); err != nil {
		return err
	}
	return tx.Create(&auditRow{ModuleCode: code, Actor: actor, Action: action, BeforeJSON: encode(before), AfterJSON: encode(after), Reason: reason, RequestID: requestID, CreatedAt: time.Now().UTC()}).Error
}
func saveIdempotent(tx *gorm.DB, requestID, operation, code string, response any) error {
	return tx.Create(&idempotencyRow{RequestID: requestID, Operation: operation, ModuleCode: code, ResponseJSON: encode(response), CreatedAt: time.Now().UTC()}).Error
}
func loadIdempotent(tx *gorm.DB, requestID, operation, code string) (Module, bool, error) {
	var row idempotencyRow
	if err := tx.Where("request_id = ?", requestID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Module{}, false, nil
		}
		return Module{}, false, err
	}
	if row.Operation != operation || row.ModuleCode != code {
		return Module{}, false, ErrInvalidRequest
	}
	var m Module
	if err := json.Unmarshal([]byte(row.ResponseJSON), &m); err != nil {
		return Module{}, false, err
	}
	return m, true, nil
}
func loadDeleteIdempotent(tx *gorm.DB, requestID, code string) (map[string]any, bool, error) {
	var row idempotencyRow
	if err := tx.Where("request_id = ?", requestID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if row.Operation != "delete" || row.ModuleCode != code {
		return nil, false, ErrInvalidRequest
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(row.ResponseJSON), &response); err != nil {
		return nil, false, err
	}
	return response, true, nil
}
