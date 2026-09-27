package persistence

import (
	"context"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	accessports "github.com/hvritual/biz/internal/access/ports"
	"github.com/hvritual/biz/internal/deviceops/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"yunka.io/framework/core/identity"
)

// NotificationSiteDirectory borrows a root transaction. Identity and current
// action scope come from Access; SQL here only reads DeviceOps-owned sites.
// Neither a cached candidate list nor a caller-supplied site list can grant
// access. Every method resolves the current Access scope again.
type NotificationSiteDirectory struct {
	db         *gorm.DB
	scopes     accessports.NotificationManagementScopes
	permission string
}

var _ ports.NotificationSites = (*NotificationSiteDirectory)(nil)

// permission is selected by trusted composition for the generated operation,
// never by request JSON. This allowlist validates adapter configuration; it is
// not an Action Catalog or a grant and cannot authorize a caller by itself.
func NewNotificationSiteDirectory(db *gorm.DB, scopes accessports.NotificationManagementScopes, permission string) (*NotificationSiteDirectory, error) {
	if db == nil || scopes == nil || !notificationSitePermission(permission) {
		return nil, ports.ErrNotificationSiteDirectoryUnavailable
	}
	return &NotificationSiteDirectory{db: db, scopes: scopes, permission: permission}, nil
}

func notificationSitePermission(value string) bool {
	switch value {
	case "tenant.notification.read", "tenant.notification.create", "tenant.notification.update", "tenant.notification.delete":
		return true
	default:
		return false
	}
}

func (d *NotificationSiteDirectory) currentScope(ctx context.Context, lock bool) (string, []string, error) {
	if d == nil || d.db == nil || d.db.Statement == nil || d.scopes == nil {
		return "", nil, ports.ErrNotificationSiteDirectoryUnavailable
	}
	// Even read calls must borrow a root transaction: they must not open their
	// own UoW or allow a mutation caller to accidentally use an autocommit DB.
	if _, ok := d.db.Statement.ConnPool.(gorm.TxCommitter); !ok {
		return "", nil, ports.ErrNotificationSiteDirectoryUnavailable
	}
	if ctx == nil {
		return "", nil, ports.ErrNotificationSiteScopeDenied
	}
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}
	p, ok := identity.FromContext(ctx)
	if !ok || !p.Authenticated || p.TenantID == "" || p.UserID == "" {
		return "", nil, ports.ErrNotificationSiteScopeDenied
	}
	current, err := d.scopes.ManagementScope(ctx, p.TenantID, p.UserID, d.permission, lock)
	if err != nil {
		return "", nil, err
	}
	if !current.All && !current.Self && !current.Sites {
		return "", nil, ports.ErrNotificationSiteScopeDenied
	}
	// #184's approved group contract requires explicit member-site membership
	// as a ceiling. ALL/SELF is action eligibility, not permission to manufacture
	// a notification group outside that explicit scope. This consumption rule
	// does not change legacy DeviceOps scope behavior for other operations.
	allowed := append([]string{}, current.MemberSiteIDs...)
	if current.PolicyBound {
		policy := make(map[string]struct{}, len(current.PolicySiteIDs))
		for _, id := range current.PolicySiteIDs {
			policy[id] = struct{}{}
		}
		filtered := make([]string, 0, len(allowed))
		for _, id := range allowed {
			if _, ok := policy[id]; ok {
				filtered = append(filtered, id)
			}
		}
		allowed = filtered
	}
	sort.Strings(allowed)
	return p.TenantID, allowed, nil
}

