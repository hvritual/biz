package main

import "testing"

func TestCE16GraceDurationRejectsInvalidAndNegativeValues(t *testing.T) {
	t.Setenv("YUNKA_BIZ_COMMERCIAL_GRACE_DURATION", "bad")
	if _, err := lifecycleConfiguration(); err == nil {
		t.Fatal("invalid grace accepted")
	}
	t.Setenv("YUNKA_BIZ_COMMERCIAL_GRACE_DURATION", "-1h")
	if _, err := lifecycleConfiguration(); err == nil {
		t.Fatal("negative grace accepted")
	}
	t.Setenv("YUNKA_BIZ_COMMERCIAL_GRACE_DURATION", "0")
	if policy, err := lifecycleConfiguration(); err != nil || policy.GraceDuration != 0 {
		t.Fatalf("zero grace=%v err=%v", policy.GraceDuration, err)
	}
}
