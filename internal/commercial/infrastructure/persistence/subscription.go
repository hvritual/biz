package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	"github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
	"github.com/hvritual/biz/internal/commercial/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
	"yunka.io/framework/requestscope"
)

type subscriptionRuleLockRow struct {
	ID uint8 `gorm:"primaryKey"`
}

func (subscriptionRuleLockRow) TableName() string { return "biz_commercial_subscription_rule_lock" }

type subscriptionRuleRow struct {
	RuleID      string `gorm:"primaryKey"`
	Version     uint64
	Priority    int32
	SalesScope  string
	PlanCode    string
	PlanVersion uint64
	Enabled     bool
	Payload     string
}

func (subscriptionRuleRow) TableName() string { return "biz_commercial_subscription_rules" }

type subscriptionRow struct {
	TenantID       string `gorm:"primaryKey"`
	SubscriptionID string `gorm:"uniqueIndex"`
	PlanCode       string
	PlanVersion    uint64
	Payload        string
}

func (subscriptionRow) TableName() string { return "biz_commercial_subscriptions" }

type subscriptionReceiptRow struct {
	ScopeID     string `gorm:"primaryKey"`
	RequestID   string `gorm:"primaryKey"`
	Kind        string `gorm:"primaryKey"`
	Fingerprint string
	Payload     string
}

func (subscriptionReceiptRow) TableName() string { return "biz_commercial_subscription_receipts" }

type subscriptionAuditRow struct {
	ID                                           uint64 `gorm:"primaryKey;autoIncrement"`
	TenantID, RequestID, ActorID, Action, Reason string
	Payload                                      string
	CreatedAt                                    time.Time
}

func (subscriptionAuditRow) TableName() string { return "biz_commercial_subscription_audit" }

type subscriptionRepository struct{ tx *gorm.DB }

