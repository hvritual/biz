// Package timetransition models durable commercial time boundaries.
// It contains no SQL, wall clock, transport or external side effects.
package timetransition

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	ScheduledChange      = "SCHEDULED_CHANGE"
	EntitlementExpiry    = "ENTITLEMENT_EXPIRY"
	SubscriptionBoundary = "SUBSCRIPTION_BOUNDARY"

	Queued            = "QUEUED"
	Running           = "RUNNING"
	Applied           = "APPLIED"
	Superseded        = "SUPERSEDED"
	ReconcileRequired = "RECONCILIATION_REQUIRED"
)

var (
	ErrInvalid  = errors.New("TIME_TRANSITION_INVALID_REQUEST")
	ErrNotFound = errors.New("TIME_TRANSITION_NOT_FOUND")
	ErrConflict = errors.New("TIME_TRANSITION_VERSION_CONFLICT")
	ErrLease    = errors.New("TIME_TRANSITION_LEASE_LOST")
	ErrCorrupt  = errors.New("TIME_TRANSITION_AUTHORITY_CORRUPT")
)

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

func Key(v string) bool    { return keyPattern.MatchString(v) }
func Tenant(v string) bool { return len(v) <= 64 && Key(v) }
func Digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func CanonicalTime(v time.Time) time.Time { return v.UTC().Truncate(time.Microsecond) }
func ID(kind, tenant, authority string, version uint64, due time.Time) string {
	return "time-" + Digest([]any{kind, tenant, authority, version, CanonicalTime(due)})[:48]
}

// Task stores the authority instant in UTC and records the business timezone
// used to display that instant. Current production sources have no trusted
// tenant timezone authority, so callers use UTC until such an authority exists.
type Task struct {
	ID               string     `json:"transition_id"`
	Kind             string     `json:"kind"`
	TenantID         string     `json:"tenant_id"`
	AuthorityID      string     `json:"authority_id"`
	AuthorityVersion uint64     `json:"authority_version"`
	DueAt            time.Time  `json:"due_at"`
	BusinessTimezone string     `json:"business_timezone"`
	Revision         uint64     `json:"revision"`
	State            string     `json:"state"`
	LeaseOwner       string     `json:"lease_owner,omitempty"`
	LeaseToken       uint64     `json:"lease_token"`
	LeaseUntil       *time.Time `json:"lease_until,omitempty"`
	Outcome          string     `json:"outcome,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Hash             string     `json:"hash"`
}

func New(kind, tenant, authority string, version uint64, due, now time.Time) (Task, error) {
	return NewInTimezone(kind, tenant, authority, version, due, now, "UTC")
}

func NewInTimezone(kind, tenant, authority string, version uint64, due, now time.Time, businessTimezone string) (Task, error) {
	due, now = CanonicalTime(due), CanonicalTime(now)
	if strings.TrimSpace(businessTimezone) != businessTimezone || businessTimezone == "" || len(businessTimezone) > 64 {
		return Task{}, ErrInvalid
	}
	if _, err := time.LoadLocation(businessTimezone); err != nil {
		return Task{}, ErrInvalid
	}
	t := Task{Kind: kind, TenantID: tenant, AuthorityID: authority, AuthorityVersion: version, DueAt: due, BusinessTimezone: businessTimezone, Revision: 1, State: Queued, CreatedAt: now, UpdatedAt: now}
	t.ID = ID(kind, tenant, authority, version, due)
	t = t.Seal()
	if err := t.Integrity(); err != nil {
		return t, err
	}
	return t, nil
}

func (t Task) Seal() Task { t.Hash = ""; t.Hash = Digest(t); return t }
func (t Task) Integrity() error {
	if !Key(t.ID) || t.ID != ID(t.Kind, t.TenantID, t.AuthorityID, t.AuthorityVersion, t.DueAt) || !Tenant(t.TenantID) || !Key(t.AuthorityID) || t.AuthorityVersion == 0 || t.Revision == 0 || t.DueAt.IsZero() || t.DueAt.Location() != time.UTC || t.CreatedAt.IsZero() || t.UpdatedAt.Before(t.CreatedAt) || t.Hash != t.Seal().Hash || t.BusinessTimezone == "" || len(t.BusinessTimezone) > 64 {
		return ErrCorrupt
	}
	if _, err := time.LoadLocation(t.BusinessTimezone); err != nil {
		return ErrCorrupt
	}
	switch t.Kind {
	case ScheduledChange, EntitlementExpiry, SubscriptionBoundary:
	default:
		return ErrCorrupt
	}
	switch t.State {
	case Queued, Running, Applied, Superseded, ReconcileRequired:
	default:
		return ErrCorrupt
	}
	if t.State == Running {
		if !Key(t.LeaseOwner) || t.LeaseToken == 0 || t.LeaseUntil == nil || !t.LeaseUntil.After(t.UpdatedAt) {
			return ErrCorrupt
		}
	} else if t.LeaseOwner != "" || t.LeaseUntil != nil {
		return ErrCorrupt
	}
	if (t.State == Applied || t.State == Superseded || t.State == ReconcileRequired) != (strings.TrimSpace(t.Outcome) != "") {
		return ErrCorrupt
	}
	return nil
}
func (t Task) Terminal() bool { return t.State == Applied || t.State == Superseded || t.State == ReconcileRequired }
func (t Task) Due(now time.Time) bool {
	now = CanonicalTime(now)
	if t.State == Running {
		return t.LeaseUntil != nil && !now.Before(*t.LeaseUntil)
	}
	return t.State == Queued && !now.Before(t.DueAt)
}
func (t Task) Owns(owner string, token uint64, now time.Time) bool {
	now = CanonicalTime(now)
	return t.State == Running && t.LeaseOwner == owner && t.LeaseToken == token && t.LeaseUntil != nil && now.Before(*t.LeaseUntil)
}
func (t *Task) Claim(owner string, now time.Time, lease time.Duration) error {
	now = CanonicalTime(now)
	if !Key(owner) || lease < 5*time.Second || lease > 5*time.Minute || !t.Due(now) || t.Revision == ^uint64(0) || t.LeaseToken == ^uint64(0) {
		return ErrConflict
	}
	t.State = Running
	t.LeaseOwner = owner
	t.LeaseToken++
	until := CanonicalTime(now.Add(lease))
	t.LeaseUntil = &until
	t.Revision++
	t.UpdatedAt = now
	*t = t.Seal()
	return t.Integrity()
}
func (t *Task) Finish(owner string, token uint64, state, outcome string, now time.Time) error {
	now = CanonicalTime(now)
	if !t.Owns(owner, token, now) || strings.TrimSpace(outcome) == "" || len(outcome) > 160 || t.Revision == ^uint64(0) {
		return ErrLease
	}
	switch state {
	case Applied, Superseded, ReconcileRequired:
	default:
		return ErrInvalid
	}
	t.State = state
	t.Outcome = strings.TrimSpace(outcome)
	t.LeaseOwner = ""
	t.LeaseUntil = nil
	t.Revision++
	t.UpdatedAt = now
	*t = t.Seal()
	return t.Integrity()
}
