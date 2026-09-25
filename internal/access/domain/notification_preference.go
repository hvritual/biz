package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
	"unicode"
)

// This default comes from Issue #183 / FR-99..102. It is not a security-message
// exemption or a conclusion about consent. Policy changes require #167 approval.
const NotificationPreferencePolicy = "optional-notifications-v1"

var (
	ErrNotificationPreferenceInvalid             = errors.New("access: invalid notification preference")
	ErrNotificationPreferenceConflict            = errors.New("access: notification preference version conflict")
	ErrNotificationPreferenceIdempotencyConflict = errors.New("access: notification preference idempotency conflict")
	ErrNotificationPreferenceUnavailable         = errors.New("access: notification preferences unavailable")
	ErrNotificationPreferenceForbidden           = errors.New("access: notification preference owner unavailable")
)

type NotificationPreferenceChannel string

const (
	NotificationPreferenceSMS   NotificationPreferenceChannel = "sms"
	NotificationPreferenceEmail NotificationPreferenceChannel = "email"
)

func (channel NotificationPreferenceChannel) Valid() bool {
	return channel == NotificationPreferenceSMS || channel == NotificationPreferenceEmail
}

type NotificationPreferenceState string

const (
	NotificationPreferenceDefault NotificationPreferenceState = "default"
	NotificationPreferenceAllow   NotificationPreferenceState = "allow"
	NotificationPreferenceDeny    NotificationPreferenceState = "deny"
)

// NotificationPreferenceOwner is supplied by a trusted session (self-service)
// or a trusted recipient-resolution port (notification routing), never a body ID.
type NotificationPreferenceOwner struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
}

func (owner NotificationPreferenceOwner) Validate() error {
	for _, value := range []string{owner.TenantID, owner.UserID} {
		if value == "" || len(value) > 64 || strings.TrimSpace(value) != value ||
			strings.IndexFunc(value, unicode.IsControl) >= 0 {
			return ErrNotificationPreferenceInvalid
		}
	}
	return nil
}

type NotificationPreference struct {
	Channel   NotificationPreferenceChannel `json:"channel"`
	State     NotificationPreferenceState   `json:"state"`
	Version   uint64                        `json:"version"`
	UpdatedAt *time.Time                    `json:"updated_at"`
}

// Validate distinguishes a missing row from an explicit choice. An invalid
// persisted value must never be interpreted as a missing row/default allow.
func (preference NotificationPreference) Validate() error {
	if !preference.Channel.Valid() {
		return ErrNotificationPreferenceInvalid
	}
	switch preference.State {
	case NotificationPreferenceDefault:
		if preference.Version != 0 || preference.UpdatedAt != nil {
			return ErrNotificationPreferenceInvalid
		}
	case NotificationPreferenceAllow, NotificationPreferenceDeny:
		if preference.Version == 0 || preference.UpdatedAt == nil || preference.UpdatedAt.IsZero() {
			return ErrNotificationPreferenceInvalid
		}
	default:
		return ErrNotificationPreferenceInvalid
	}
	return nil
}

func (preference NotificationPreference) OptionalAllowed() (bool, error) {
	if err := preference.Validate(); err != nil {
		return false, err
	}
	return preference.State != NotificationPreferenceDeny, nil
}

// MarshalJSON carries the server-resolved effective value. A browser does not
// need to implement the default policy or infer allow from a missing row.
func (preference NotificationPreference) MarshalJSON() ([]byte, error) {
	allowed, err := preference.OptionalAllowed()
	if err != nil {
		return nil, err
	}
	type data NotificationPreference
	return json.Marshal(struct {
		data
		Allowed bool `json:"allowed"`
	}{data: data(preference), Allowed: allowed})
}

type NotificationPreferenceSet struct {
	NotificationPreferenceOwner
	Policy string                 `json:"policy"`
	SMS    NotificationPreference `json:"sms"`
	Email  NotificationPreference `json:"email"`
}

func DefaultNotificationPreferences(owner NotificationPreferenceOwner) (NotificationPreferenceSet, error) {
	if err := owner.Validate(); err != nil {
		return NotificationPreferenceSet{}, err
	}
	return NotificationPreferenceSet{
		NotificationPreferenceOwner: owner,
		Policy:                      NotificationPreferencePolicy,
		SMS:                         NotificationPreference{Channel: NotificationPreferenceSMS, State: NotificationPreferenceDefault},
		Email:                       NotificationPreference{Channel: NotificationPreferenceEmail, State: NotificationPreferenceDefault},
	}, nil
}

// No reset-to-default mutation is exposed. A rollback must retain explicit deny.
type NotificationPreferenceChange struct {
	Channel         NotificationPreferenceChannel
	Allowed         bool
	ExpectedVersion uint64
	IdempotencyKey  string
}

func (change NotificationPreferenceChange) Validate() error {
	if !change.Channel.Valid() || change.IdempotencyKey == "" || len(change.IdempotencyKey) > 256 {
		return ErrNotificationPreferenceInvalid
	}
	// HTTP opaque keys are printable ASCII with no trimming/normalization.
	for _, c := range change.IdempotencyKey {
		if c < 33 || c > 126 {
			return ErrNotificationPreferenceInvalid
		}
	}
	return nil
}

// Fingerprints bind one key to one owner and one exact payload. A key reused
// for another channel is a conflict; SMS/email do not share a preference version.
func (change NotificationPreferenceChange) Fingerprints(owner NotificationPreferenceOwner) (string, string, error) {
	if err := owner.Validate(); err != nil {
		return "", "", err
	}
	if err := change.Validate(); err != nil {
		return "", "", err
	}
	key, err := json.Marshal([]string{"notification-preference.update", owner.TenantID, owner.UserID, change.IdempotencyKey})
	if err != nil {
		return "", "", err
	}
	payload, err := json.Marshal(struct {
		Policy          string
		Channel         NotificationPreferenceChannel
		Allowed         bool
		ExpectedVersion uint64
	}{NotificationPreferencePolicy, change.Channel, change.Allowed, change.ExpectedVersion})
	if err != nil {
		return "", "", err
	}
	keyHash, payloadHash := sha256.Sum256(key), sha256.Sum256(payload)
	return hex.EncodeToString(keyHash[:]), hex.EncodeToString(payloadHash[:]), nil
}

// Apply checks CAS for both first writes (version 0) and existing rows. Even an
// explicit allow of the source default is a persisted choice, not a no-op read.
func (change NotificationPreferenceChange) Apply(current NotificationPreference, now time.Time) (NotificationPreference, error) {
	if err := change.Validate(); err != nil {
		return NotificationPreference{}, err
	}
	if err := current.Validate(); err != nil {
		return NotificationPreference{}, err
	}
	if change.Channel != current.Channel || now.IsZero() {
		return NotificationPreference{}, ErrNotificationPreferenceInvalid
	}
	if current.Version != change.ExpectedVersion || current.Version == math.MaxUint64 {
		return NotificationPreference{}, ErrNotificationPreferenceConflict
	}
	state := NotificationPreferenceDeny
	if change.Allowed {
		state = NotificationPreferenceAllow
	}
	updated := now.UTC().Truncate(time.Microsecond)
	return NotificationPreference{Channel: change.Channel, State: state, Version: current.Version + 1, UpdatedAt: &updated}, nil
}

type NotificationPreferenceReceipt struct {
	NotificationPreferenceOwner
	Policy     string                 `json:"policy"`
	ReceiptID  string                 `json:"receipt_id"`
	Preference NotificationPreference `json:"preference"`
}
