package domain

import (
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNotificationPreferenceDefaultIsChannelSpecificAndSourceDefined(t *testing.T) {
	set, err := DefaultNotificationPreferences(NotificationPreferenceOwner{TenantID: "tenant-a", UserID: "shared-user"})
	if err != nil {
		t.Fatal(err)
	}
	for _, preference := range []NotificationPreference{set.SMS, set.Email} {
		allowed, err := preference.OptionalAllowed()
		if err != nil || !allowed || preference.State != NotificationPreferenceDefault || preference.Version != 0 || preference.UpdatedAt != nil {
			t.Fatalf("missing preference: %+v allowed=%v err=%v", preference, allowed, err)
		}
	}
	if set.SMS.Channel == set.Email.Channel || set.Policy != NotificationPreferencePolicy {
		t.Fatal("missing channel/policy binding")
	}
}

func TestNotificationPreferenceExplicitChoiceNeverBecomesMissing(t *testing.T) {
	now := time.Date(2026, 9, 25, 0, 0, 0, 123456789, time.UTC)
	for _, channel := range []NotificationPreferenceChannel{NotificationPreferenceSMS, NotificationPreferenceEmail} {
		for _, allowed := range []bool{true, false} {
			t.Run(string(channel)+"/"+map[bool]string{true: "allow", false: "deny"}[allowed], func(t *testing.T) {
				change := NotificationPreferenceChange{Channel: channel, Allowed: allowed, IdempotencyKey: "choice-1"}
				current := NotificationPreference{Channel: channel, State: NotificationPreferenceDefault}
				next, err := change.Apply(current, now)
				if err != nil {
					t.Fatal(err)
				}
				got, err := next.OptionalAllowed()
				if err != nil || got != allowed || next.Version != 1 || next.State == NotificationPreferenceDefault {
					t.Fatalf("bad explicit state: %+v %v", next, err)
				}
				if next.UpdatedAt.Nanosecond()%1000 != 0 {
					t.Fatal("receipt is not MySQL microsecond stable")
				}
				if current.Version != 0 || current.UpdatedAt != nil || current.State != NotificationPreferenceDefault {
					t.Fatal("Apply mutated input")
				}
			})
		}
	}
}

func TestNotificationPreferenceCASRejectsStaleCrossChannelAndOverflow(t *testing.T) {
	now := time.Now().UTC()
	valid := NotificationPreference{Channel: NotificationPreferenceSMS, State: NotificationPreferenceDeny, Version: 5, UpdatedAt: &now}
	cases := []struct {
		name       string
		preference NotificationPreference
		change     NotificationPreferenceChange
		want       error
	}{
		{"stale", valid, NotificationPreferenceChange{Channel: NotificationPreferenceSMS, Allowed: true, ExpectedVersion: 4, IdempotencyKey: "key"}, ErrNotificationPreferenceConflict},
		{"future", valid, NotificationPreferenceChange{Channel: NotificationPreferenceSMS, Allowed: true, ExpectedVersion: 6, IdempotencyKey: "key"}, ErrNotificationPreferenceConflict},
		{"cross-channel", valid, NotificationPreferenceChange{Channel: NotificationPreferenceEmail, ExpectedVersion: 5, IdempotencyKey: "key"}, ErrNotificationPreferenceInvalid},
		{"overflow", NotificationPreference{Channel: NotificationPreferenceSMS, State: NotificationPreferenceDeny, Version: math.MaxUint64, UpdatedAt: &now}, NotificationPreferenceChange{Channel: NotificationPreferenceSMS, ExpectedVersion: math.MaxUint64, IdempotencyKey: "key"}, ErrNotificationPreferenceConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.preference
			_, err := tc.change.Apply(tc.preference, now)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if !reflect.DeepEqual(before, tc.preference) {
				t.Fatal("rejected mutation changed state")
			}
		})
	}
}

func TestNotificationPreferenceCorruptPersistedStateFailsClosed(t *testing.T) {
	now := time.Now().UTC()
	cases := []NotificationPreference{
		{},
		{Channel: NotificationPreferenceSMS, State: "important"},
		{Channel: "push", State: NotificationPreferenceDefault},
		{Channel: NotificationPreferenceSMS, State: NotificationPreferenceDefault, Version: 1},
		{Channel: NotificationPreferenceSMS, State: NotificationPreferenceDefault, UpdatedAt: &now},
		{Channel: NotificationPreferenceSMS, State: NotificationPreferenceAllow, Version: 1},
		{Channel: NotificationPreferenceSMS, State: NotificationPreferenceDeny, Version: 0, UpdatedAt: &now},
		{Channel: NotificationPreferenceSMS, State: NotificationPreferenceAllow, Version: 1, UpdatedAt: new(time.Time)},
	}
	for i, preference := range cases {
		allowed, err := preference.OptionalAllowed()
		if err == nil || allowed {
			t.Fatalf("corrupt case %d was allowed: %+v", i, preference)
		}
		if _, err := json.Marshal(preference); err == nil {
			t.Fatalf("corrupt case %d serialized as success", i)
		}
	}
}

