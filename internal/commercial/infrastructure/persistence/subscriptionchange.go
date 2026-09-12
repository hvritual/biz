package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	p "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
	"github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
	"github.com/hvritual/biz/internal/commercial/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/requestscope"
)

type changePreviewRow struct {
	SourcePlanCode    string
	SourcePlanVersion uint64
	TargetPlanCode    string
	TargetPlanVersion uint64
	ChangeID          string `gorm:"column:change_id;primaryKey"`
	TenantID          string
	ActorID           string
	RequestID         string
	Fingerprint       string
	PayloadSHA256     string `gorm:"column:payload_sha256"`
	Payload           string
	CreatedAt         time.Time
	ExpiresAt         time.Time
}

func (changePreviewRow) TableName() string { return "biz_commercial_change_previews" }

type changeReceiptRow struct {
	ChangeID      string `gorm:"column:change_id;primaryKey"`
	TenantID      string
	ActorID       string
	RequestID     string
	Fingerprint   string
	PayloadSHA256 string `gorm:"column:payload_sha256"`
	Payload       string
	Status        string
	ConfirmedAt   time.Time
}

func (changeReceiptRow) TableName() string { return "biz_commercial_change_receipts" }

type changeAuditRow struct {
	ID            uint64 `gorm:"primaryKey;autoIncrement"`
	ChangeID      string
	TenantID      string
	ActorID       string
	PayloadSHA256 string `gorm:"column:payload_sha256"`
	Payload       string
	CreatedAt     time.Time
}

func (changeAuditRow) TableName() string { return "biz_commercial_change_audit" }

type subscriptionChangeRepository struct{ tx *gorm.DB }

