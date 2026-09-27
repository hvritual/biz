package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type routingEventRecord struct {
	EventID       string     `gorm:"column:event_id;primaryKey;type:varbinary(160)"`
	TenantID      string     `gorm:"column:tenant_id;type:varbinary(64);not null;index"`
	GroupID       string     `gorm:"column:group_id;type:varbinary(64);not null;index"`
	TypeCode      string     `gorm:"column:type_code;size:64;not null"`
	Level         string     `gorm:"column:level;size:16;not null"`
	TraceID       string     `gorm:"column:trace_id;type:varbinary(160);not null"`
	ReferenceKind string     `gorm:"column:reference_kind;size:64;not null"`
	ReferenceID   string     `gorm:"column:reference_id;type:varbinary(160);not null"`
	PayloadHash   string     `gorm:"column:payload_hash;size:64;not null"`
	State         string     `gorm:"column:state;size:24;not null;index"`
	Attempts      uint32     `gorm:"column:attempts;not null;default:0"`
	LeaseOwner    string     `gorm:"column:lease_owner;size:96;not null;default:''"`
	LeaseToken    uint64     `gorm:"column:lease_token;not null;default:0"`
	LeaseUntil    *time.Time `gorm:"column:lease_until;type:datetime(6);index"`
	FailureCode   string     `gorm:"column:failure_code;size:64;not null;default:''"`
	OccurredAt    time.Time  `gorm:"column:occurred_at;type:datetime(6);not null;index"`
	RoutedAt      *time.Time `gorm:"column:routed_at;type:datetime(6)"`
	CreatedAt     time.Time  `gorm:"column:created_at;type:datetime(6);not null"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;type:datetime(6);not null"`
}

func (routingEventRecord) TableName() string { return "biz_notification_events" }

type inAppRecord struct {
	MessageID            string     `gorm:"column:message_id;primaryKey;size:64"`
	TenantID             string     `gorm:"column:tenant_id;type:varbinary(64);not null;uniqueIndex:uq_notification_inapp,priority:1;index"`
	UserID               string     `gorm:"column:user_id;type:varbinary(64);not null;uniqueIndex:uq_notification_inapp,priority:2;index"`
	EventID              string     `gorm:"column:event_id;type:varbinary(160);not null;uniqueIndex:uq_notification_inapp,priority:3"`
	ConfigurationID      string     `gorm:"column:configuration_id;type:varbinary(64);not null"`
	ConfigurationVersion uint64     `gorm:"column:configuration_version;not null"`
	GroupID              string     `gorm:"column:group_id;type:varbinary(64);not null"`
	TypeCode             string     `gorm:"column:type_code;size:64;not null"`
	Level                string     `gorm:"column:level;size:16;not null"`
	TraceID              string     `gorm:"column:trace_id;type:varbinary(160);not null"`
	ReferenceKind        string     `gorm:"column:reference_kind;size:64;not null"`
	ReferenceID          string     `gorm:"column:reference_id;type:varbinary(160);not null"`
	CreatedAt            time.Time  `gorm:"column:created_at;type:datetime(6);not null;index"`
	ReadAt               *time.Time `gorm:"column:read_at;type:datetime(6);index"`
}

func (inAppRecord) TableName() string { return "biz_notification_in_app" }

type externalTaskRecord struct {
	TaskID               string     `gorm:"column:task_id;primaryKey;size:64"`
	TenantID             string     `gorm:"column:tenant_id;type:varbinary(64);not null;uniqueIndex:uq_notification_external,priority:1;index"`
	UserID               string     `gorm:"column:user_id;type:varbinary(64);not null;uniqueIndex:uq_notification_external,priority:2;index"`
	EventID              string     `gorm:"column:event_id;type:varbinary(160);not null;uniqueIndex:uq_notification_external,priority:3"`
	Channel              string     `gorm:"column:channel;size:16;not null;uniqueIndex:uq_notification_external,priority:4"`
	ConfigurationID      string     `gorm:"column:configuration_id;type:varbinary(64);not null"`
	ConfigurationVersion uint64     `gorm:"column:configuration_version;not null"`
	GroupID              string     `gorm:"column:group_id;type:varbinary(64);not null"`
	TypeCode             string     `gorm:"column:type_code;size:64;not null"`
	Level                string     `gorm:"column:level;size:16;not null"`
	TraceID              string     `gorm:"column:trace_id;type:varbinary(160);not null"`
	ReferenceKind        string     `gorm:"column:reference_kind;size:64;not null"`
	ReferenceID          string     `gorm:"column:reference_id;type:varbinary(160);not null"`
	State                string     `gorm:"column:state;size:24;not null;index"`
	Attempts             uint32     `gorm:"column:attempts;not null;default:0"`
	LeaseOwner           string     `gorm:"column:lease_owner;size:96;not null;default:''"`
	LeaseToken           uint64     `gorm:"column:lease_token;not null;default:0"`
	LeaseUntil           *time.Time `gorm:"column:lease_until;type:datetime(6);index"`
	NextAttemptAt        *time.Time `gorm:"column:next_attempt_at;type:datetime(6);index"`
	FailureCode          string     `gorm:"column:failure_code;size:64;not null;default:''"`
	ProviderReceipt      string     `gorm:"column:provider_receipt;size:200;not null;default:''"`
	AcceptedAt           *time.Time `gorm:"column:accepted_at;type:datetime(6)"`
	DeliveredAt          *time.Time `gorm:"column:delivered_at;type:datetime(6)"`
	CreatedAt            time.Time  `gorm:"column:created_at;type:datetime(6);not null"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;type:datetime(6);not null"`
}