func NewSubscriptionRepositoryFactory() requestscope.RepositoryFactory[ports.SubscriptionRepositories] {
	return requestscope.GORMRepositories(func(_ context.Context, tx *gorm.DB) (ports.SubscriptionRepositories, error) {
		if tx == nil {
			return ports.SubscriptionRepositories{}, errors.New("subscriptions: root transaction required")
		}
		t := tx.Session(&gorm.Session{SkipDefaultTransaction: true})
		return ports.SubscriptionRepositories{Subscriptions: &subscriptionRepository{t}, Entitlements: &entitlementRepository{tx: t}}, nil
	})
}
func (r *subscriptionRepository) Now(ctx context.Context) (time.Time, error) {
	return consistency.Now(r.tx.WithContext(ctx))
}
func (r *subscriptionRepository) LockRules(ctx context.Context) error {
	if _, e := consistency.LockCatalog(r.tx.WithContext(ctx), false); e != nil {
		return e
	}
	db := r.tx.WithContext(ctx)
	if e := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&subscriptionRuleLockRow{1}).Error; e != nil {
		return e
	}
	var x subscriptionRuleLockRow
	return db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&x, 1).Error
}
func decodeRule(x subscriptionRuleRow) (subscription.Rule, error) {
	var v subscription.Rule
	if e := json.Unmarshal([]byte(x.Payload), &v); e != nil {
		return v, subscription.ErrInvalid
	}
	return v, v.Validate()
}
func (r *subscriptionRepository) GetRule(ctx context.Context, id string, lock bool) (subscription.Rule, error) {
	db := r.tx.WithContext(ctx).Where("rule_id=?", id)
	if lock {
		db = db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var x subscriptionRuleRow
	e := db.First(&x).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return subscription.Rule{}, subscription.ErrNotFound
	}
	if e != nil {
		return subscription.Rule{}, e
	}
	return decodeRule(x)
}
func (r *subscriptionRepository) ListRules(ctx context.Context, lock bool) ([]subscription.Rule, error) {
	db := r.tx.WithContext(ctx)
	if lock {
		db = db.Clauses(clause.Locking{Strength: "SHARE"})
	}
	var xs []subscriptionRuleRow
	if e := db.Order("priority DESC, rule_id ASC").Find(&xs).Error; e != nil {
		return nil, e
	}
	out := make([]subscription.Rule, 0, len(xs))
	for _, x := range xs {
		v, e := decodeRule(x)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (r *subscriptionRepository) SaveRule(ctx context.Context, v subscription.Rule, expected uint64) error {
	b, _ := json.Marshal(v)
	x := subscriptionRuleRow{v.RuleID, v.Version, v.Priority, v.SalesScope, v.PlanCode, v.PlanVersion, v.Enabled, string(b)}
	if expected == 0 {
		return r.tx.WithContext(ctx).Create(&x).Error
	}
	res := r.tx.WithContext(ctx).Model(&subscriptionRuleRow{}).Where("rule_id=? AND version=?", v.RuleID, expected).Updates(map[string]any{"version": v.Version, "priority": v.Priority, "sales_scope": v.SalesScope, "plan_code": v.PlanCode, "plan_version": v.PlanVersion, "enabled": v.Enabled, "payload": string(b)})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return subscription.ErrConflict
	}
	return nil
}
func (r *subscriptionRepository) receipt(ctx context.Context, scope, id, kind, fp string) ([]byte, error) {
	var x subscriptionReceiptRow
	e := r.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("scope_id=? AND request_id=? AND kind=?", scope, id, kind).First(&x).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if x.Fingerprint != fp {
		return nil, subscription.ErrRequestConflict
	}
	return []byte(x.Payload), nil
}
func (r *subscriptionRepository) RuleReceipt(ctx context.Context, s, id, fp string) (*subscription.Rule, error) {
	b, e := r.receipt(ctx, s, id, "rule", fp)
	if e != nil || b == nil {
		return nil, e
	}
	var v subscription.Rule
	e = json.Unmarshal(b, &v)
	return &v, e
}
func (r *subscriptionRepository) SaveRuleReceipt(ctx context.Context, s, id, fp string, v subscription.Rule) error {
	b, _ := json.Marshal(v)
	return r.tx.WithContext(ctx).Create(&subscriptionReceiptRow{s, id, "rule", fp, string(b)}).Error
}
func (r *subscriptionRepository) GetBase(ctx context.Context, t string, lock bool) (subscription.Subscription, error) {
	db := r.tx.WithContext(ctx).Where("tenant_id=?", t)
	if lock {
		db = db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var x subscriptionRow
	e := db.First(&x).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return subscription.Subscription{}, subscription.ErrNotFound
	}
	if e != nil {
		return subscription.Subscription{}, e
	}
	var v subscription.Subscription
	e = json.Unmarshal([]byte(x.Payload), &v)
	if e != nil {
		return v, e
	}
	if v.TenantID != x.TenantID || v.ID != x.SubscriptionID || v.PlanCode != x.PlanCode || v.PlanVersion != x.PlanVersion {
		return v, subscription.ErrInvalid
	}
	return v, v.Validate()
}
func (r *subscriptionRepository) BootstrapReceipt(ctx context.Context, t, id, fp string) (*subscription.Subscription, error) {
	b, e := r.receipt(ctx, t, id, "bootstrap", fp)
	if e != nil || b == nil {
		return nil, e
	}
	var v subscription.Subscription
	e = json.Unmarshal(b, &v)
	return &v, e
}
func (r *subscriptionRepository) SaveBase(ctx context.Context, v subscription.Subscription) error {
	b, _ := json.Marshal(v)
	return r.tx.WithContext(ctx).Create(&subscriptionRow{v.TenantID, v.ID, v.PlanCode, v.PlanVersion, string(b)}).Error
}
func (r *subscriptionRepository) SaveBootstrapReceipt(ctx context.Context, t, id, fp string, v subscription.Subscription) error {
	b, _ := json.Marshal(v)
	return r.tx.WithContext(ctx).Create(&subscriptionReceiptRow{t, id, "bootstrap", fp, string(b)}).Error
}
func (r *subscriptionRepository) Audit(ctx context.Context, a ports.SubscriptionAudit) error {
	b, _ := json.Marshal(a.Subscription)
	return r.tx.WithContext(ctx).Create(&subscriptionAuditRow{TenantID: a.TenantID, RequestID: a.RequestID, ActorID: a.ActorID, Action: a.Action, Reason: a.Reason, Payload: string(b), CreatedAt: a.At}).Error
}