func TestNotificationPreferenceJSONCarriesServerEffectiveValue(t *testing.T) {
	set, _ := DefaultNotificationPreferences(NotificationPreferenceOwner{TenantID: "a", UserID: "u"})
	next, err := (NotificationPreferenceChange{Channel: NotificationPreferenceSMS, Allowed: false, IdempotencyKey: "deny"}).Apply(set.SMS, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	set.SMS = next
	encoded, err := json.Marshal(set)
	if err != nil {
		t.Fatal(err)
	}
	var actual struct {
		TenantID string `json:"tenant_id"`
		Policy   string `json:"policy"`
		SMS      struct {
			Allowed *bool  `json:"allowed"`
			State   string `json:"state"`
			Version uint64 `json:"version"`
		} `json:"sms"`
		Email struct {
			Allowed *bool  `json:"allowed"`
			State   string `json:"state"`
		} `json:"email"`
	}
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if actual.SMS.Allowed == nil || *actual.SMS.Allowed || actual.Email.Allowed == nil || !*actual.Email.Allowed || actual.SMS.Version != 1 || actual.TenantID != "a" || actual.Policy == "" {
		t.Fatalf("missing server effective value: %s", encoded)
	}
}

func TestNotificationPreferenceFingerprintBindsOwnerKeyAndPayload(t *testing.T) {
	owner := NotificationPreferenceOwner{TenantID: "a", UserID: "u"}
	change := NotificationPreferenceChange{Channel: NotificationPreferenceSMS, Allowed: false, ExpectedVersion: 2, IdempotencyKey: "request-1"}
	key, payload, err := change.Fingerprints(owner)
	if err != nil || len(key) != 64 || len(payload) != 64 {
		t.Fatalf("fingerprints %s %s %v", key, payload, err)
	}
	keyAgain, payloadAgain, _ := change.Fingerprints(owner)
	if key != keyAgain || payload != payloadAgain {
		t.Fatal("retry is unstable")
	}
	for _, otherOwner := range []NotificationPreferenceOwner{{TenantID: "b", UserID: "u"}, {TenantID: "a", UserID: "v"}, {TenantID: "A", UserID: "u"}} {
		otherKey, _, _ := change.Fingerprints(otherOwner)
		if otherKey == key {
			t.Fatal("owner not bound to key")
		}
	}
	variants := []NotificationPreferenceChange{change, change, change, change}
	variants[0].Channel = NotificationPreferenceEmail
	variants[1].Allowed = true
	variants[2].ExpectedVersion++
	variants[3].IdempotencyKey = "request-2"
	for i, variant := range variants {
		otherKey, otherPayload, err := variant.Fingerprints(owner)
		if err != nil {
			t.Fatal(err)
		}
		if i < 3 && (otherKey != key || otherPayload == payload) {
			t.Fatal("same key conflicting payload not distinguishable")
		}
		if i == 3 && (otherKey == key || otherPayload != payload) {
			t.Fatal("opaque key mixed into payload")
		}
	}
}

func TestNotificationPreferenceRejectsInvalidOwnerAndKey(t *testing.T) {
	owners := []NotificationPreferenceOwner{{}, {TenantID: " a", UserID: "u"}, {TenantID: "a", UserID: ""}, {TenantID: "a\x00b", UserID: "u"}, {TenantID: strings.Repeat("a", 65), UserID: "u"}, {TenantID: "a", UserID: "u\n"}}
	for _, owner := range owners {
		if _, err := DefaultNotificationPreferences(owner); err == nil {
			t.Fatalf("accepted invalid owner %+v", owner)
		}
	}
	for _, key := range []string{"", " ", "leading ", "\x00", "a\nb", "中文", strings.Repeat("a", 257)} {
		change := NotificationPreferenceChange{Channel: NotificationPreferenceSMS, IdempotencyKey: key}
		if change.Validate() == nil {
			t.Fatalf("accepted invalid key %q", key)
		}
	}
	for _, key := range []string{"k", strings.Repeat("A", 256), "uuid-0001_az:99"} {
		if (NotificationPreferenceChange{Channel: NotificationPreferenceEmail, IdempotencyKey: key}).Validate() != nil {
			t.Fatalf("rejected key %q", key)
		}
	}
}

func TestNotificationPreferenceExactCASAdvancesOnlyItsChannel(t *testing.T) {
	set, _ := DefaultNotificationPreferences(NotificationPreferenceOwner{TenantID: "a", UserID: "u"})
	originalEmail := set.Email
	for version := uint64(0); version < 20; version++ {
		change := NotificationPreferenceChange{Channel: NotificationPreferenceSMS, Allowed: version%2 == 0, ExpectedVersion: version, IdempotencyKey: "valid-key"}
		next, err := change.Apply(set.SMS, time.Now())
		if err != nil || next.Version != version+1 {
			t.Fatalf("CAS version %d: %+v %v", version, next, err)
		}
		set.SMS = next
		if !reflect.DeepEqual(set.Email, originalEmail) {
			t.Fatal("email changed while changing SMS")
		}
	}
}