func (externalTaskRecord) TableName() string { return "biz_notification_external_tasks" }

type routeOutcomeRecord struct {
	OutcomeID            string    `gorm:"column:outcome_id;primaryKey;size:64"`
	EventID              string    `gorm:"column:event_id;type:varbinary(160);not null;index"`
	TenantID             string    `gorm:"column:tenant_id;type:varbinary(64);not null"`
	ConfigurationID      string    `gorm:"column:configuration_id;type:varbinary(64);not null;default:''"`
	ConfigurationVersion uint64    `gorm:"column:configuration_version;not null;default:0"`
	UserID               string    `gorm:"column:user_id;type:varbinary(64);not null;default:''"`
	Channel              string    `gorm:"column:channel;size:16;not null;default:''"`
	Outcome              string    `gorm:"column:outcome;size:40;not null"`
	CreatedAt            time.Time `gorm:"column:created_at;type:datetime(6);not null"`
}

func (routeOutcomeRecord) TableName() string { return "biz_notification_route_outcomes" }

type EventPublisher struct{ tx *gorm.DB }

var _ ports.BusinessEventPublisher = (*EventPublisher)(nil)

func NewEventPublisher(tx *gorm.DB) (*EventPublisher, error) {
	if tx == nil || tx.Statement == nil {
		return nil, domain.ErrRoutingUnavailable
	}
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return nil, errors.New("notification: root transaction required for business event")
	}
	return &EventPublisher{tx: tx}, nil
}

func (p *EventPublisher) AppendBusinessEvent(ctx context.Context, event domain.BusinessEvent) error {
	digest, err := event.Digest()
	if err != nil {
		return err
	}
	now, err := routingNow(ctx, p.tx)
	if err != nil {
		return err
	}
	event.OccurredAt = time.UnixMicro(event.OccurredAt.UTC().UnixMicro()).UTC()
	row := routingEventRecord{EventID: event.EventID, TenantID: event.TenantID, GroupID: event.GroupID, TypeCode: event.TypeCode, Level: string(event.Level), TraceID: event.TraceID, ReferenceKind: event.ReferenceKind, ReferenceID: event.ReferenceID, PayloadHash: digest, State: domain.RoutingStatePending, OccurredAt: event.OccurredAt, CreatedAt: now, UpdatedAt: now}
	result := p.tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	var existing routingEventRecord
	if err := p.tx.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("BINARY event_id=?", event.EventID).Take(&existing).Error; err != nil {
		return err
	}
	if existing.PayloadHash != digest {
		return domain.ErrRoutingConflict
	}
	return nil
}

type RoutingRepository struct{ db *gorm.DB }

var _ ports.RoutingQueue = (*RoutingRepository)(nil)
var _ ports.RoutingConfigurationReader = (*RoutingRepository)(nil)

func NewRoutingRepository(db *gorm.DB) (*RoutingRepository, error) {
	if db == nil {
		return nil, domain.ErrRoutingUnavailable
	}
	return &RoutingRepository{db: db}, nil
}
func MigrateRouting(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return domain.ErrRoutingUnavailable
	}
	return db.WithContext(ctx).AutoMigrate(&routingEventRecord{}, &inAppRecord{}, &externalTaskRecord{}, &routeOutcomeRecord{})
}
func routingNow(ctx context.Context, db *gorm.DB) (time.Time, error) {
	var row struct {
		Now time.Time `gorm:"column:now"`
	}
	if err := db.WithContext(ctx).Raw("SELECT UTC_TIMESTAMP(6) AS now").Scan(&row).Error; err != nil {
		return time.Time{}, err
	}
	if row.Now.IsZero() {
		return time.Time{}, domain.ErrRoutingUnavailable
	}
	return row.Now.UTC(), nil
}
func eventFromRecord(row routingEventRecord) domain.BusinessEvent {
	return domain.BusinessEvent{EventID: row.EventID, TenantID: row.TenantID, GroupID: row.GroupID, TypeCode: row.TypeCode, Level: domain.MessageLevel(row.Level), TraceID: row.TraceID, ReferenceKind: row.ReferenceKind, ReferenceID: row.ReferenceID, OccurredAt: row.OccurredAt.UTC()}
}