func NewSubscriptionChangeRepositoryFactory() requestscope.RepositoryFactory[ports.SubscriptionChangeRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.SubscriptionChangeRepositories, error) {
		if tx == nil {
			return ports.SubscriptionChangeRepositories{}, errors.New("subscription changes: root transaction required")
		}
		t := tx.Session(&gorm.Session{SkipDefaultTransaction: true})
		return ports.SubscriptionChangeRepositories{Changes: &subscriptionChangeRepository{tx: t}, Entitlements: &entitlementRepository{tx: t}, Tasks: &provisioningRepository{tx: t}, Events: &outboxRepository{tx: t}}, nil
	})
}
func (r *subscriptionChangeRepository) Now(ctx context.Context) (time.Time, error) {
	return consistency.Now(r.tx.WithContext(ctx))
}
func (r *subscriptionChangeRepository) LockTenant(ctx context.Context, tenant string) (subscription.Subscription, error) {
	if _, err := consistency.LockCatalog(r.tx.WithContext(ctx), false); err != nil {
		return subscription.Subscription{}, err
	}
	return (&subscriptionRepository{tx: r.tx}).GetBase(ctx, tenant, true)
}
func (r *subscriptionChangeRepository) Preview(ctx context.Context, tenant, id string, current bool) (*change.Preview, error) {
	db := r.tx.WithContext(ctx)
	if current {
		db = db.Clauses(clause.Locking{Strength: "SHARE"})
	}
	var row changePreviewRow
	err := db.Where("tenant_id=? AND change_id=?", tenant, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p change.Preview
	if json.Unmarshal([]byte(row.Payload), &p) != nil || p.Before.PlanCode != row.SourcePlanCode || p.Before.PlanVersion != row.SourcePlanVersion || p.Target.PlanCode != row.TargetPlanCode || p.Target.Number != row.TargetPlanVersion || p.ChangeID != row.ChangeID || p.Input.TenantID != row.TenantID || p.ActorID != row.ActorID || p.Input.RequestID != row.RequestID || p.Fingerprint != row.Fingerprint || p.Hash != row.PayloadSHA256 || !p.CreatedAt.Equal(row.CreatedAt) || !p.ExpiresAt.Equal(row.ExpiresAt) || p.Integrity() != nil {
		return nil, change.ErrCorrupt
	}
	return &p, nil
}
func (r *subscriptionChangeRepository) SavePreview(ctx context.Context, p change.Preview) error {
	if err := p.Integrity(); err != nil {
		return err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return r.tx.WithContext(ctx).Create(&changePreviewRow{SourcePlanCode: p.Before.PlanCode, SourcePlanVersion: p.Before.PlanVersion, TargetPlanCode: p.Target.PlanCode, TargetPlanVersion: p.Target.Number, ChangeID: p.ChangeID, TenantID: p.Input.TenantID, ActorID: p.ActorID, RequestID: p.Input.RequestID, Fingerprint: p.Fingerprint, PayloadSHA256: p.Hash, Payload: string(b), CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt}).Error
}
func decodeChangeReceipt(row changeReceiptRow) (*change.Receipt, error) {
	var v change.Receipt
	if json.Unmarshal([]byte(row.Payload), &v) != nil || v.ChangeID != row.ChangeID || v.TenantID != row.TenantID || v.ActorID != row.ActorID || v.RequestID != row.RequestID || v.Fingerprint != row.Fingerprint || v.Hash != row.PayloadSHA256 || v.Status != row.Status || !v.ConfirmedAt.Equal(row.ConfirmedAt) || v.Integrity() != nil {
		return nil, change.ErrCorrupt
	}
	return &v, nil
}
func (r *subscriptionChangeRepository) Receipt(ctx context.Context, tenant, id string, current bool) (*change.Receipt, error) {
	db := r.tx.WithContext(ctx)
	if current {
		db = db.Clauses(clause.Locking{Strength: "SHARE"})
	}
	var row changeReceiptRow
	err := db.Where("tenant_id=? AND change_id=?", tenant, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decodeChangeReceipt(row)
}
func (r *subscriptionChangeRepository) ReceiptForRequest(ctx context.Context, tenant, actor, key, fp string) (*change.Receipt, error) {
	var row changeReceiptRow
	err := r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("actor_id=? AND request_id=?", actor, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	v, err := decodeChangeReceipt(row)
	if err != nil {
		return nil, err
	}
	if v.Fingerprint != fp {
		return nil, change.ErrRequestConflict
	}
	return v, nil
}
func (r *subscriptionChangeRepository) SaveCurrent(ctx context.Context, before, after subscription.Subscription) error {
	if before.TenantID != after.TenantID || before.ID != after.ID || before.Revision == 0 || before.Revision == ^uint64(0) || after.Revision != before.Revision+1 || after.Validate() != nil {
		return change.ErrConflict
	}
	b, err := json.Marshal(after)
	if err != nil {
		return err
	}
	// Existing CE-08 rows have no revision field and are canonically revision 1.
	// The locking read provides serialization; this predicate also rejects a
	// stale caller and does not require a destructive legacy-data rewrite.
	res := r.tx.WithContext(ctx).Model(&subscriptionRow{}).Where("tenant_id=? AND COALESCE(NULLIF(CAST(JSON_UNQUOTE(JSON_EXTRACT(payload,'$.revision')) AS UNSIGNED),0),1)=?", before.TenantID, before.Revision).Updates(map[string]any{"plan_code": after.PlanCode, "plan_version": after.PlanVersion, "payload": string(b)})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return change.ErrConflict
	}
	return nil
}
func (r *subscriptionChangeRepository) Complete(ctx context.Context, v change.Receipt) error {
	if err := v.Integrity(); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	row := changeReceiptRow{ChangeID: v.ChangeID, TenantID: v.TenantID, ActorID: v.ActorID, RequestID: v.RequestID, Fingerprint: v.Fingerprint, PayloadSHA256: v.Hash, Payload: string(b), Status: v.Status, ConfirmedAt: v.ConfirmedAt}
	if err = r.tx.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	if err = r.tx.WithContext(ctx).Create(&changeAuditRow{ChangeID: v.ChangeID, TenantID: v.TenantID, ActorID: v.ActorID, PayloadSHA256: v.Hash, Payload: string(b), CreatedAt: v.ConfirmedAt}).Error; err != nil {
		return err
	}
	return (&outboxRepository{tx: r.tx}).Append(ctx, p.Event{TenantID: v.TenantID, AggregateID: v.After.ID, AggregateVersion: v.After.Revision, ChangeID: v.ChangeID, TaskID: v.ProvisioningTaskID, Status: v.Status, SourceVersion: v.AfterSourceVersion, EntitlementVersion: v.AfterEntitlementVersion, OccurredAt: v.ConfirmedAt}.Seal())
}

func (r *subscriptionChangeRepository) PreviewForRequest(ctx context.Context, actor, key, fp string) (*change.Preview, error) {
	var row changePreviewRow
	err := r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("actor_id=? AND request_id=?", actor, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.Fingerprint != fp {
		return nil, change.ErrRequestConflict
	}
	return r.Preview(ctx, row.TenantID, row.ChangeID, true)
}
