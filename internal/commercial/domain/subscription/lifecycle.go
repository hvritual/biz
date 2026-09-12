package subscription

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// LifecyclePolicy is trusted process configuration. It deliberately identifies
// trial plans by immutable code/version rather than inferring a trial from an
// opaque price reference. Grace is optional; a zero duration moves directly to
// RESTRICTED at a fixed-period boundary.
type LifecyclePolicy struct {
	TrialPlans       []PlanReference
	GraceDuration    time.Duration
	BusinessTimezone string
}

type PlanReference struct {
	PlanCode string
	Version  uint64
}

func (p LifecyclePolicy) Validate() error {
	if p.GraceDuration < 0 || p.GraceDuration > 365*24*time.Hour || p.GraceDuration%time.Microsecond != 0 {
		return errors.New("subscription: invalid lifecycle grace duration")
	}
	if p.BusinessTimezone != "" {
		if strings.TrimSpace(p.BusinessTimezone) != p.BusinessTimezone || len(p.BusinessTimezone) > 64 {
			return errors.New("subscription: invalid lifecycle business timezone")
		}
		if _, err := time.LoadLocation(p.BusinessTimezone); err != nil {
			return errors.New("subscription: invalid lifecycle business timezone")
		}
	}
	seen := make(map[PlanReference]struct{}, len(p.TrialPlans))
	for _, plan := range p.TrialPlans {
		if !ValidCode(plan.PlanCode) || plan.Version == 0 {
			return errors.New("subscription: invalid lifecycle trial plan")
		}
		if _, exists := seen[plan]; exists {
			return errors.New("subscription: duplicate lifecycle trial plan")
		}
		seen[plan] = struct{}{}
	}
	return nil
}

func (p LifecyclePolicy) StateFor(planCode string, version uint64) string {
	for _, trial := range p.TrialPlans {
		if trial.PlanCode == planCode && trial.Version == version {
			return StateTrial
		}
	}
	return StateActive
}

func (p LifecyclePolicy) Canonical() LifecyclePolicy {
	out := LifecyclePolicy{GraceDuration: p.GraceDuration, BusinessTimezone: p.BusinessTimezone, TrialPlans: append([]PlanReference(nil), p.TrialPlans...)}
	sort.Slice(out.TrialPlans, func(i, j int) bool {
		if out.TrialPlans[i].PlanCode != out.TrialPlans[j].PlanCode {
			return out.TrialPlans[i].PlanCode < out.TrialPlans[j].PlanCode
		}
		return out.TrialPlans[i].Version < out.TrialPlans[j].Version
	})
	return out
}

func (p LifecyclePolicy) Timezone() string {
	if p.BusinessTimezone == "" {
		return "UTC"
	}
	return p.BusinessTimezone
}
