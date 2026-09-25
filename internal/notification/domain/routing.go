package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
)

var (
	ErrRoutingInvalid     = errors.New("notification: invalid routing event")
	ErrRoutingConflict    = errors.New("notification: routing event conflict")
	ErrRoutingLease       = errors.New("notification: routing lease invalid")
	ErrRoutingUnavailable = errors.New("notification: routing unavailable")
)

const (
	RoutingStatePending = "PENDING"
	RoutingStateLeased  = "LEASED"
	RoutingStateRouted  = "ROUTED"
ExternalTaskStatePending = "PENDING"

	RouteOutcomeInAppCreated       = "IN_APP_CREATED"
	RouteOutcomeExternalTask       = "EXTERNAL_TASK_CREATED"
	RouteOutcomePreferenceDenied   = "PREFERENCE_DENIED"
	RouteOutcomeChannelUnavailable = "CHANNEL_UNAVAILABLE"
	RouteOutcomeRecipientInactive  = "RECIPIENT_INACTIVE"
	RouteOutcomeNoConfiguration    = "NO_CONFIGURATION"
	RouteOutcomeTypeUnavailable    = "TYPE_UNAVAILABLE"
	RouteOutcomeGroupUnavailable   = "GROUP_UNAVAILABLE"
)

type BusinessEvent struct {
	EventID       string       `json:"event_id"`
	TenantID      string       `json:"tenant_id"`
	GroupID       string       `json:"group_id"`
	TypeCode      string       `json:"type_code"`
	Level         MessageLevel `json:"level"`
	TraceID       string       `json:"trace_id"`
	ReferenceKind string       `json:"reference_kind"`
	ReferenceID   string       `json:"reference_id"`
	OccurredAt    time.Time    `json:"occurred_at"`
}

func (event BusinessEvent) Validate() error {
	if !ValidConfigurationID(event.EventID, 160) || !ValidConfigurationID(event.TenantID, 64) ||
		!ValidConfigurationID(event.GroupID, 64) || !validCode(event.TypeCode) || !event.Level.Valid() ||
		!ValidConfigurationID(event.TraceID, 160) || !ValidConfigurationID(event.ReferenceKind, 64) ||
		!ValidConfigurationID(event.ReferenceID, 160) || event.OccurredAt.IsZero() {
		return ErrRoutingInvalid
	}
	return nil
}

func (event BusinessEvent) Digest() (string, error) {
	if err := event.Validate(); err != nil {
		return "", err
	}
	normalized := event
	normalized.OccurredAt = time.UnixMicro(event.OccurredAt.UTC().UnixMicro()).UTC()
	data, err := json.Marshal(normalized)
	if err != nil { return "", err }
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

type RoutingClaim struct {
	Event      BusinessEvent
	WorkerID   string
	LeaseToken uint64
	LeaseUntil time.Time
	Attempt    uint32
}

func (claim RoutingClaim) Validate() error {
	if err := claim.Event.Validate(); err != nil { return err }
	if !ValidConfigurationID(claim.WorkerID, 96) || claim.LeaseToken == 0 || claim.LeaseUntil.IsZero() || claim.Attempt == 0 {
		return ErrRoutingLease
	}
	return nil
}

type RouteDecision struct {
	ConfigurationID      string
	ConfigurationVersion uint64
	UserID               string
	Channel              string
	Outcome              string
}

func (decision RouteDecision) Validate() error {
	if decision.Outcome == RouteOutcomeNoConfiguration || decision.Outcome == RouteOutcomeTypeUnavailable || decision.Outcome == RouteOutcomeGroupUnavailable {
		if decision.UserID != "" || decision.Channel != "" { return ErrRoutingInvalid }
		return nil
	}
	if !ValidConfigurationID(decision.ConfigurationID, 64) || decision.ConfigurationVersion == 0 ||
		!ValidConfigurationID(decision.UserID, 64) || !validCode(decision.Channel) {
		return ErrRoutingInvalid
	}
	switch decision.Outcome {
	case RouteOutcomeInAppCreated, RouteOutcomeExternalTask, RouteOutcomePreferenceDenied,
		RouteOutcomeChannelUnavailable, RouteOutcomeRecipientInactive:
		return nil
	default:
		return ErrRoutingInvalid
	}
}

type RoutePlan struct{ Decisions []RouteDecision }

func (plan RoutePlan) Canonical() (RoutePlan, error) {
	if len(plan.Decisions) == 0 || len(plan.Decisions) > 1000 { return RoutePlan{}, ErrRoutingInvalid }
	out := RoutePlan{Decisions: append([]RouteDecision{}, plan.Decisions...)}
	seen := map[string]bool{}
	for _, d := range out.Decisions {
		if err := d.Validate(); err != nil { return RoutePlan{}, err }
		key := d.UserID + "\x00" + d.Channel + "\x00" + d.Outcome
		if seen[key] { return RoutePlan{}, ErrRoutingInvalid }
		seen[key] = true
	}
	sort.Slice(out.Decisions, func(i, j int) bool {
		a,b:=out.Decisions[i],out.Decisions[j]
		if a.UserID!=b.UserID{return a.UserID<b.UserID}
		if a.Channel!=b.Channel{return a.Channel<b.Channel}
		return a.Outcome<b.Outcome
	})
	return out,nil
}

type RoutingResult struct {
	EventID         string
	State           string
	InAppCreated    uint64
	ExternalTasks   uint64
	Denied          uint64
	Unavailable     uint64
	Inactive        uint64
	NoConfiguration uint64
	TypeUnavailable uint64
	RoutedAt        time.Time
}

func StableRoutingID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}
