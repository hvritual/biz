package deviceops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"gorm.io/gorm"
	"yunka.io/framework/requestscope"
)

type Service struct {
	scopes *requestscope.Factory[Repositories]
}

func NewService(database *gorm.DB) (*Service, error) {
	unitOfWork, err := requestscope.NewGORMFactory(database, nil)
	if err != nil {
		return nil, err
	}
	factory, err := requestscope.NewFactory(requestscope.FactoryOptions[Repositories]{
		UnitOfWork: unitOfWork,
		Repositories: requestscope.GORMRepositories(func(ctx context.Context, transaction *gorm.DB) (Repositories, error) {
			return newRepositories(transaction.WithContext(ctx))
		}),
	})
	if err != nil {
		return nil, err
	}
	return &Service{scopes: factory}, nil
}

func (service *Service) ListDevices(ctx context.Context, plan AccessPlan) ([]Device, error) {
	if !plan.Can(PermissionDeviceRead) {
		return nil, ErrForbidden
	}
	return requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[Repositories]) ([]Device, error) {
		return scope.Repositories().ListDevices(plan)
	})
}

type CreateDeviceInput struct {
	SiteID string `json:"siteId"`
	Name   string `json:"name"`
	Serial string `json:"serial"`
}

func (service *Service) CreateDevice(ctx context.Context, plan AccessPlan, input CreateDeviceInput) (Device, error) {
	input.SiteID = strings.TrimSpace(input.SiteID)
	input.Name = strings.TrimSpace(input.Name)
	input.Serial = strings.TrimSpace(input.Serial)
	if input.SiteID == "" || input.Name == "" || input.Serial == "" {
		return Device{}, ErrInvalid
	}
	if !plan.Can(PermissionDeviceCreate) || !plan.canTargetSite(PermissionDeviceCreate, input.SiteID) {
		return Device{}, ErrForbidden
	}
	return requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[Repositories]) (Device, error) {
		repos := scope.Repositories()
		exists, err := repos.SiteExists(plan.Principal.TenantID, input.SiteID)
		if err != nil {
			return Device{}, err
		}
		if !exists {
			return Device{}, ErrInvalid
		}
		device := Device{
			ID:        newID(),
			TenantID:  plan.Principal.TenantID,
			SiteID:    input.SiteID,
			Name:      input.Name,
			Serial:    input.Serial,
			CreatedBy: plan.Principal.UserID,
			Version:   1,
		}
		if err := repos.CreateDevice(&device); err != nil {
			return Device{}, err
		}
		return device, nil
	})
}

func (service *Service) GetDevice(ctx context.Context, plan AccessPlan, id string) (Device, error) {
	if !plan.Can(PermissionDeviceRead) {
		return Device{}, ErrForbidden
	}
	return requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[Repositories]) (Device, error) {
		return scope.Repositories().FindDevice(plan, PermissionDeviceRead, strings.TrimSpace(id))
	})
}

type UpdateDeviceInput struct {
	Name    string `json:"name"`
	SiteID  string `json:"siteId"`
	Version uint64 `json:"version"`
}

func (service *Service) UpdateDevice(ctx context.Context, plan AccessPlan, id string, input UpdateDeviceInput) (Device, error) {
	if !plan.Can(PermissionDeviceUpdate) || input.Version == 0 {
		return Device{}, ErrForbidden
	}
	return requestscope.ExecuteValue(ctx, service.scopes, func(scope *requestscope.Scope[Repositories]) (Device, error) {
		repos := scope.Repositories()
		current, err := repos.FindDevice(plan, PermissionDeviceUpdate, strings.TrimSpace(id))
		if err != nil {
			return Device{}, err
		}
		if current.Version != input.Version {
			return Device{}, ErrConflict
		}
		name := strings.TrimSpace(input.Name)
		if name == "" {
			name = current.Name
		}
		siteID := strings.TrimSpace(input.SiteID)
		if siteID == "" {
			siteID = current.SiteID
		}
		if siteID != current.SiteID {
			if !plan.canTargetSite(PermissionDeviceUpdate, siteID) {
				return Device{}, ErrForbidden
			}
			exists, err := repos.SiteExists(plan.Principal.TenantID, siteID)
			if err != nil {
				return Device{}, err
			}
			if !exists {
				return Device{}, ErrInvalid
			}
		}
		return repos.UpdateDevice(plan, current, name, siteID, input.Version)
	})
}

func (service *Service) DeleteDevice(ctx context.Context, plan AccessPlan, id string, version uint64) error {
	if !plan.Can(PermissionDeviceDelete) || version == 0 {
		return ErrForbidden
	}
	return requestscope.Execute(ctx, service.scopes, func(scope *requestscope.Scope[Repositories]) error {
		repos := scope.Repositories()
		current, err := repos.FindDevice(plan, PermissionDeviceDelete, strings.TrimSpace(id))
		if err != nil {
			return err
		}
		if current.Version != version {
			return ErrConflict
		}
		return repos.DeleteDevice(plan, current, version)
	})
}

func newID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes[:])
}

func isBusinessError(err error) bool {
	return errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrForbidden) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrConflict) || errors.Is(err, ErrInvalid)
}
