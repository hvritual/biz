package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	TenantDepartmentStatusActive   = "active"
	TenantDepartmentStatusDisabled = "disabled"
)

var (
	ErrInvalidTenantDepartment        = errors.New("access: invalid tenant department")
	ErrInvalidTenantDepartmentMove    = errors.New("access: invalid tenant department hierarchy move")
	ErrInvalidTenantDepartmentLeader  = errors.New("access: invalid tenant department leader")
	ErrInvalidTenantDepartmentStatus  = errors.New("access: invalid tenant department state transition")
)

type Department struct {
	ID           string
	TenantID     string
	Name         string
	ParentID     string
	LeaderUserID string
	Email        string
	Phone        string
	Status       string
	Sort         int32
	Version      uint64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NewDepartment(id, tenantID, name, parentID, leaderUserID, email, phone string, sort int32, now time.Time) (Department, error) {
	department := Department{
		ID:           strings.TrimSpace(id),
		TenantID:     strings.TrimSpace(tenantID),
		Status:       TenantDepartmentStatusActive,
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := department.Update(name, parentID, leaderUserID, email, phone, sort, now); err != nil {
		return Department{}, err
	}
	if department.ID == "" || department.TenantID == "" {
		return Department{}, ErrInvalidTenantDepartment
	}
	return department, nil
}

func (department *Department) Update(name, parentID, leaderUserID, email, phone string, sort int32, now time.Time) error {
	if department == nil {
		return ErrInvalidTenantDepartment
	}
	name = strings.TrimSpace(name)
	parentID = strings.TrimSpace(parentID)
	leaderUserID = strings.TrimSpace(leaderUserID)
	email = strings.TrimSpace(strings.ToLower(email))
	phone = strings.TrimSpace(phone)
	if name == "" || len([]rune(name)) > 100 || len([]rune(parentID)) > 64 || len([]rune(leaderUserID)) > 64 || len([]rune(email)) > 320 || len([]rune(phone)) > 40 {
		return ErrInvalidTenantDepartment
	}
	if parentID != "" && parentID == department.ID {
		return ErrInvalidTenantDepartmentMove
	}
	department.Name = name
	department.ParentID = parentID
	department.LeaderUserID = leaderUserID
	department.Email = email
	department.Phone = phone
	department.Sort = sort
	department.UpdatedAt = now
	return nil
}

func (department *Department) Disable(now time.Time) error {
	if department == nil || department.Status != TenantDepartmentStatusActive {
		return ErrInvalidTenantDepartmentStatus
	}
	department.Status = TenantDepartmentStatusDisabled
	department.UpdatedAt = now
	return nil
}

func (department *Department) Enable(now time.Time) error {
	if department == nil || department.Status != TenantDepartmentStatusDisabled {
		return ErrInvalidTenantDepartmentStatus
	}
	department.Status = TenantDepartmentStatusActive
	department.UpdatedAt = now
	return nil
}
