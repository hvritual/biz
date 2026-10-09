package persistence

import (
	"testing"

	"github.com/hvritual/biz/internal/commercial/domain/subscription"
	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
)

func TestChangePreviewSourceNullabilityIsActionSpecific(t *testing.T) {
	code, version := "plan-a", uint64(1)
	initial := change.Preview{Input: change.Input{Action: change.Initial}}
	legacy := change.Preview{Input: change.Input{Action: change.Switch}, Before: subscription.Subscription{PlanCode: code, PlanVersion: version}}
	for _, tc := range []struct {
		name    string
		row     changePreviewRow
		preview change.Preview
		want    bool
	}{
		{"initial has no source", changePreviewRow{}, initial, true},
		{"initial rejects fabricated source", changePreviewRow{SourcePlanCode: &code, SourcePlanVersion: &version}, initial, false},
		{"initial rejects half source", changePreviewRow{SourcePlanCode: &code}, initial, false},
		{"legacy exact source", changePreviewRow{SourcePlanCode: &code, SourcePlanVersion: &version}, legacy, true},
		{"legacy rejects absent source", changePreviewRow{}, legacy, false},
		{"legacy rejects half source", changePreviewRow{SourcePlanVersion: &version}, legacy, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.row.matchesSource(tc.preview); got != tc.want {
				t.Fatalf("matchesSource=%v want=%v", got, tc.want)
			}
		})
	}
}
