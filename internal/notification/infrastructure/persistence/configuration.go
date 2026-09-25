package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/core/identity"
)

type configurationRecord struct {
	ID              string    `gorm:"column:id;primaryKey;type:varbinary(64)"`
	TenantID        string    `gorm:"column:tenant_id;type:varbinary(64);not null;uniqueIndex:uq_notification_configuration,priority:1"`
	GroupID         string    `gorm:"column:group_id;type:varbinary(64);not null;uniqueIndex:uq_notification_configuration,priority:2"`
	Level           string    `gorm:"column:level;size:16;not null;uniqueIndex:uq_notification_configuration,priority:3"`
	PrimaryUserID   string    `gorm:"column:primary_user_id;type:varbinary(64);not null"`
	SecondaryUserID string    `gorm:"column:secondary_user_id;type:varbinary(64);not null"`
	Notes           string    `gorm:"column:notes;type:text;not null"`
	Version         uint64    `gorm:"column:version;not null"`
	CreatedAt       time.Time `gorm:"column:created_at;not null"`
	UpdatedAt       time.Time `gorm:"column:updated_at;not null"`
}

func (configurationRecord) TableName() string { return "biz_notification_configurations" }

type configurationChannelRecord struct {
	TenantID        string `gorm:"column:tenant_id;primaryKey;type:varbinary(64)"`
	ConfigurationID string `gorm:"column:configuration_id;primaryKey;type:varbinary(64)"`
	Code            string `gorm:"column:channel;primaryKey;size:64"`
}

func (configurationChannelRecord) TableName() string {
	return "biz_notification_configuration_channels"
}

type configurationRecipientRecord struct {
	TenantID        string `gorm:"column:tenant_id;primaryKey;type:varbinary(64)"`
	ConfigurationID string `gorm:"column:configuration_id;primaryKey;type:varbinary(64)"`
	UserID          string `gorm:"column:user_id;primaryKey;type:varbinary(64);index:idx_notification_recipient"`
}

func (configurationRecipientRecord) TableName() string {
	return "biz_notification_configuration_recipients"
}

type ConfigurationRepository struct{ tx *gorm.DB }

var _ ports.ConfigurationRepository = (*ConfigurationRepository)(nil)

