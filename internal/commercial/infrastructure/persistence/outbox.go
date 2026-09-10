package persistence

import (
	"context"
	"encoding/json"
	"errors"
	p "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	"github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const subscriptionConsumer = "subscription-state-v1"

type outboxRow struct {
	EventID          string `gorm:"primaryKey"`
	TenantID         string
	AggregateID      string
	AggregateVersion uint64
	ChangeID         string
	PayloadSHA256    string `gorm:"column:payload_sha256"`
	Payload          string
	State            string
	Attempts         uint32
	LeaseOwner       string
	LeaseToken       uint64
	LeaseUntil       *time.Time
	NextAttemptAt    time.Time
	FailureCode      string
	OccurredAt       time.Time
	DeliveredAt      *time.Time
}

func (outboxRow) TableName() string { return "biz_commercial_outbox" }

type inboxRow struct {
	ConsumerID    string `gorm:"primaryKey"`
	EventID       string `gorm:"primaryKey"`
	PayloadSHA256 string `gorm:"column:payload_sha256"`
	Outcome       string
	ReceivedAt    time.Time
}

func (inboxRow) TableName() string { return "biz_commercial_inbox" }

type notificationRow struct {
	TenantID         string `gorm:"primaryKey"`
	AggregateID      string `gorm:"primaryKey"`
	AggregateVersion uint64
	PayloadSHA256    string `gorm:"column:payload_sha256"`
	Payload          string
	UpdatedAt        time.Time
}

func (notificationRow) TableName() string { return "biz_commercial_subscription_notifications" }

type outboxRepository struct{ tx *gorm.DB }

func (r *outboxRepository) Append(ctx context.Context, e p.Event) error {
	if e.Integrity() != nil {
		return p.ErrCorrupt
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return r.tx.WithContext(ctx).Create(&outboxRow{EventID: e.ID, TenantID: e.TenantID, AggregateID: e.AggregateID, AggregateVersion: e.AggregateVersion, ChangeID: e.ChangeID, PayloadSHA256: e.Hash, Payload: string(b), State: "PENDING", NextAttemptAt: e.OccurredAt, OccurredAt: e.OccurredAt}).Error
}
func eventFromRow(row outboxRow) (p.Event, error) {
	var e p.Event
	if json.Unmarshal([]byte(row.Payload), &e) != nil || e.Integrity() != nil || e.ID != row.EventID || e.TenantID != row.TenantID || e.AggregateID != row.AggregateID || e.AggregateVersion != row.AggregateVersion || e.ChangeID != row.ChangeID || e.Hash != row.PayloadSHA256 || !e.OccurredAt.Equal(row.OccurredAt) {
		return e, p.ErrCorrupt
	}
	return e, nil
}
func delivery(row outboxRow) p.Delivery {
	e, err := eventFromRow(row)
	code := row.FailureCode
	if err != nil {
		code = "OUTBOX_AUTHORITY_CORRUPT"
		e = p.Event{ID: row.EventID, TenantID: row.TenantID, AggregateID: row.AggregateID, AggregateVersion: row.AggregateVersion, ChangeID: row.ChangeID}
	}
	return p.Delivery{ID: row.EventID, Event: e, Attempts: row.Attempts, State: row.State, FailureCode: code, NextAttemptAt: row.NextAttemptAt, LeaseOwner: row.LeaseOwner, LeaseToken: row.LeaseToken, LeaseUntil: row.LeaseUntil}
}
func (r *outboxRepository) Claim(ctx context.Context, owner string, lease time.Duration) (*p.Delivery, error) {
	if !p.Key(owner) || lease < 5*time.Second || lease > 5*time.Minute {
		return nil, p.ErrInvalid
	}
	now, e := consistency.Now(r.tx.WithContext(ctx))
	if e != nil {
		return nil, e
	}
	var row outboxRow
	e = r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("(state IN ? AND next_attempt_at<=?) OR (state='LEASED' AND lease_until<=?)", []string{"PENDING", "RETRY_WAIT"}, now, now).Order("next_attempt_at,event_id").First(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if row.LeaseToken == ^uint64(0) {
		return nil, p.ErrCorrupt
	}
	values := map[string]any{}
	if row.Attempts >= 5 {
		row.State = "DEAD"
		row.FailureCode = "DELIVERY_RETRIES_EXHAUSTED"
		row.LeaseOwner = ""
		row.LeaseUntil = nil
		values = map[string]any{"state": row.State, "failure_code": row.FailureCode, "lease_owner": "", "lease_until": nil}
	} else {
		row.Attempts++
		row.LeaseToken++
		row.State = "LEASED"
		row.LeaseOwner = owner
		until := now.Add(lease)
		row.LeaseUntil = &until
		values = map[string]any{"state": row.State, "attempts": row.Attempts, "lease_token": row.LeaseToken, "lease_owner": owner, "lease_until": until}
	}
	if e = r.tx.WithContext(ctx).Model(&outboxRow{}).Where("event_id=?", row.EventID).Updates(values).Error; e != nil {
		return nil, e
	}
	d := delivery(row)
	return &d, nil
}
func (r *outboxRepository) locked(ctx context.Context, id, owner string, token uint64) (outboxRow, time.Time, error) {
	var now time.Time
	var row outboxRow
	e := r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id=?", id).First(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return row, now, p.ErrNotFound
	}
	if e != nil {
		return row, now, e
	}
	// The row may have waited behind a live transaction until after expiry.
	// Database time sampled before acquiring it is not admission authority.
	now, e = consistency.Now(r.tx.WithContext(ctx))
	if e != nil {
		return row, now, e
	}
	if row.LeaseToken != token {
		return row, now, p.ErrLease
	}
	if row.State == "DELIVERED" {
		return row, now, nil
	}
	if row.State != "LEASED" || row.LeaseOwner != owner || row.LeaseUntil == nil || !now.Before(*row.LeaseUntil) {
		return row, now, p.ErrLease
	}
	return row, now, nil
}
func (r *outboxRepository) Deliver(ctx context.Context, id, owner string, token uint64) (p.DeliveryReceipt, error) {
	row, now, err := r.locked(ctx, id, owner, token)
	if err != nil {
		return p.DeliveryReceipt{}, err
	}
	e, err := eventFromRow(row)
	if err != nil {
		return p.DeliveryReceipt{}, err
	}
	var inbox inboxRow
	err = r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("consumer_id=? AND event_id=?", subscriptionConsumer, id).First(&inbox).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return p.DeliveryReceipt{}, err
	}
	if row.State == "DELIVERED" {
		if inbox.PayloadSHA256 != e.Hash {
			return p.DeliveryReceipt{}, p.ErrCorrupt
		}
		return p.DeliveryReceipt{EventID: id, Outcome: inbox.Outcome, AggregateVersion: e.AggregateVersion, EntitlementVersion: e.EntitlementVersion}, nil
	}
	initial := notificationRow{TenantID: e.TenantID, AggregateID: e.AggregateID, UpdatedAt: now}
	if err = r.tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&initial).Error; err != nil {
		return p.DeliveryReceipt{}, err
	}
	var head notificationRow
	if err = r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND aggregate_id=?", e.TenantID, e.AggregateID).First(&head).Error; err != nil {
		return p.DeliveryReceipt{}, err
	}
	if head.AggregateVersion > 0 {
		var previous p.Event
		if json.Unmarshal([]byte(head.Payload), &previous) != nil || previous.Integrity() != nil || previous.Hash != head.PayloadSHA256 || previous.AggregateVersion != head.AggregateVersion || previous.TenantID != head.TenantID || previous.AggregateID != head.AggregateID {
			return p.DeliveryReceipt{}, p.ErrCorrupt
		}
	}
	// Acquiring the inbox/projection head may itself have blocked. Recheck
	// after every competing row lock and before committing consumer effects.
	now, err = consistency.Now(r.tx.WithContext(ctx))
	if err != nil {
		return p.DeliveryReceipt{}, err
	}
	if row.LeaseUntil == nil || !now.Before(*row.LeaseUntil) {
		return p.DeliveryReceipt{}, p.ErrLease
	}
	outcome, err := p.ClassifyDelivery(e, inbox.PayloadSHA256, head.AggregateVersion, head.PayloadSHA256)
	if err != nil {
		return p.DeliveryReceipt{}, err
	}
	if inbox.EventID == "" {
		if err = r.tx.WithContext(ctx).Create(&inboxRow{ConsumerID: subscriptionConsumer, EventID: id, PayloadSHA256: e.Hash, Outcome: outcome, ReceivedAt: now}).Error; err != nil {
			return p.DeliveryReceipt{}, err
		}
	}
	if outcome == "APPLIED" {
		if err = r.tx.WithContext(ctx).Model(&notificationRow{}).Where("tenant_id=? AND aggregate_id=? AND aggregate_version<?", e.TenantID, e.AggregateID, e.AggregateVersion).Updates(map[string]any{"aggregate_version": e.AggregateVersion, "payload_sha256": e.Hash, "payload": row.Payload, "updated_at": now}).Error; err != nil {
			return p.DeliveryReceipt{}, err
		}
	}
	if err = r.tx.WithContext(ctx).Model(&outboxRow{}).Where("event_id=?", id).Updates(map[string]any{"state": "DELIVERED", "delivered_at": now, "lease_owner": "", "lease_until": nil, "failure_code": ""}).Error; err != nil {
		return p.DeliveryReceipt{}, err
	}
	return p.DeliveryReceipt{EventID: id, Outcome: outcome, AggregateVersion: e.AggregateVersion, EntitlementVersion: e.EntitlementVersion}, nil
}
func (r *outboxRepository) Fail(ctx context.Context, id, owner string, token uint64, code string) error {
	if !p.Key(code) {
		return p.ErrInvalid
	}
	row, now, e := r.locked(ctx, id, owner, token)
	if e != nil {
		return e
	}
	if row.State == "DELIVERED" {
		return nil
	}
	state := "RETRY_WAIT"
	if row.Attempts >= 5 {
		state = "DEAD"
	}
	return r.tx.WithContext(ctx).Model(&outboxRow{}).Where("event_id=?", id).Updates(map[string]any{"state": state, "failure_code": code, "next_attempt_at": now.Add(p.Backoff(row.Attempts)), "lease_owner": "", "lease_until": nil}).Error
}
func (r *outboxRepository) List(ctx context.Context, tenant, after string, limit uint32) ([]p.Delivery, error) {
	if limit < 1 || limit > 100 {
		return nil, p.ErrInvalid
	}
	var rows []outboxRow
	if err := r.tx.WithContext(ctx).Where("tenant_id=? AND event_id>?", tenant, after).Order("event_id").Limit(int(limit)).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := []p.Delivery{}
	for _, row := range rows {
		out = append(out, delivery(row))
	}
	return out, nil
}
