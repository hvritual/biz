package ports

import (
	"context"
	"time"

	transition "github.com/hvritual/biz/internal/commercial/domain/timetransition"
)

type TimeTransitionRepository interface {
	Now(context.Context) (time.Time, error)
	Insert(context.Context, transition.Task) error
	ClaimDue(context.Context, string, time.Time, time.Duration) (*transition.Task, error)
	Lock(context.Context, string) (*transition.Task, error)
	Save(context.Context, transition.Task, transition.Task) error
}
