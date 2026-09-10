package bizruntime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	v1 "github.com/hvritual/biz/contracts/gen/commercial/v1"
	app "github.com/hvritual/biz/internal/commercial/application"
	p "github.com/hvritual/biz/internal/commercial/domain/provisioning"
	policy "github.com/hvritual/biz/internal/commercial/policy"
	ports "github.com/hvritual/biz/internal/commercial/ports"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log/slog"
	"sync"
	"time"
	"yunka.io/framework/core"
	"yunka.io/framework/core/identity"
	"yunka.io/framework/operation"
)

// Worker credentials are existing persisted platform credentials. There is no
// special principal, bypass, or second Executor. An embedding process may use
// Automatic=false and invoke RunProvisioningOnce through the same checks.
type ProvisioningWorkerOptions struct {
	Token         string
	Automatic     bool
	PollInterval  time.Duration
	LeaseDuration time.Duration
	StepTimeout   time.Duration
}

func (o ProvisioningWorkerOptions) normalized() ProvisioningWorkerOptions {
	if o.PollInterval == 0 {
		o.PollInterval = time.Second
	}
	if o.LeaseDuration == 0 {
		o.LeaseDuration = 30 * time.Second
	}
	if o.StepTimeout == 0 {
		o.StepTimeout = 10 * time.Second
	}
	return o
}
func (o ProvisioningWorkerOptions) Validate() error {
	o = o.normalized()
	if o.Automatic && o.Token == "" {
		return errors.New("provisioning: automatic worker requires a persisted platform credential")
	}
	if o.PollInterval < 10*time.Millisecond || o.PollInterval > time.Hour || o.LeaseDuration < 5*time.Second || o.LeaseDuration > 5*time.Minute || o.StepTimeout < time.Millisecond || o.StepTimeout+time.Second >= o.LeaseDuration || o.LeaseDuration%time.Second != 0 {
		return errors.New("provisioning: invalid bounded worker timing")
	}
	return nil
}

type ProvisioningTick struct {
	DeliveryID string
	TaskID     string
	TaskState  string
}
type provisioningRunner struct {
	options       ProvisioningWorkerOptions
	id            string
	application   app.ProvisioningApplication
	executor      operation.Executor
	authenticator *runtimeAuthenticator
	adapters      map[string]ports.PreparationAdapter
	runMu         sync.Mutex
	mu            sync.RWMutex
	cancel        context.CancelFunc
	done          chan struct{}
	started       bool
	lastError     error
}

