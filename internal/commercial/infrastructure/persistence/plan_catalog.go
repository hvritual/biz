package persistence

import (
	"context"

	"github.com/hvritual/biz/internal/commercial/domain/plan"
)

type planCatalogRow struct {
	PlanCode      string `gorm:"column:plan_code"`
	HeadRevision  uint64 `gorm:"column:head_revision"`
	Version       uint64 `gorm:"column:version"`
	Revision      uint64 `gorm:"column:revision"`
	State         string `gorm:"column:state"`
	ContentSHA256 string `gorm:"column:content_sha256"`
	Payload       string `gorm:"column:payload"`
}

func (r *planRepository) Catalog(ctx context.Context, after string, limit int) ([]plan.Version, error) {
	var rows []planCatalogRow
	query := r.tx.WithContext(ctx).
		Table("biz_commercial_plans AS p").
		Select("p.plan_code, p.revision AS head_revision, v.version, v.revision, v.state, v.content_sha256, v.payload").
		Joins("JOIN biz_commercial_plan_versions AS v ON v.plan_code = p.plan_code AND v.version = p.latest_version").
		Where("p.latest_version > 0")
	if after != "" {
		query = query.Where("p.plan_code > ?", after)
	}
	if err := query.Order("p.plan_code ASC").Limit(limit).Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]plan.Version, 0, len(rows))
	for _, row := range rows {
		value, err := decodePlan(planVersionRow{
			PlanCode:      row.PlanCode,
			Version:       row.Version,
			Revision:      row.Revision,
			State:         row.State,
			ContentSHA256: row.ContentSHA256,
			Payload:       row.Payload,
		})
		if err != nil {
			return nil, err
		}
		if row.HeadRevision < value.PlanRevision {
			return nil, plan.ErrCorrupt
		}
		value.PlanRevision = row.HeadRevision
		out = append(out, value)
	}
	return out, nil
}
