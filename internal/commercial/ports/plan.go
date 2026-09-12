package ports

import (
	"context"
	"github.com/hvritual/biz/internal/commercial/domain/plan"
	"time"
)

type PlanHead struct {
	Code     string
	Revision uint64
	Latest   uint64
}
type PlanRepository interface {
	Lock(context.Context, string) (PlanHead, error)
	Get(context.Context, string, uint64, bool) (plan.Version, error)
	Catalog(context.Context, string, int) ([]plan.Version, error)
	List(context.Context, string, uint64, int) ([]plan.Version, error)
	Receipt(context.Context, string, string, string) (*plan.Version, error)
	Save(context.Context, PlanHead, *plan.Version, plan.Version) error
	Complete(context.Context, string, string, string, *plan.Version, plan.Version, time.Time) error
	Now(context.Context) (time.Time, error)
}
type PlanRepositories struct{ Plans PlanRepository }
