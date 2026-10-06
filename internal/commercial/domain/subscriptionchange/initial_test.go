package subscriptionchange

import (
	"errors"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/commercial/domain/subscription"
)

func TestInitialSubscriptionChangeRequiresExplicitTenantSalesScope(t *testing.T) {
	input := Input{
		TenantID:          "tenant-initial",
		RequestID:         "initial-request",
		Action:            Initial,
		TargetPlanCode:    "office-pro",
		TargetPlanVersion: 3,
		SalesScope:        "rental",
		Reason:            "platform first activation",
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("valid initial activation rejected: %v", err)
	}

	for name, mutate := range map[string]func(*Input){
		"missing sales scope": func(value *Input) { value.SalesScope = "" },
		"wildcard tenant scope": func(value *Input) { value.SalesScope = "*" },
		"missing exact version": func(value *Input) { value.TargetPlanVersion = 0 },
		"scheduled initial activation": func(value *Input) {
			at := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
			value.EffectiveAt = &at
		},
	} {
		t.Run(name, func(t *testing.T) {
			value := input
			mutate(&value)
			if !errors.Is(value.Validate(), ErrInvalid) {
				t.Fatal("invalid initial activation request was accepted")
			}
		})
	}
}

func TestExistingSubscriptionChangesCannotOverrideStoredSalesScope(t *testing.T) {
	for _, action := range []string{Switch, Renew, StopRenewal} {
		input := Input{
			TenantID:   "tenant-existing",
			RequestID:  "existing-request",
			Action:     action,
			SalesScope: "rental",
			Reason:     "must use stored subscription scope",
		}
		if action != StopRenewal {
			input.TargetPlanCode = "office-pro"
			input.TargetPlanVersion = 3
		}
		if !errors.Is(input.Validate(), ErrInvalid) {
			t.Fatalf("%s accepted a client sales scope override", action)
		}
	}
}

func TestInitialSubscriptionPeriodStartsAtAuthoritativeAdmissionTime(t *testing.T) {
	now := time.Date(2026, 10, 6, 9, 30, 0, 0, time.UTC)
	terms := ce09Terms(10, false)
	mode, at, end, err := Period(Input{Action: Initial}, subscription.Subscription{}, Initial, now, terms)
	if err != nil {
		t.Fatal(err)
	}
	if mode != Immediate || !at.Equal(now) || end == nil || !end.Equal(now.AddDate(0, 0, int(terms.ValidityDays))) {
		t.Fatalf("unexpected initial period mode=%s at=%s end=%v", mode, at, end)
	}
}