func (r *RoutingRepository) ClaimNextBusinessEvent(ctx context.Context, worker string, lease time.Duration) (domain.RoutingClaim, error) {
	if r == nil || r.db == nil || !domain.ValidConfigurationID(worker, 96) || lease < 5*time.Second || lease > 5*time.Minute {
		return domain.RoutingClaim{}, domain.ErrRoutingInvalid
	}
	var claim domain.RoutingClaim
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now, err := routingNow(ctx, tx)
		if err != nil {
			return err
		}
		var row routingEventRecord
		err = tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("state=? OR (state=? AND lease_until<=?)", domain.RoutingStatePending, domain.RoutingStateLeased, now).Order("occurred_at ASC,event_id ASC").First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if row.LeaseToken == ^uint64(0) {
			return domain.ErrRoutingUnavailable
		}
		row.Attempts++
		row.LeaseToken++
		until := time.UnixMicro(now.Add(lease).UnixMicro()).UTC()
		if err := tx.Model(&routingEventRecord{}).Where("event_id=?", row.EventID).Updates(map[string]any{"state": domain.RoutingStateLeased, "attempts": row.Attempts, "lease_owner": worker, "lease_token": row.LeaseToken, "lease_until": until, "updated_at": now}).Error; err != nil {
			return err
		}
		claim = domain.RoutingClaim{Event: eventFromRecord(row), WorkerID: worker, LeaseToken: row.LeaseToken, LeaseUntil: until, Attempt: row.Attempts}
		return nil
	})
	return claim, err
}

func (r *RoutingRepository) lockedEvent(ctx context.Context, tx *gorm.DB, claim domain.RoutingClaim) (routingEventRecord, time.Time, error) {
	if err := claim.Validate(); err != nil {
		return routingEventRecord{}, time.Time{}, err
	}
	var row routingEventRecord
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_id=?", claim.Event.EventID).Take(&row).Error; err != nil {
		return row, time.Time{}, err
	}
	now, err := routingNow(ctx, tx)
	if err != nil {
		return row, time.Time{}, err
	}
	if row.State == domain.RoutingStateRouted {
		return row, now, nil
	}
	if row.State != domain.RoutingStateLeased || row.LeaseOwner != claim.WorkerID || row.LeaseToken != claim.LeaseToken || row.LeaseUntil == nil || !now.Before(*row.LeaseUntil) {
		return row, now, domain.ErrRoutingLease
	}
	digest, err := claim.Event.Digest()
	if err != nil || digest != row.PayloadHash {
		return row, now, domain.ErrRoutingConflict
	}
	return row, now, nil
}

func (r *RoutingRepository) CompleteBusinessEventRoute(ctx context.Context, claim domain.RoutingClaim, plan domain.RoutePlan) (domain.RoutingResult, error) {
	canonical, err := plan.Canonical()
	if err != nil {
		return domain.RoutingResult{}, err
	}
	var result domain.RoutingResult
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, now, err := r.lockedEvent(ctx, tx, claim)
		if err != nil {
			return err
		}
		if row.State == domain.RoutingStateRouted {
			var readErr error
			result, readErr = r.routeResult(ctx, tx, row.EventID)
			return readErr
		}
		for _, d := range canonical.Decisions {
			outcome := routeOutcomeRecord{OutcomeID: domain.StableRoutingID(row.EventID, d.UserID, d.Channel, d.Outcome), EventID: row.EventID, TenantID: row.TenantID, ConfigurationID: d.ConfigurationID, ConfigurationVersion: d.ConfigurationVersion, UserID: d.UserID, Channel: d.Channel, Outcome: d.Outcome, CreatedAt: now}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&outcome).Error; err != nil {
				return err
			}
			switch d.Outcome {
			case domain.RouteOutcomeInAppCreated:
				message := inAppRecord{MessageID: domain.StableRoutingID("in_app", row.TenantID, row.EventID, d.UserID), TenantID: row.TenantID, UserID: d.UserID, EventID: row.EventID, ConfigurationID: d.ConfigurationID, ConfigurationVersion: d.ConfigurationVersion, GroupID: row.GroupID, TypeCode: row.TypeCode, Level: row.Level, TraceID: row.TraceID, ReferenceKind: row.ReferenceKind, ReferenceID: row.ReferenceID, CreatedAt: now}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&message).Error; err != nil {
					return err
				}
			case domain.RouteOutcomeExternalTask:
				task := externalTaskRecord{TaskID: domain.StableRoutingID("external", row.TenantID, row.EventID, d.UserID, d.Channel), TenantID: row.TenantID, UserID: d.UserID, EventID: row.EventID, Channel: d.Channel, ConfigurationID: d.ConfigurationID, ConfigurationVersion: d.ConfigurationVersion, GroupID: row.GroupID, TypeCode: row.TypeCode, Level: row.Level, TraceID: row.TraceID, ReferenceKind: row.ReferenceKind, ReferenceID: row.ReferenceID, State: domain.ExternalTaskStatePending, CreatedAt: now, UpdatedAt: now}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&task).Error; err != nil {
					return err
				}
			}
		}
		routed := now
		if err := tx.Model(&routingEventRecord{}).Where("event_id=?", row.EventID).Updates(map[string]any{"state": domain.RoutingStateRouted, "lease_owner": "", "lease_until": nil, "routed_at": routed, "updated_at": now}).Error; err != nil {
			return err
		}
		result, err = r.routeResult(ctx, tx, row.EventID)
		return err
	})
	return result, err
}