func notificationSiteID(value string) bool {
	if len(value) == 0 || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

// List computes both the filtered total and the requested page in a single
// MySQL statement. An out-of-range page still returns the correct total; there
// is no count/list race even under READ COMMITTED. SQL parameters stay bound.
func (d *NotificationSiteDirectory) List(ctx context.Context, query string, page, size int) ([]ports.NotificationSite, uint64, error) {
	if page < 1 || page > 1000000 || size < 1 || size > 100 ||
		!utf8.ValidString(query) || utf8.RuneCountInString(query) > 200 || strings.IndexFunc(query, unicode.IsControl) >= 0 {
		return nil, 0, ports.ErrNotificationSiteQueryInvalid
	}
	tenant, allowed, err := d.currentScope(ctx, false)
	if err != nil {
		return nil, 0, err
	}
	out := make([]ports.NotificationSite, 0)
	if len(allowed) == 0 {
		return out, 0, nil
	}
	var rows []struct {
		Total   uint64
		ID      *string
		Name    *string
		Version *uint64
	}
	err = d.db.WithContext(ctx).Raw(`WITH visible AS (
        SELECT id, name, version FROM biz_deviceops_site
        WHERE BINARY tenant_id = ? AND deleted_at IS NULL AND BINARY id IN ?
          AND (? = '' OR LOCATE(BINARY ?, BINARY name) > 0 OR LOCATE(BINARY ?, BINARY id) > 0)
    ), page_rows AS (
        SELECT id, name, version FROM visible
        ORDER BY BINARY name, BINARY id LIMIT ? OFFSET ?
    )
    SELECT totals.total, page_rows.id, page_rows.name, page_rows.version
    FROM (SELECT COUNT(*) AS total FROM visible) totals
    LEFT JOIN page_rows ON TRUE ORDER BY BINARY page_rows.name, BINARY page_rows.id`,
		tenant, allowed, query, query, query, size, (page-1)*size).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return nil, 0, ports.ErrNotificationSiteDirectoryUnavailable
	}
	total := rows[0].Total
	for _, row := range rows {
		if row.Total != total {
			return nil, 0, ports.ErrNotificationSiteDirectoryUnavailable
		}
		if row.ID == nil {
			continue // the totals-only row when no page item exists
		}
		if row.Name == nil || row.Version == nil || *row.Version == 0 {
			return nil, 0, ports.ErrNotificationSiteDirectoryUnavailable
		}
		out = append(out, ports.NotificationSite{ID: *row.ID, Name: *row.Name, Version: *row.Version})
	}
	return out, total, nil
}

// Resolve validates the entire set. Missing, retired, foreign and out-of-scope
// sites all use the same not-found result and never return a partial list. Write
// callers request shared row locks held until their root transaction completes.
func (d *NotificationSiteDirectory) Resolve(ctx context.Context, ids []string, lock bool) ([]ports.NotificationSite, error) {
	if len(ids) == 0 || len(ids) > 100 {
		return nil, ports.ErrNotificationSiteQueryInvalid
	}
	sorted := append([]string{}, ids...)
	sort.Strings(sorted)
	for i, id := range sorted {
		if !notificationSiteID(id) || (i > 0 && id == sorted[i-1]) {
			return nil, ports.ErrNotificationSiteQueryInvalid
		}
	}
	tenant, allowed, err := d.currentScope(ctx, lock)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(allowed))
	for _, id := range allowed {
		set[id] = struct{}{}
	}
	for _, id := range sorted {
		if _, ok := set[id]; !ok {
			return nil, ports.ErrNotFound
		}
	}
	q := d.db.WithContext(ctx).Model(&SitePORecord{}).
		Select("id", "name", "version").
		Where("BINARY tenant_id = ? AND BINARY id IN ?", tenant, sorted).
		Order("id ASC")
	if lock {
		q = q.Clauses(clause.Locking{Strength: "SHARE"})
	}
	var rows []SitePORecord
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != len(sorted) {
		return nil, ports.ErrNotFound
	}
	out := make([]ports.NotificationSite, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if row.Version == 0 {
			return nil, ports.ErrNotificationSiteDirectoryUnavailable
		}
		if _, ok := set[row.ID]; !ok {
			return nil, ports.ErrNotFound
		}
		if _, duplicate := seen[row.ID]; duplicate {
			return nil, ports.ErrNotificationSiteDirectoryUnavailable
		}
		seen[row.ID] = struct{}{}
		out = append(out, ports.NotificationSite{ID: row.ID, Name: row.Name, Version: row.Version})
	}
	for _, id := range sorted {
		if _, ok := seen[id]; !ok {
			return nil, ports.ErrNotFound
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// CurrentIDs resolves the resource ceiling without inventing a page-size cap on
// the actor's authorized scope. Returned IDs are not an authorization cache.
func (d *NotificationSiteDirectory) CurrentIDs(ctx context.Context) ([]string, error) {
	tenant, allowed, err := d.currentScope(ctx, false)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0)
	if len(allowed) == 0 {
		return ids, nil
	}
	err = d.db.WithContext(ctx).Model(&SitePORecord{}).Where("BINARY tenant_id = ? AND BINARY id IN ?", tenant, allowed).Order("id ASC").Pluck("id", &ids).Error
	return ids, err
}