func NewConfigurationRepository(tx *gorm.DB) (*ConfigurationRepository, error) {
	if tx == nil || tx.Statement == nil {
		return nil, domain.ErrConfigurationUnavailable
	}
	// Row locks and an all-or-none multi-level write are not safe in autocommit.
	if _, ok := tx.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return nil, errors.New("notification: root transaction required")
	}
	return &ConfigurationRepository{tx: tx}, nil
}
func MigrateConfigurations(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return domain.ErrConfigurationUnavailable
	}
	return db.WithContext(ctx).AutoMigrate(&configurationRecord{}, &configurationChannelRecord{}, &configurationRecipientRecord{}, &configurationReceiptRecord{})
}
func (r *ConfigurationRepository) scoped(ctx context.Context, tenant string) (*gorm.DB, error) {
	if ctx == nil {
		return nil, domain.ErrConfigurationNotFound
	}
	p, ok := identity.FromContext(ctx)
	if r == nil || r.tx == nil {
		return nil, domain.ErrConfigurationUnavailable
	}
	if !ok || !p.Authenticated || p.TenantID != tenant || !domain.ValidConfigurationID(tenant, 64) || p.UserID == "" {
		return nil, domain.ErrConfigurationNotFound
	}
	return r.tx.WithContext(ctx), nil
}
func configurationDBError(err error) error {
	var e *mysql.MySQLError
	if errors.As(err, &e) && e.Number == 1062 {
		return domain.ErrConfigurationDuplicate
	}
	return err
}
func (r *ConfigurationRepository) Get(ctx context.Context, tenant, id string, lock bool) (domain.Configuration, error) {
	tx, err := r.scoped(ctx, tenant)
	if err != nil {
		return domain.Configuration{}, err
	}
	if !domain.ValidConfigurationID(id, 64) {
		return domain.Configuration{}, domain.ErrConfigurationNotFound
	}
	q := tx.Where("BINARY tenant_id = ? AND BINARY id = ?", tenant, id)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row configurationRecord
	if err := q.Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err = domain.ErrConfigurationNotFound
		}
		return domain.Configuration{}, err
	}
	return r.hydrate(ctx, row, lock)
}
func (r *ConfigurationRepository) hydrate(ctx context.Context, row configurationRecord, lock bool) (domain.Configuration, error) {
	c := domain.Configuration{ID: row.ID, TenantID: row.TenantID, GroupID: row.GroupID, Level: domain.MessageLevel(row.Level), PrimaryUserID: row.PrimaryUserID, SecondaryUserID: row.SecondaryUserID, Notes: row.Notes, Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Channels: []string{}, AdditionalUserIDs: []string{}}
	if !c.Level.Valid() || c.Version == 0 {
		return domain.Configuration{}, domain.ErrConfigurationUnavailable
	}
	tx, err := r.scoped(ctx, row.TenantID)
	if err != nil {
		return domain.Configuration{}, err
	}
	channels := tx.Model(&configurationChannelRecord{})
	if lock {
		channels = channels.Clauses(clause.Locking{Strength: "SHARE"})
	}
	if err := channels.Where("BINARY tenant_id = ? AND BINARY configuration_id = ?", row.TenantID, row.ID).Order("channel ASC").Pluck("channel", &c.Channels).Error; err != nil {
		return domain.Configuration{}, err
	}
	recipients := tx.Model(&configurationRecipientRecord{})
	if lock {
		recipients = recipients.Clauses(clause.Locking{Strength: "SHARE"})
	}
	if err := recipients.Where("BINARY tenant_id = ? AND BINARY configuration_id = ?", row.TenantID, row.ID).Order("user_id ASC").Pluck("user_id", &c.AdditionalUserIDs).Error; err != nil {
		return domain.Configuration{}, err
	}
	if err := c.Values().Validate(); err != nil {
		return domain.Configuration{}, domain.ErrConfigurationUnavailable
	}
	return c, nil
}
func (r *ConfigurationRepository) List(ctx context.Context, tenant string, f domain.ConfigurationFilter) ([]domain.Configuration, uint64, error) {
	tx, err := r.scoped(ctx, tenant)
	if err != nil {
		return nil, 0, err
	}
	if err = f.Validate(); err != nil {
		return nil, 0, err
	}
	// A missing/empty authorized group set never means unrestricted access.
	if len(f.GroupIDs) == 0 {
		return []domain.Configuration{}, 0, nil
	}

	where := "BINARY c.tenant_id = ? AND BINARY c.group_id IN ?"
	args := []any{tenant, f.GroupIDs}
	if f.Level != "" {
		where += " AND c.level = ?"
		args = append(args, f.Level)
	}
	if f.RecipientID != "" {
		where += " AND (BINARY c.primary_user_id = ? OR BINARY c.secondary_user_id = ? OR EXISTS (SELECT 1 FROM biz_notification_configuration_recipients nr WHERE nr.tenant_id=c.tenant_id AND nr.configuration_id=c.id AND BINARY nr.user_id=?))"
		args = append(args, f.RecipientID, f.RecipientID, f.RecipientID)
	}
	args = append(args, f.PageSize, (f.Page-1)*f.PageSize)
	var rows []struct {
		Record         configurationRecord `gorm:"embedded"`
		Total          uint64
		ChannelsJSON   string
		RecipientsJSON string
	}
	// One statement binds count, page, and relationship collections to one MySQL
	// snapshot, including READ COMMITTED and pages beyond the last item.
	query := `WITH visible AS (
  SELECT c.* FROM biz_notification_configurations c WHERE ` + where + `
 ), page_rows AS (
  SELECT * FROM visible ORDER BY created_at DESC, id ASC LIMIT ? OFFSET ?
 )
 SELECT totals.total, page_rows.*,
 COALESCE((SELECT JSON_ARRAYAGG(nc.channel) FROM biz_notification_configuration_channels nc WHERE nc.tenant_id=page_rows.tenant_id AND nc.configuration_id=page_rows.id), JSON_ARRAY()) AS channels_json,
 COALESCE((SELECT JSON_ARRAYAGG(CONVERT(nr.user_id USING utf8mb4)) FROM biz_notification_configuration_recipients nr WHERE nr.tenant_id=page_rows.tenant_id AND nr.configuration_id=page_rows.id), JSON_ARRAY()) AS recipients_json
 FROM (SELECT COUNT(*) AS total FROM visible) totals LEFT JOIN page_rows ON TRUE
 ORDER BY page_rows.created_at DESC, page_rows.id ASC`
	if err := tx.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return nil, 0, domain.ErrConfigurationUnavailable
	}
	items := make([]domain.Configuration, 0, len(rows))
	total := rows[0].Total
	for _, row := range rows {
		if row.Total != total {
			return nil, 0, domain.ErrConfigurationUnavailable
		}
		record := row.Record
		if record.ID == "" {
			continue
		}
		c := domain.Configuration{ID: record.ID, TenantID: record.TenantID, GroupID: record.GroupID, Level: domain.MessageLevel(record.Level), PrimaryUserID: record.PrimaryUserID, SecondaryUserID: record.SecondaryUserID, Notes: record.Notes, Version: record.Version, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}
		if err := json.Unmarshal([]byte(row.ChannelsJSON), &c.Channels); err != nil {
			return nil, 0, domain.ErrConfigurationUnavailable
		}
		if err := json.Unmarshal([]byte(row.RecipientsJSON), &c.AdditionalUserIDs); err != nil {
			return nil, 0, domain.ErrConfigurationUnavailable
		}
		sort.Strings(c.Channels)
		sort.Strings(c.AdditionalUserIDs)
		if !c.Level.Valid() || c.Version == 0 {
			return nil, 0, domain.ErrConfigurationUnavailable
		}
		if err := c.Values().Validate(); err != nil {
			return nil, 0, domain.ErrConfigurationUnavailable
		}
		items = append(items, c)
	}
	return items, total, nil
}

