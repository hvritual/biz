//go:build integration

package integration

import (
	"encoding/json"
	"testing"
	"time"

	transition "github.com/hvritual/biz/internal/commercial/domain/timetransition"
)

// ce16InsertDueTransition creates a fully sealed test authority record. Tests
// move only this fixture's business instant; production clocks remain DB-owned.
func ce16InsertDueTransition(t *testing.T, e *ce10Environment, kind, authority string, version uint64, due time.Time, timezone string) string {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	task, err := transition.NewInTimezone(kind, e.tenant, authority, version, due.UTC().Truncate(time.Microsecond), now, timezone)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.db.Table("biz_commercial_time_transitions").Create(map[string]any{
		"transition_id": task.ID, "kind": task.Kind, "tenant_id": task.TenantID, "authority_id": task.AuthorityID,
		"authority_version": task.AuthorityVersion, "due_at": task.DueAt, "business_timezone": task.BusinessTimezone,
		"revision": task.Revision, "state": task.State, "payload_sha256": task.Hash, "payload": string(payload),
		"created_at": task.CreatedAt, "updated_at": task.UpdatedAt,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return task.ID
}

func TestCE16MySQLConfiguredTimezoneIsPersistedForDueAuthority(t *testing.T) {
	e := ce10New(t)
	id := ce16InsertDueTransition(t, e, transition.EntitlementExpiry, "ce16-dst-source", 1, time.Now().UTC().Add(time.Hour), "America/New_York")
	var timezone string
	if err := e.db.Table("biz_commercial_time_transitions").Select("business_timezone").Where("transition_id=?", id).Scan(&timezone).Error; err != nil || timezone != "America/New_York" {
		t.Fatalf("timezone=%q err=%v", timezone, err)
	}
}
