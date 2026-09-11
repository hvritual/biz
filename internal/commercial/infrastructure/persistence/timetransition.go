package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	transition "github.com/hvritual/biz/internal/commercial/domain/timetransition"
	"github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
	"github.com/hvritual/biz/internal/commercial/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type timeTransitionRow struct {
	TransitionID     string     `gorm:"column:transition_id;primaryKey"`
	Kind             string     `gorm:"column:kind"`
	TenantID         string     `gorm:"column:tenant_id"`
	AuthorityID      string     `gorm:"column:authority_id"`
	AuthorityVersion uint64     `gorm:"column:authority_version"`
	DueAt            time.Time  `gorm:"column:due_at"`
	BusinessTimezone string     `gorm:"column:business_timezone"`
	Revision         uint64     `gorm:"column:revision"`
	State            string     `gorm:"column:state"`
	LeaseUntil       *time.Time `gorm:"column:lease_until"`
	PayloadSHA256    string     `gorm:"column:payload_sha256"`
	Payload          string     `gorm:"column:payload"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (timeTransitionRow) TableName() string { return "biz_commercial_time_transitions" }

type timeTransitionAuditRow struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	TransitionID  string    `gorm:"column:transition_id"`
	Revision      uint64    `gorm:"column:revision"`
	State         string    `gorm:"column:state"`
	Outcome       string    `gorm:"column:outcome"`
	PayloadSHA256 string    `gorm:"column:payload_sha256"`
	Payload       string    `gorm:"column:payload"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (timeTransitionAuditRow) TableName() string { return "biz_commercial_time_transition_audit" }

type timeTransitionRepository struct{ tx *gorm.DB }

func newTimeTransitionRepository(tx *gorm.DB) ports.TimeTransitionRepository {
	return &timeTransitionRepository{tx: tx.Session(&gorm.Session{SkipDefaultTransaction: true})}
}
func (r *timeTransitionRepository) Now(ctx context.Context) (time.Time, error) {
	return consistency.Now(r.tx.WithContext(ctx))
}
func transitionRow(t transition.Task) (timeTransitionRow, error) {
	if err := t.Integrity(); err != nil {
		return timeTransitionRow{}, err
	}
	payload, err := json.Marshal(t)
	if err != nil {
		return timeTransitionRow{}, err
	}
	return timeTransitionRow{TransitionID: t.ID, Kind: t.Kind, TenantID: t.TenantID, AuthorityID: t.AuthorityID, AuthorityVersion: t.AuthorityVersion, DueAt: t.DueAt, BusinessTimezone: t.BusinessTimezone, Revision: t.Revision, State: t.State, LeaseUntil: t.LeaseUntil, PayloadSHA256: t.Hash, Payload: string(payload), CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt}, nil
}
func decodeTransition(row timeTransitionRow) (*transition.Task, error) {
	var t transition.Task
	if json.Unmarshal([]byte(row.Payload), &t) != nil || t.Integrity() != nil || t.ID != row.TransitionID || t.Kind != row.Kind || t.TenantID != row.TenantID || t.AuthorityID != row.AuthorityID || t.AuthorityVersion != row.AuthorityVersion || !t.DueAt.Equal(row.DueAt) || t.BusinessTimezone != row.BusinessTimezone || t.Revision != row.Revision || t.State != row.State || t.Hash != row.PayloadSHA256 || !sameTime(t.LeaseUntil, row.LeaseUntil) || !t.CreatedAt.Equal(row.CreatedAt) || !t.UpdatedAt.Equal(row.UpdatedAt) {
		return nil, transition.ErrCorrupt
	}
	return &t, nil
}
func (r *timeTransitionRepository) Insert(ctx context.Context, t transition.Task) error {
	row, err := transitionRow(t)
	if err != nil {
		return err
	}
	result := r.tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var existing timeTransitionRow
		if err := r.tx.WithContext(ctx).Where("transition_id=?", t.ID).First(&existing).Error; err != nil {
			return err
		}
		current, err := decodeTransition(existing)
		if err != nil || current.Hash != t.Hash {
			return transition.ErrConflict
		}
		return nil
	}
	return r.audit(ctx, t)
}
func (r *timeTransitionRepository) Get(ctx context.Context, id string) (*transition.Task, error) {
	var row timeTransitionRow
	err := r.tx.WithContext(ctx).Where("transition_id=?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, transition.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeTransition(row)
}
func (r *timeTransitionRepository) ClaimDue(ctx context.Context, owner string, now time.Time, lease time.Duration) (*transition.Task, error) {
	db := r.tx.WithContext(ctx)
	var row timeTransitionRow
	err := db.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("(state=? AND due_at<=?) OR (state=? AND lease_until<=?)", transition.Queued, now, transition.Running, now).
		Order("due_at,transition_id").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	before, err := decodeTransition(row)
	if err != nil {
		return nil, err
	}
	after := *before
	if err := after.Claim(owner, now, lease); err != nil {
		return nil, err
	}
	if err := r.Save(ctx, *before, after); err != nil {
		return nil, err
	}
	return &after, nil
}
func (r *timeTransitionRepository) Lock(ctx context.Context, id string) (*transition.Task, error) {
	var row timeTransitionRow
	err := r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("transition_id=?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, transition.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeTransition(row)
}
func (r *timeTransitionRepository) Save(ctx context.Context, before, after transition.Task) error {
	if before.ID != after.ID || before.Revision == 0 || after.Revision != before.Revision+1 || before.Kind != after.Kind || before.TenantID != after.TenantID || before.AuthorityID != after.AuthorityID || before.AuthorityVersion != after.AuthorityVersion || !before.DueAt.Equal(after.DueAt) || before.BusinessTimezone != after.BusinessTimezone {
		return transition.ErrConflict
	}
	row, err := transitionRow(after)
	if err != nil {
		return err
	}
	result := r.tx.WithContext(ctx).Model(&timeTransitionRow{}).
		Where("transition_id=? AND revision=? AND payload_sha256=?", before.ID, before.Revision, before.Hash).
		Updates(map[string]any{"revision": row.Revision, "state": row.State, "lease_until": row.LeaseUntil, "payload_sha256": row.PayloadSHA256, "payload": row.Payload, "updated_at": row.UpdatedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return transition.ErrConflict
	}
	return r.audit(ctx, after)
}
func (r *timeTransitionRepository) audit(ctx context.Context, t transition.Task) error {
	payload, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return r.tx.WithContext(ctx).Create(&timeTransitionAuditRow{TransitionID: t.ID, Revision: t.Revision, State: t.State, Outcome: t.Outcome, PayloadSHA256: t.Hash, Payload: string(payload), CreatedAt: t.UpdatedAt}).Error
}