func (r *ConfigurationRepository) CreateMany(ctx context.Context, configs []domain.Configuration) error {
	if len(configs) < 1 || len(configs) > 3 {
		return domain.ErrConfigurationInvalid
	}
	first := configs[0]
	tx, err := r.scoped(ctx, first.TenantID)
	if err != nil {
		return err
	}
	configs = append([]domain.Configuration{}, configs...)
	sort.Slice(configs, func(i, j int) bool { return configs[i].Level < configs[j].Level })
	seen := map[domain.MessageLevel]bool{}
	// Validate the entire batch before the first SQL mutation.
	for _, c := range configs {
		if c.TenantID != first.TenantID || c.GroupID != first.GroupID || !domain.ValidConfigurationID(c.ID, 64) || !domain.ValidConfigurationID(c.GroupID, 64) || !c.Level.Valid() || seen[c.Level] || c.Version != 1 || c.Deleted || c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
			return domain.ErrConfigurationInvalid
		}
		if err := c.Values().Validate(); err != nil {
			return err
		}
		seen[c.Level] = true
	}
	for _, c := range configs {
		row := configurationRecord{ID: c.ID, TenantID: c.TenantID, GroupID: c.GroupID, Level: string(c.Level), PrimaryUserID: c.PrimaryUserID, SecondaryUserID: c.SecondaryUserID, Notes: c.Notes, Version: 1, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
		if err := tx.Create(&row).Error; err != nil {
			return configurationDBError(err)
		}
		if err := r.writeRelations(ctx, c); err != nil {
			return err
		}
	}
	return nil
}
func (r *ConfigurationRepository) writeRelations(ctx context.Context, c domain.Configuration) error {
	tx, err := r.scoped(ctx, c.TenantID)
	if err != nil {
		return err
	}
	for _, code := range c.Channels {
		if err := tx.Create(&configurationChannelRecord{TenantID: c.TenantID, ConfigurationID: c.ID, Code: code}).Error; err != nil {
			return configurationDBError(err)
		}
	}
	for _, id := range c.AdditionalUserIDs {
		if err := tx.Create(&configurationRecipientRecord{TenantID: c.TenantID, ConfigurationID: c.ID, UserID: id}).Error; err != nil {
			return configurationDBError(err)
		}
	}
	return nil
}
func (r *ConfigurationRepository) deleteRelations(ctx context.Context, tenant, id string) error {
	tx, err := r.scoped(ctx, tenant)
	if err != nil {
		return err
	}
	if err := tx.Where("BINARY tenant_id = ? AND BINARY configuration_id = ?", tenant, id).Delete(&configurationChannelRecord{}).Error; err != nil {
		return err
	}
	return tx.Where("BINARY tenant_id = ? AND BINARY configuration_id = ?", tenant, id).Delete(&configurationRecipientRecord{}).Error
}
func (r *ConfigurationRepository) Replace(ctx context.Context, c domain.Configuration, expected uint64) error {
	if err := c.Values().Validate(); err != nil {
		return err
	}
	if c.UpdatedAt.IsZero() {
		return domain.ErrConfigurationInvalid
	}
	if expected == 0 || expected == ^uint64(0) || c.Version != expected+1 || c.Deleted {
		return domain.ErrConfigurationInvalid
	}
	tx, err := r.scoped(ctx, c.TenantID)
	if err != nil {
		return err
	}
	updated := tx.Model(&configurationRecord{}).Where("BINARY tenant_id = ? AND BINARY id = ? AND BINARY group_id = ? AND level = ? AND version = ?", c.TenantID, c.ID, c.GroupID, c.Level, expected).Updates(map[string]any{"primary_user_id": c.PrimaryUserID, "secondary_user_id": c.SecondaryUserID, "notes": c.Notes, "version": c.Version, "updated_at": c.UpdatedAt})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return domain.ErrConfigurationConflict
	}
	if err := r.deleteRelations(ctx, c.TenantID, c.ID); err != nil {
		return err
	}
	return r.writeRelations(ctx, c)
}
func (r *ConfigurationRepository) Delete(ctx context.Context, tenant, id string, expected uint64) error {
	if expected == 0 || expected == ^uint64(0) {
		return domain.ErrConfigurationInvalid
	}
	current, err := r.Get(ctx, tenant, id, true)
	if err != nil {
		return err
	}
	if current.Version != expected {
		return domain.ErrConfigurationConflict
	}
	tx, err := r.scoped(ctx, tenant)
	if err != nil {
		return err
	}
	if err := r.deleteRelations(ctx, tenant, id); err != nil {
		return err
	}
	deleted := tx.Where("BINARY tenant_id = ? AND BINARY id = ? AND version = ?", tenant, id, expected).Delete(&configurationRecord{})
	if deleted.Error != nil {
		return deleted.Error
	}
	if deleted.RowsAffected != 1 {
		return domain.ErrConfigurationConflict
	}
	return nil
}

func exactDigest(value string) bool {
	return len(value) == 64 && strings.IndexFunc(value, func(r rune) bool { return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') }) < 0
}
