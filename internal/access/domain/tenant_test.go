package domain

import (
	"errors"
	"testing"
	"time"
)

func TestTenantLifecycleStateMachine(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	tenant := NewTenant("tenant-a", "Tenant A", now)
	if tenant.Status != TenantStatusPending || tenant.Version != 1 {
		t.Fatalf("new tenant=%+v", tenant)
	}
	if err := tenant.Suspend(now.Add(time.Second)); !errors.Is(err, ErrInvalidTenantTransition) {
		t.Fatalf("pending suspend err=%v", err)
	}
	if err := tenant.Activate(now.Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if tenant.Status != TenantStatusActive {
		t.Fatalf("status=%s", tenant.Status)
	}
	if err := tenant.Suspend(now.Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if tenant.Status != TenantStatusSuspended {
		t.Fatalf("status=%s", tenant.Status)
	}
	if err := tenant.Activate(now.Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := tenant.Close(now.Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if tenant.Status != TenantStatusClosed {
		t.Fatalf("status=%s", tenant.Status)
	}
	if err := tenant.Activate(now.Add(6 * time.Second)); !errors.Is(err, ErrInvalidTenantTransition) {
		t.Fatalf("closed activate err=%v", err)
	}
	if err := tenant.Rename("Closed", now.Add(7*time.Second)); !errors.Is(err, ErrInvalidTenantTransition) {
		t.Fatalf("closed rename err=%v", err)
	}
}
