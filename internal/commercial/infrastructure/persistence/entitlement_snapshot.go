package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"github.com/hvritual/biz/internal/commercial/domain/snapshot"
	"github.com/hvritual/biz/internal/commercial/infrastructure/consistency"
	"github.com/hvritual/biz/internal/commercial/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"time"
	"yunka.io/framework/execution"
)

// SnapshotCatalogProjection is a read-only infrastructure seam owned by
// ModuleCatalog. The supplied transaction is the existing root or a standalone
// read-model transaction, never an independently opened child transaction.
type SnapshotCatalogProjection interface {
	ReadSnapshotCatalog(context.Context, *gorm.DB) (entitlement.Catalog, error)
}
type snapshotHead struct {
	TenantID        string `gorm:"column:tenant_id;primaryKey"`
	Version         uint64 `gorm:"column:version"`
	SourceVersion   uint64 `gorm:"column:source_version"`
	CatalogRevision uint64 `gorm:"column:catalog_revision"`
	InputHash       string `gorm:"column:input_hash"`
	PayloadSHA256   string `gorm:"column:payload_sha256"`
	Invalidated     bool   `gorm:"column:invalidated"`
}

func (snapshotHead) TableName() string { return "biz_commercial_entitlement_snapshot_heads" }

type snapshotRow struct {
	TenantID      string    `gorm:"column:tenant_id;primaryKey"`
	Version       uint64    `gorm:"column:version;primaryKey"`
	Payload       string    `gorm:"column:payload"`
	PayloadSHA256 string    `gorm:"column:payload_sha256"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (snapshotRow) TableName() string { return "biz_commercial_entitlement_snapshots" }

type SnapshotStore struct {
	db      *gorm.DB
	catalog SnapshotCatalogProjection
	cache   ports.SnapshotCache
}

func NewSnapshotStore(db *gorm.DB, catalog SnapshotCatalogProjection, cache ports.SnapshotCache) (*SnapshotStore, error) {
	if db == nil || catalog == nil {
		return nil, errors.New("commercial: snapshot authority required")
	}
	return &SnapshotStore{db: db, catalog: catalog, cache: cache}, nil
}
func cacheKey(tenant string, version uint64, digest string) string {
	return fmt.Sprintf("%s/%d/%s", tenant, version, digest)
}
func (s *SnapshotStore) ReadSnapshot(ctx context.Context, tenant string, requested []string) (entitlement.Result, error) {
	if tenant == "" || len(tenant) > 64 || len(requested) > 128 {
		return entitlement.Result{}, entitlement.ErrInvalid
	}
	frame, joined := execution.Current(ctx)
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ctx = bounded
	if joined && frame.Transaction == execution.TransactionReadOnly {
		// No write or lock upgrade in a declared read-only root. Preflight has built
		// its view; a concurrent boundary may cause a safe retry, never stale allow.
		return s.readOnly(ctx, tenant, requested)
	}
	var result entitlement.Result
	err := consistency.Within(ctx, s.db, 2, func(tx *gorm.DB) error { var err error; result, err = s.locked(ctx, tx, tenant); return err })
	if err != nil {
		return entitlement.Result{}, err
	}
	// Publish only after an owned transaction committed. Joined-root results can
	// still roll back, so they are never published to the shared cache here.
	if !joined && s.cache != nil {
		if b, err := json.Marshal(result); err == nil {
			_ = s.cache.Put(ctx, cacheKey(tenant, result.EntitlementVersion, snapshot.Digest(b)), b)
		}
	}
	return snapshot.Requested(result, requested), nil
}
func (s *SnapshotStore) locked(ctx context.Context, tx *gorm.DB, tenant string) (entitlement.Result, error) {
	epoch, err := consistency.LockCatalog(tx, false)
	if err != nil {
		return entitlement.Result{}, err
	}
	repo := &entitlementRepository{tx: tx.Session(&gorm.Session{SkipDefaultTransaction: true})}
	state, err := repo.Lock(ctx, tenant)
	if err != nil {
		return entitlement.Result{}, err
	}
	// Current locking reads defeat an earlier REPEATABLE READ image in this root.
	catalog, err := s.catalog.ReadSnapshotCatalog(ctx, tx)
	if err != nil {
		return entitlement.Result{}, err
	}
	at, err := consistency.Now(tx)
	if err != nil {
		return entitlement.Result{}, err
	}
	input, err := snapshot.InputHash(catalog, state.Sources)
	if err != nil {
		return entitlement.Result{}, err
	}
	var head snapshotHead
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=?", tenant).First(&head).Error
	exists := !errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && exists {
		return entitlement.Result{}, err
	}
	var latest snapshotRow
	err = tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("tenant_id=?", tenant).Order("version DESC").First(&latest).Error
	history := !errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && history {
		return entitlement.Result{}, err
	}
	if exists != history || (exists && (head.Version != latest.Version || state.Version < head.SourceVersion || epoch < head.CatalogRevision)) {
		return entitlement.Result{}, consistency.ErrStateLost
	}
	if exists && !head.Invalidated && head.SourceVersion == state.Version && head.CatalogRevision == epoch && head.InputHash == input {
		result, err := s.decode(ctx, head, latest, at)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, snapshot.ErrStale) {
			return entitlement.Result{}, err
		}
	}
	if exists && head.Version == math.MaxUint64 {
		return entitlement.Result{}, consistency.ErrStateLost
	}
	result, err := entitlement.Resolve(tenant, state.Version, at, catalog, state.Sources, nil)
	if err != nil {
		return entitlement.Result{}, err
	}
	result.EntitlementVersion = head.Version + 1
	result.CatalogRevision = epoch
	b, err := json.Marshal(result)
	if err != nil {
		return entitlement.Result{}, err
	}
	digest := snapshot.Digest(b)
	row := snapshotRow{TenantID: tenant, Version: result.EntitlementVersion, Payload: string(b), PayloadSHA256: digest, CreatedAt: at}
	if err := tx.Create(&row).Error; err != nil {
		return entitlement.Result{}, err
	}
	next := snapshotHead{TenantID: tenant, Version: row.Version, SourceVersion: state.Version, CatalogRevision: epoch, InputHash: input, PayloadSHA256: digest}
	if exists {
		res := tx.Model(&snapshotHead{}).Where("tenant_id=? AND version=?", tenant, head.Version).Updates(map[string]any{"version": next.Version, "source_version": next.SourceVersion, "catalog_revision": epoch, "input_hash": input, "payload_sha256": digest, "invalidated": false})
		if res.Error != nil {
			return entitlement.Result{}, res.Error
		}
		if res.RowsAffected != 1 {
			return entitlement.Result{}, consistency.ErrStateLost
		}
	} else if err := tx.Create(&next).Error; err != nil {
		return entitlement.Result{}, err
	}
	return result, nil
}
func (s *SnapshotStore) decode(ctx context.Context, h snapshotHead, row snapshotRow, at time.Time) (entitlement.Result, error) {
	if row.TenantID != h.TenantID || row.Version != h.Version || row.PayloadSHA256 != h.PayloadSHA256 {
		return entitlement.Result{}, snapshot.ErrInvalid
	}
	if s.cache != nil {
		if b, err := s.cache.Get(ctx, cacheKey(h.TenantID, h.Version, h.PayloadSHA256)); err == nil && len(b) > 0 {
			if r, err := snapshot.Decode(b, h.TenantID, h.Version, h.SourceVersion, h.CatalogRevision, h.PayloadSHA256, at); err == nil {
				return r, nil
			}
		}
	}
	return snapshot.Decode([]byte(row.Payload), h.TenantID, h.Version, h.SourceVersion, h.CatalogRevision, h.PayloadSHA256, at)
}
func (s *SnapshotStore) readOnly(ctx context.Context, tenant string, requested []string) (entitlement.Result, error) {
	db, err := consistency.DB(ctx, nil)
	if err != nil {
		return entitlement.Result{}, err
	}
	var row struct {
		Version, SourceVersion, CatalogRevision, CurrentSource, CurrentCatalog uint64
		Invalidated                                                            bool
		PayloadSHA256, Payload                                                 string
		At                                                                     time.Time
	}
	// One statement binds every pointer/stamp and DB time; no global write lock.
	r := db.Raw(`SELECT h.version,h.source_version,h.catalog_revision,h.invalidated,h.payload_sha256,p.payload,
 s.version AS current_source,c.version AS current_catalog,UTC_TIMESTAMP(6) AS at
 FROM biz_commercial_entitlement_snapshot_heads h
 JOIN biz_commercial_entitlement_snapshots p ON p.tenant_id=h.tenant_id AND p.version=h.version AND p.payload_sha256=h.payload_sha256
 JOIN biz_commercial_entitlement_state s ON s.tenant_id=h.tenant_id
 JOIN biz_commercial_catalog_state c ON c.id=1 WHERE h.tenant_id=?`, tenant).Scan(&row)
	if r.Error != nil {
		return entitlement.Result{}, r.Error
	}
	if r.RowsAffected != 1 {
		return entitlement.Result{}, snapshot.ErrStale
	}
	if row.Invalidated || row.SourceVersion != row.CurrentSource || row.CatalogRevision != row.CurrentCatalog {
		return entitlement.Result{}, snapshot.ErrStale
	}
	result, err := snapshot.Decode([]byte(row.Payload), tenant, row.Version, row.SourceVersion, row.CatalogRevision, row.PayloadSHA256, row.At)
	return snapshot.Requested(result, requested), err
}