func newProvisioningRunner(options Options) (*provisioningRunner, error) {
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return nil, e
	}
	r := &provisioningRunner{options: options.ProvisioningWorker.normalized(), id: "provisioner-" + hex.EncodeToString(nonce[:]), adapters: map[string]ports.PreparationAdapter{}}
	for _, a := range options.PreparationAdapters {
		if !p.Key(a.AdapterID) || !p.Key(a.Version) || a.Adapter == nil {
			return nil, errors.New("provisioning: a named, versioned compiled adapter is required")
		}
		key := a.AdapterID + "/" + a.Version
		if _, ok := r.adapters[key]; ok {
			return nil, errors.New("provisioning: duplicate adapter version")
		}
		r.adapters[key] = a.Adapter
	}
	return r, nil
}
func (r *provisioningRunner) component() core.RuntimeComponent {
	return core.RuntimeComponent{Name: "commercial-provisioning-worker", StartFunc: r.start, HealthFunc: r.health, ShutdownFunc: r.shutdown}
}
func (r *provisioningRunner) start(ctx context.Context) error {
	if r.application == nil || r.executor == nil || r.authenticator == nil {
		return errors.New("provisioning: worker binding incomplete")
	}
	if _, err := r.callContext(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return nil
	}
	r.started = true
	r.done = make(chan struct{})
	loop, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.mu.Unlock()
	if !r.options.Automatic {
		close(r.done)
		return nil
	}
	go func() {
		defer close(r.done)
		timer := time.NewTicker(r.options.PollInterval)
		defer timer.Stop()
		for {
			select {
			case <-loop.Done():
				return
			default:
			}
			_, err := r.tick(loop)
			r.mu.Lock()
			r.lastError = err
			r.mu.Unlock()
			if err != nil && loop.Err() == nil {
				slog.Warn("commercial provisioning poll failed", "worker_id", r.id, "code", status.Code(err).String())
			}
			select {
			case <-loop.Done():
				return
			case <-timer.C:
			}
		}
	}()
	return nil
}
func (r *provisioningRunner) health(context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.started {
		return errors.New("provisioning worker not started")
	}
	return r.lastError
}
func (r *provisioningRunner) shutdown(ctx context.Context) error {
	r.mu.RLock()
	cancel, done := r.cancel, r.done
	r.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (r *provisioningRunner) callContext(ctx context.Context) (context.Context, error) {
	if r.options.Token == "" || r.authenticator == nil {
		return nil, errors.New("provisioning worker is disabled")
	}
	principal, e := r.authenticator.authenticate(ctx, r.options.Token)
	if e != nil {
		return nil, e
	}
	if !principal.Authenticated || principal.TenantID != "" {
		return nil, p.ErrScope
	}
	return identity.WithPrincipal(ctx, principal), nil
}
func (s *Started) RunProvisioningOnce(ctx context.Context) (ProvisioningTick, error) {
	if s == nil || s.provisioningRunner == nil {
		return ProvisioningTick{}, errors.New("provisioning worker unavailable")
	}
	return s.provisioningRunner.tick(ctx)
}
func (r *provisioningRunner) tick(ctx context.Context) (ProvisioningTick, error) {
	r.runMu.Lock()
	defer r.runMu.Unlock()
	out := ProvisioningTick{}
	call, e := r.callContext(ctx)
	if e != nil {
		return out, e
	}
	lease := uint32(r.options.LeaseDuration / time.Second)
	delivery, e := operation.ExecuteTyped(call, r.executor, policy.OperationPlanProvisioningClaimProvisioningDelivery(), &v1.ClaimProvisioningDeliveryRequest{WorkerId: r.id, LeaseSeconds: lease}, r.application.ClaimProvisioningDelivery)
	if e != nil {
		return out, e
	}
	if delivery != nil && delivery.Found {
		out.DeliveryID = delivery.Delivery.EventId
		command := &v1.CompleteProvisioningDeliveryRequest{EventId: out.DeliveryID, WorkerId: r.id, LeaseToken: delivery.LeaseToken}
		call, e = r.callContext(ctx)
		if e != nil {
			return out, e
		}
		_, e = operation.ExecuteTyped(call, r.executor, policy.OperationPlanProvisioningCompleteProvisioningDelivery(), command, r.application.CompleteProvisioningDelivery)
		if e != nil {
			// Inbox, monotonic projection and ack rolled back together. Record a fenced
			// failure in another ordinary root, preserving the original immutable event.
			command.FailureCode = "DELIVERY_TRANSACTION_FAILED"
			call, authErr := r.callContext(ctx)
			if authErr != nil {
				return out, authErr
			}
			_, recordErr := operation.ExecuteTyped(call, r.executor, policy.OperationPlanProvisioningFailProvisioningDelivery(), command, r.application.FailProvisioningDelivery)
			if recordErr != nil {
				return out, errors.Join(e, recordErr)
			}
		}
	}
	call, e = r.callContext(ctx)
	if e != nil {
		return out, e
	}
	work, e := operation.ExecuteTyped(call, r.executor, policy.OperationPlanProvisioningClaimProvisioningWork(), &v1.ClaimProvisioningWorkRequest{WorkerId: r.id, LeaseSeconds: lease}, r.application.ClaimProvisioningWork)
	if e != nil {
		return out, e
	}
	if work == nil || !work.Found {
		return out, nil
	}
	if work.Task == nil {
		return out, p.ErrCorrupt
	}
	task := work.Task
	out.TaskID = task.TaskId
	if task.Stage == p.ActivateStage {
		call, e = r.callContext(ctx)
		if e != nil {
			return out, e
		}
		result, applyErr := operation.ExecuteTyped(call, r.executor, policy.OperationPlanProvisioningFinalizeProvisioningWork(), &v1.FinalizeProvisioningWorkRequest{TenantId: task.TenantId, TaskId: task.TaskId, WorkerId: r.id, LeaseToken: work.LeaseToken}, r.application.FinalizeProvisioningWork)
		if applyErr == nil {
			out.TaskState = result.State
			return out, nil
		}
		outcome, code := p.Unknown, "ACTIVATION_REVALIDATION_FAILED"
		if status.Code(applyErr) == codes.Unavailable || status.Code(applyErr) == codes.DeadlineExceeded {
			outcome = p.Retryable
			code = "ACTIVATION_TRANSACTION_RETRY"
		}
		observation := p.Observation{Outcome: outcome, Evidence: "activation root failed; authoritative task state must be read before retry", FailureCode: code}
		result, e = r.record(ctx, work, observation)
		if e != nil {
			return out, errors.Join(applyErr, e)
		}
		out.TaskState = result.State
		return out, nil
	}
	if int(task.StepIndex) >= len(task.Steps) || task.Steps[task.StepIndex].Requirement == nil {
		return out, p.ErrCorrupt
	}
	step := task.Steps[task.StepIndex].Requirement
	request := ports.PreparationRequest{TaskID: task.TaskId, TenantID: task.TenantId, ChangeID: task.ChangeId, TargetPlanCode: task.TargetPlanCode, TargetPlanVersion: task.TargetPlanVersion, Step: p.Requirement{Code: step.Code, Adapter: step.Adapter, Version: step.Version, MaxAttempts: step.MaxAttempts}, IdempotencyKey: work.IdempotencyKey}
	adapter := r.adapters[step.Adapter+"/"+step.Version]
	// Claim has committed. Never pass the execution transaction to the adapter.
	external, cancel := context.WithTimeout(ctx, r.options.StepTimeout)
	observation := invokePreparation(external, adapter, request, task.Stage == p.ReconcileStage)
	cancel()
	result, e := r.record(ctx, work, observation)
	if e != nil {
		return out, e
	}
	out.TaskState = result.State
	return out, nil
}
func invokePreparation(ctx context.Context, a ports.PreparationAdapter, in ports.PreparationRequest, reconcile bool) (out p.Observation) {
	defer func() {
		if recover() != nil {
			out = p.Observation{Outcome: p.Unknown, Evidence: "adapter panicked after dispatch; reconcile the stable provider key", FailureCode: "ADAPTER_PANIC"}
		}
	}()
	if a == nil {
		if reconcile {
			return p.Observation{Outcome: p.Unknown, Evidence: "the exact adapter version is unavailable for reconciliation", FailureCode: "ADAPTER_UNAVAILABLE"}
		}
		return p.Observation{Outcome: p.Retryable, Evidence: "no registered adapter was invoked; dispatch did not occur", FailureCode: "ADAPTER_UNAVAILABLE"}
	}
	if reconcile {
		out = a.Reconcile(ctx, in)
	} else {
		out = a.Prepare(ctx, in)
	}
	if out.Validate() != nil {
		return p.Observation{Outcome: p.Unknown, Evidence: "adapter returned an invalid outcome; no success may be inferred", FailureCode: "ADAPTER_RESULT_INVALID"}
	}
	return out
}
func (r *provisioningRunner) record(ctx context.Context, w *v1.ProvisioningWorkDTO, o p.Observation) (*v1.ProvisioningTaskDTO, error) {
	call, e := r.callContext(ctx)
	if e != nil {
		return nil, e
	}
	return operation.ExecuteTyped(call, r.executor, policy.OperationPlanProvisioningRecordProvisioningWork(), &v1.RecordProvisioningWorkRequest{TenantId: w.Task.TenantId, TaskId: w.Task.TaskId, WorkerId: r.id, LeaseToken: w.LeaseToken, Outcome: o.Outcome, Evidence: o.Evidence, FailureCode: o.FailureCode}, r.application.RecordProvisioningWork)
}
