package featurecatalog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/core/identity"
)

type Service struct{ store *Store }

func NewService(store *Store) (*Service, error) {
	if store == nil || store.db == nil {
		return nil, errors.New("commercial feature: store required")
	}
	return &Service{store: store}, nil
}

type Command struct {
	RequestID string
	Reason    string
	Version   uint64
}

func platformActor(ctx context.Context) (string, error) {
	principal, ok := identity.FromContext(ctx)
	if !ok || !principal.Authenticated || principal.Subject == "" || principal.TenantID != "" {
		return "", ErrInvalid
	}
	return principal.Subject, nil
}

func validCommand(command Command) bool {
	return strings.TrimSpace(command.RequestID) != "" && strings.TrimSpace(command.Reason) != ""
}

func (s *Service) Create(ctx context.Context, definition Definition, command Command) (Feature, error) {
	actor, err := platformActor(ctx)
	if err != nil || !validCommand(command) {
		return Feature{}, ErrInvalid
	}
	feature, err := New(definition)
	if err != nil {
		return Feature{}, err
	}
	feature.Version = 1
	var result Feature
	err = s.store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if replay, found, err := loadReplay(tx, command.RequestID, "create", feature.Code); err != nil || found {
			result = replay
			return err
		}
		row := featureToRow(feature)
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := saveReferences(tx, feature); err != nil {
			return err
		}
		if err := writeAudit(tx, actor, "create", Feature{}, feature, command); err != nil {
			return err
		}
		result = feature
		return saveReplay(tx, "create", feature, command.RequestID)
	})
	return result, err
}

func (s *Service) Get(ctx context.Context, code string) (Feature, error) {
	var row featureRow
	if err := s.store.db.WithContext(ctx).Where("feature_code = ?", code).Take(&row).Error; err != nil {
		return Feature{}, err
	}
	var refs []featureModuleRow
	if err := s.store.db.WithContext(ctx).Where("feature_code = ?", code).Order("module_code").Find(&refs).Error; err != nil {
		return Feature{}, err
	}
	return rowToFeature(row, refs)
}

func (s *Service) StopSell(ctx context.Context, code string, command Command) (Feature, error) {
	return s.mutate(ctx, code, command, "stop_sell", func(feature Feature) (Feature, error) { return feature.StopSell() })
}

func (s *Service) Publish(ctx context.Context, code string, command Command) (Feature, error) {
	return s.mutate(ctx, code, command, "publish", func(feature Feature) (Feature, error) { return feature.Publish() })
}

func (s *Service) PlanSunset(ctx context.Context, code string, plan SunsetPlan, command Command) (Feature, error) {
	return s.mutate(ctx, code, command, "plan_sunset", func(feature Feature) (Feature, error) { return feature.PlanSunset(plan) })
}

func (s *Service) CompleteMigration(ctx context.Context, code string, command Command) (Feature, error) {
	return s.mutate(ctx, code, command, "complete_migration", func(feature Feature) (Feature, error) { return feature.CompleteMigration() })
}

func (s *Service) Retire(ctx context.Context, code string, references ReferenceImpact, command Command) (Feature, error) {
	return s.mutate(ctx, code, command, "retire", func(feature Feature) (Feature, error) { return feature.Retire(references) })
}

func (s *Service) mutate(ctx context.Context, code string, command Command, action string, transition func(Feature) (Feature, error)) (Feature, error) {
	actor, err := platformActor(ctx)
	if err != nil || !validCommand(command) || command.Version == 0 {
		return Feature{}, ErrInvalid
	}
	var result Feature
	err = s.store.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if replay, found, err := loadReplay(tx, command.RequestID, action, code); err != nil || found {
			result = replay
			return err
		}
		var row featureRow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("feature_code = ?", code).Take(&row).Error; err != nil {
			return err
		}
		var refs []featureModuleRow
		if err := tx.Where("feature_code = ?", code).Order("module_code").Find(&refs).Error; err != nil {
			return err
		}
		before, err := rowToFeature(row, refs)
		if err != nil || before.Version != command.Version {
			if err != nil {
				return err
			}
			return ErrTransition
		}
		after, err := transition(before)
		if err != nil {
			return err
		}
		after.Version++
		if err := tx.Model(&featureRow{}).Where("feature_code = ? AND version = ?", code, command.Version).Updates(featureToRow(after)).Error; err != nil {
			return err
		}
		if err := writeAudit(tx, actor, action, before, after, command); err != nil {
			return err
		}
		result = after
		return saveReplay(tx, action, after, command.RequestID)
	})
	return result, err
}

func saveReferences(tx *gorm.DB, feature Feature) error {
	for _, reference := range feature.ModuleRefs {
		encoded, err := json.Marshal(reference.CapabilityCodes)
		if err != nil {
			return err
		}
		if err := tx.Create(&featureModuleRow{FeatureCode: feature.Code, ModuleCode: reference.ModuleCode, CapabilityCodes: string(encoded)}).Error; err != nil {
			return err
		}
	}
	return nil
}

func writeAudit(tx *gorm.DB, actor, action string, before, after Feature, command Command) error {
	beforeJSON, _ := json.Marshal(before)
	afterJSON, _ := json.Marshal(after)
	return tx.Create(&featureAuditRow{Feature: after.Code, Actor: actor, Action: action, Before: string(beforeJSON), After: string(afterJSON), Reason: command.Reason, RequestID: command.RequestID}).Error
}

func loadReplay(tx *gorm.DB, requestID, operation, code string) (Feature, bool, error) {
	var row featureIdempotencyRow
	if err := tx.Where("request_id = ?", requestID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Feature{}, false, nil
		}
		return Feature{}, false, err
	}
	if row.Operation != operation || row.Feature != code {
		return Feature{}, false, ErrInvalid
	}
	var feature Feature
	if err := json.Unmarshal([]byte(row.Response), &feature); err != nil {
		return Feature{}, false, err
	}
	return feature, true, feature.Validate()
}

func saveReplay(tx *gorm.DB, operation string, feature Feature, requestID string) error {
	encoded, err := json.Marshal(feature)
	if err != nil {
		return err
	}
	return tx.Create(&featureIdempotencyRow{RequestID: requestID, Operation: operation, Feature: feature.Code, Response: string(encoded)}).Error
}