func (r *RoutingRepository) routeResult(ctx context.Context, tx *gorm.DB, eventID string) (domain.RoutingResult, error) {
	var row routingEventRecord
	if err := tx.WithContext(ctx).Where("event_id=?", eventID).Take(&row).Error; err != nil {
		return domain.RoutingResult{}, err
	}
	out := domain.RoutingResult{EventID: eventID, State: row.State}
	if row.RoutedAt != nil {
		out.RoutedAt = row.RoutedAt.UTC()
	}
	var counts []struct {
		Outcome string
		Count   uint64
	}
	if err := tx.WithContext(ctx).Table("biz_notification_route_outcomes").Select("outcome,COUNT(*) AS count").Where("event_id=?", eventID).Group("outcome").Scan(&counts).Error; err != nil {
		return out, err
	}
	for _, c := range counts {
		switch c.Outcome {
		case domain.RouteOutcomeInAppCreated:
			out.InAppCreated += c.Count
		case domain.RouteOutcomeExternalTask:
			out.ExternalTasks += c.Count
		case domain.RouteOutcomePreferenceDenied:
			out.Denied += c.Count
		case domain.RouteOutcomeChannelUnavailable:
			out.Unavailable += c.Count
		case domain.RouteOutcomeRecipientInactive:
			out.Inactive += c.Count
		case domain.RouteOutcomeNoConfiguration:
			out.NoConfiguration += c.Count
		case domain.RouteOutcomeTypeUnavailable:
			out.TypeUnavailable += c.Count
		}
	}
	return out, nil
}

func (r *RoutingRepository) FindRoutingConfiguration(ctx context.Context, tenant, group string, level domain.MessageLevel) (domain.Configuration, bool, error) {
	if r == nil || r.db == nil || !domain.ValidConfigurationID(tenant, 64) || !domain.ValidConfigurationID(group, 64) || !level.Valid() {
		return domain.Configuration{}, false, domain.ErrRoutingInvalid
	}
	var row configurationRecord
	err := r.db.WithContext(ctx).Where("BINARY tenant_id=? AND BINARY group_id=? AND level=?", tenant, group, string(level)).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.Configuration{}, false, nil
	}
	if err != nil {
		return domain.Configuration{}, false, err
	}
	c := domain.Configuration{ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, Level: domain.MessageLevel(row.Level), PrimaryUserID: row.PrimaryUserID, SecondaryUserID: row.SecondaryUserID, Notes: row.Notes, Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Channels: []string{}, AdditionalUserIDs: []string{}}
	if err := r.db.WithContext(ctx).Model(&configurationChannelRecord{}).Where("BINARY tenant_id=? AND BINARY configuration_id=?", tenant, row.ID).Order("channel ASC").Pluck("channel", &c.Channels).Error; err != nil {
		return domain.Configuration{}, false, err
	}
	if err := r.db.WithContext(ctx).Model(&configurationRecipientRecord{}).Where("BINARY tenant_id=? AND BINARY configuration_id=?", tenant, row.ID).Order("user_id ASC").Pluck("user_id", &c.AdditionalUserIDs).Error; err != nil {
		return domain.Configuration{}, false, err
	}
	if err := c.Values().Validate(); err != nil {
		return domain.Configuration{}, false, domain.ErrRoutingUnavailable
	}
	return c, true, nil
}
