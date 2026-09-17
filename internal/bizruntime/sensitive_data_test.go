package bizruntime

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestBuildContactProtectionRequiresCompleteConfig(t *testing.T) {
	if value, err := BuildContactProtection("", "", ""); err != nil || value != nil {
		t.Fatalf("legacy mode must remain explicit and empty: %v %v", value, err)
	}
	if _, err := BuildContactProtection("v1", "", ""); err == nil {
		t.Fatal("partial protected configuration must fail closed")
	}
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("K", 32)))
	lookup := base64.RawStdEncoding.EncodeToString([]byte(strings.Repeat("L", 32)))
	protection, err := BuildContactProtection("v1", `{"v1":"`+key+`"}`, lookup)
	if err != nil || protection == nil || protection.ActiveVersion() != "v1" {
		t.Fatalf("valid protected config rejected: %v", err)
	}
}
