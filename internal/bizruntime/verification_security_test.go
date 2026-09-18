package bizruntime

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestEnterprise170VerificationRuntimeConfigIsExplicitAndFailClosed(t *testing.T) {
	config := VerificationSecurityConfig{
		CodeTTL: 5 * time.Minute,
		AuthorizationTTL: 5 * time.Minute,
		ResendInterval: time.Minute,
		SendLimitWindow: 24 * time.Hour,
		MaxSendsPerWindow: 5,
		MaxVerificationTries: 5,
		CodeDigits: 6,
		Notification: SecurityNotificationProviderConfig{Provider: "disabled"},
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("valid verification config rejected: %v", err)
	}
	invalid := config
	invalid.ResendInterval = 59 * time.Second
	if err := invalid.Validate(); err == nil {
		t.Fatal("runtime accepted resend interval below server minimum")
	}
	partialExternal := config
	partialExternal.Notification = SecurityNotificationProviderConfig{Provider: "external", Endpoint: "https://notify.example.invalid"}
	if err := partialExternal.Validate(); err == nil {
		t.Fatal("partial external notification configuration accepted")
	}
}

func TestEnterprise170BuildVerificationProtectionRequiresCompleteKeySet(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("K", 32)))
	hmac := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("H", 32)))
	protection, err := BuildVerificationProtection("v1", `{"v1":"` + key + `"}`, hmac)
	if err != nil || protection == nil {
		t.Fatalf("complete verification key set rejected: %v", err)
	}
	if _, err := BuildVerificationProtection("v1", "", hmac); err == nil {
		t.Fatal("partial verification key configuration accepted")
	}
}
