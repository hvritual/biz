package usecase

import (
	"context"
	"errors"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	app "github.com/hvritual/biz/internal/commercial/application"
	p "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	"github.com/hvritual/biz/internal/commercial/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	"time"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/execution"
	"yunka.io/framework/requestscope"
)

type service struct {
	repositories requestscope.RepositoryFactory[ports.ProvisioningRepositories]
	capabilities app.ProvisioningCapabilities
}

func New(r requestscope.RepositoryFactory[ports.ProvisioningRepositories], c app.ProvisioningCapabilities) (app.ProvisioningApplication, error) {
	if r == nil || c == nil || c.CommercialSubscriptionChanges() == nil {
		return nil, errors.New("provisioning: repository and typed change capability required")
	}
	return &service{r, c}, nil
}
func actor(ctx context.Context) (string, error) {
	v, ok := identity.FromContext(ctx)
	if !ok || !v.Authenticated || v.Subject == "" || len(v.Subject) > 200 || v.TenantID != "" {
		return "", p.ErrScope
	}
	return v.Subject, nil
}
func expose(err error) error {
	if err == nil {
		return nil
	}
	code, reason := codes.Unavailable, "PROVISIONING_STORAGE_UNAVAILABLE"
	switch {
	case errors.Is(err, p.ErrInvalid):
		code = codes.InvalidArgument
		reason = p.ErrInvalid.Error()
	case errors.Is(err, p.ErrScope):
		code = codes.PermissionDenied
		reason = p.ErrScope.Error()
	case errors.Is(err, p.ErrNotFound):
		code = codes.NotFound
		reason = p.ErrNotFound.Error()
	case errors.Is(err, p.ErrConflict), errors.Is(err, p.ErrKeyConflict), errors.Is(err, p.ErrLease):
		code = codes.Aborted
		reason = err.Error()
	case errors.Is(err, p.ErrCorrupt):
		code = codes.DataLoss
		reason = p.ErrCorrupt.Error()
	case errors.Is(err, p.ErrCancellation), errors.Is(err, p.ErrRetry), errors.Is(err, p.ErrStale):
		code = codes.FailedPrecondition
		reason = err.Error()
	default:
		if c := status.Code(err); c != codes.Unknown {
			return err
		}
	}
	return status.Error(code, reason)
}
func validTask(tenant, id string) bool { return p.Tenant(tenant) && p.Key(id) }
func (s *service) GetProvisioningTask(ctx context.Context, r *v1.ReadProvisioningTaskRequest) (*v1.ProvisioningTaskDTO, error) {
	if _, e := actor(ctx); e != nil {
		return nil, expose(e)
	}
	if r == nil || !validTask(r.TenantId, r.TaskId) {
		return nil, expose(p.ErrInvalid)
	}
	t, e := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ProvisioningRepositories]) (*p.Task, error) {
		return sc.Repositories().Tasks.Get(sc.Context(), r.TenantId, r.TaskId)
	})
	if e != nil {
		return nil, expose(e)
	}
	return taskDTO(*t), nil
}
func page(after string, n uint32) (uint32, error) {
	if after != "" && !p.Key(after) {
		return 0, p.ErrInvalid
	}
	if n == 0 {
		n = 50
	}
	if n > 100 {
		return 0, p.ErrInvalid
	}
	return n, nil
}
func (s *service) ListProvisioningTasks(ctx context.Context, r *v1.ListProvisioningTasksRequest) (*v1.ListProvisioningTasksResponse, error) {
	if _, e := actor(ctx); e != nil {
		return nil, expose(e)
	}
	if r == nil || !p.Tenant(r.TenantId) {
		return nil, expose(p.ErrInvalid)
	}
	n, e := page(r.AfterTaskId, r.Limit)
	if e != nil {
		return nil, expose(e)
	}
	tasks, e := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ProvisioningRepositories]) ([]p.Task, error) {
		return sc.Repositories().Tasks.List(sc.Context(), r.TenantId, r.AfterTaskId, n)
	})
	if e != nil {
		return nil, expose(e)
	}
	out := &v1.ListProvisioningTasksResponse{}
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, taskDTO(t))
	}
	if len(tasks) == int(n) {
		out.NextAfterTaskId = tasks[len(tasks)-1].ID
	}
	return out, nil
}
func (s *service) RetryProvisioningTask(ctx context.Context, r *v1.MutateProvisioningTaskRequest) (*v1.ProvisioningTaskDTO, error) {
	return s.mutate(ctx, r, "RETRY")
}
func (s *service) CancelProvisioningTask(ctx context.Context, r *v1.MutateProvisioningTaskRequest) (*v1.ProvisioningTaskDTO, error) {
	return s.mutate(ctx, r, "CANCEL")
}
func (s *service) mutate(ctx context.Context, r *v1.MutateProvisioningTaskRequest, operation string) (*v1.ProvisioningTaskDTO, error) {
	a, e := actor(ctx)
	if e != nil {
		return nil, expose(e)
	}
	if r == nil || !validTask(r.TenantId, r.TaskId) || r.ExpectedRevision == 0 || !p.Key(r.RequestId) || execution.IdempotencyKeyFrom(ctx) != r.RequestId || !p.Reason(r.Reason) {
		return nil, expose(p.ErrInvalid)
	}
	reason := strings.TrimSpace(r.Reason)
	fp := p.Digest([]any{a, operation, r.TenantId, r.TaskId, r.ExpectedRevision, r.RequestId, reason})
	result, e := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ProvisioningRepositories]) (p.Task, error) {
		repo, call := sc.Repositories().Tasks, sc.Context()
		t, e := repo.LockTask(call, r.TenantId, r.TaskId)
		if e != nil {
			return p.Task{}, e
		}
		replay, e := repo.ActionReceipt(call, a, operation, r.RequestId, fp)
		if e != nil {
			return p.Task{}, e
		}
		if replay != nil {
			return *replay, nil
		}
		if t.Revision != r.ExpectedRevision {
			return p.Task{}, p.ErrConflict
		}
		before := t.Clone()
		now, e := repo.Now(call)
		if e != nil {
			return p.Task{}, e
		}
		if operation == "RETRY" {
			e = t.Retry(now)
		} else {
			if !t.Cancellable() {
				return p.Task{}, p.ErrCancellation
			}
			_, e = s.capabilities.CommercialSubscriptionChanges().CancelPreparedSubscriptionChange(call, &v1.PreparedSubscriptionChangeRequest{TenantId: r.TenantId, TaskId: r.TaskId, TaskRevision: t.Revision})
			if e != nil {
				return p.Task{}, e
			}
			e = t.Cancel(now)
		}
		if e != nil {
			return p.Task{}, e
		}
		if e = repo.Save(call, before, *t, a, reason); e != nil {
			return p.Task{}, e
		}
		if e = repo.StoreActionReceipt(call, a, operation, r.RequestId, fp, *t); e != nil {
			return p.Task{}, e
		}
		return *t, nil
	})
	if e != nil {
		return nil, expose(e)
	}
	return taskDTO(result), nil
}
func (s *service) ClaimProvisioningWork(ctx context.Context, r *v1.ClaimProvisioningWorkRequest) (*v1.ProvisioningWorkDTO, error) {
	a, e := actor(ctx)
	if e != nil {
		return nil, expose(e)
	}
	if r == nil || !p.Key(r.WorkerId) || r.LeaseSeconds < 5 || r.LeaseSeconds > 300 {
		return nil, expose(p.ErrInvalid)
	}
	t, e := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ProvisioningRepositories]) (*p.Task, error) {
		repo, call := sc.Repositories().Tasks, sc.Context()
		now, e := repo.Now(call)
		if e != nil {
			return nil, e
		}
		due, e := repo.Due(call, now, 1)
		if e != nil || len(due) == 0 {
			return nil, e
		}
		t, e := repo.LockTask(call, due[0].TenantID, due[0].ID)
		if e != nil {
			return nil, e
		}
		now, e = repo.Now(call)
		if e != nil {
			return nil, e
		}
		if !t.Due(now) {
			return nil, nil
		}
		before := t.Clone()
		if e = t.Claim(r.WorkerId, now, time.Duration(r.LeaseSeconds)*time.Second); e != nil {
			return nil, e
		}
		if e = repo.Save(call, before, *t, a, "worker lease claim"); e != nil {
			return nil, e
		}
		if t.State != p.Running {
			return nil, nil
		}
		return t, nil
	})
	if e != nil {
		return nil, expose(e)
	}
	if t == nil {
		return &v1.ProvisioningWorkDTO{}, nil
	}
	out := &v1.ProvisioningWorkDTO{Found: true, Task: taskDTO(*t), WorkerId: r.WorkerId, LeaseToken: t.LeaseToken}
	if t.StepIndex < len(t.Steps) {
		out.IdempotencyKey = t.Steps[t.StepIndex].IdempotencyKey
	}
	return out, nil
}
func (s *service) RecordProvisioningWork(ctx context.Context, r *v1.RecordProvisioningWorkRequest) (*v1.ProvisioningTaskDTO, error) {
	a, e := actor(ctx)
	if e != nil {
		return nil, expose(e)
	}
	if r == nil || !validTask(r.TenantId, r.TaskId) || !p.Key(r.WorkerId) || r.LeaseToken == 0 {
		return nil, expose(p.ErrInvalid)
	}
	result, e := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ProvisioningRepositories]) (p.Task, error) {
		repo, call := sc.Repositories().Tasks, sc.Context()
		t, e := repo.LockTask(call, r.TenantId, r.TaskId)
		if e != nil {
			return p.Task{}, e
		}
		// Lost acknowledgement cannot turn an applied task back into failure.
		if t.State == p.Applied && t.LeaseToken == r.LeaseToken {
			return *t, nil
		}
		before := t.Clone()
		now, e := repo.Now(call)
		if e != nil {
			return p.Task{}, e
		}
		if e = t.Observe(r.WorkerId, r.LeaseToken, p.Observation{Outcome: r.Outcome, Evidence: r.Evidence, FailureCode: r.FailureCode}, now); e != nil {
			return p.Task{}, e
		}
		if e = repo.Save(call, before, *t, a, "worker outcome recorded"); e != nil {
			return p.Task{}, e
		}
		return *t, nil
	})
	if e != nil {
		return nil, expose(e)
	}
	return taskDTO(result), nil
}
func (s *service) FinalizeProvisioningWork(ctx context.Context, r *v1.FinalizeProvisioningWorkRequest) (*v1.ProvisioningTaskDTO, error) {
	a, e := actor(ctx)
	if e != nil {
		return nil, expose(e)
	}
	if r == nil || !validTask(r.TenantId, r.TaskId) || !p.Key(r.WorkerId) || r.LeaseToken == 0 {
		return nil, expose(p.ErrInvalid)
	}
	result, e := requestscope.JoinValue(ctx, s.repositories, func(sc *requestscope.View[ports.ProvisioningRepositories]) (p.Task, error) {
		repo, call := sc.Repositories().Tasks, sc.Context()
		t, e := repo.LockTask(call, r.TenantId, r.TaskId)
		if e != nil {
			return p.Task{}, e
		}
		if t.State == p.Applied && t.LeaseToken == r.LeaseToken {
			return *t, nil
		}
		before := t.Clone()
		now, e := repo.Now(call)
		if e != nil {
			return p.Task{}, e
		}
		if !t.Owns(r.WorkerId, r.LeaseToken, now) || t.Stage != p.ActivateStage {
			return p.Task{}, p.ErrLease
		}
		done, e := s.capabilities.CommercialSubscriptionChanges().CompletePreparedSubscriptionChange(call, &v1.PreparedSubscriptionChangeRequest{TenantId: r.TenantId, TaskId: r.TaskId, WorkerId: r.WorkerId, LeaseToken: r.LeaseToken})
		if e != nil {
			return p.Task{}, e
		}
		if done == nil {
			return p.Task{}, p.ErrCorrupt
		}
		at, e := time.Parse(time.RFC3339Nano, done.AppliedAt)
		if e != nil {
			return p.Task{}, p.ErrCorrupt
		}
		now, e = repo.Now(call)
		if e != nil {
			return p.Task{}, e
		}
		if e = t.Complete(r.WorkerId, r.LeaseToken, p.Completion{ChangeID: done.ChangeId, SubscriptionRevision: done.SubscriptionRevision, SourceVersion: done.SourceVersion, EntitlementVersion: done.EntitlementVersion, AppliedAt: at}, now); e != nil {
			return p.Task{}, e
		}
		if e = repo.Save(call, before, *t, a, "authoritative activation and completion receipt"); e != nil {
			return p.Task{}, e
		}
		return *t, nil
	})
	if e != nil {
		return nil, expose(e)
	}
	return taskDTO(result), nil
}
