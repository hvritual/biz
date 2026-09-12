package timetransition

import (
	"errors"
	"testing"
	"time"
)

func TestCE16LeaseReclaimRejectsStaleWorkerAndTerminalReclaim(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 123456000, time.UTC)
	task, err := New(ScheduledChange, "tenant-1", "change-1", 1, now, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = task.Claim("worker-a", now, 5*time.Second); err != nil {
		t.Fatal(err)
	}
	first := task.LeaseToken
	if err = task.Claim("worker-b", now.Add(4*time.Second), 5*time.Second); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err = task.Claim("worker-b", now.Add(5*time.Second), 5*time.Second); err != nil {
		t.Fatal(err)
	}
	if err = task.Finish("worker-a", first, Applied, "applied", now.Add(6*time.Second)); !errors.Is(err, ErrLease) {
		t.Fatal(err)
	}
	if err = task.Finish("worker-b", task.LeaseToken, Applied, "applied", now.Add(6*time.Second)); err != nil {
		t.Fatal(err)
	}
	if task.Due(now.Add(time.Hour)) || !task.Terminal() {
		t.Fatal("terminal transition became executable")
	}
	if err = task.Claim("worker-c", now.Add(time.Hour), 5*time.Second); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}
func TestCE16TimeBoundaryAndTamperIntegrity(t *testing.T) {
	now := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	due := now.Add(time.Hour)
	task, err := NewInTimezone(EntitlementExpiry, "tenant-1", "source-1", 2, due, now, "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	if task.Due(due.Add(-time.Microsecond)) || !task.Due(due) {
		t.Fatal("incorrect due boundary")
	}
	task.AuthorityVersion++
	if !errors.Is(task.Integrity(), ErrCorrupt) {
		t.Fatal("tampered authority accepted")
	}
	if _, err = NewInTimezone(EntitlementExpiry, "tenant-1", "source-1", 2, due, now, "unknown/zone"); err == nil {
		t.Fatal("untrusted timezone accepted")
	}
}
