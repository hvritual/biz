package persistence

import (
	"errors"
	"strings"
	"testing"

	"github.com/hvritual/biz/internal/access/domain"
)

func testVerificationProtection(t *testing.T) *VerificationProtection {
	t.Helper()
	protection, err := NewVerificationProtection(VerificationProtectionConfig{
		ActiveVersion: "v1",
		Keys:          map[string][]byte{"v1": []byte(strings.Repeat("K", 32))},
		HMACKey:       []byte(strings.Repeat("H", 32)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return protection
}

func TestEnterprise170VerificationProtectionRoundTripAndPurposeSeparation(t *testing.T) {
	protection := testVerificationProtection(t)
	code, err := protection.GenerateCode(6)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Fatalf("code length=%d", len(code))
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			t.Fatalf("code is not numeric: %q", code)
		}
	}
	emailHash, normalized, err := protection.DestinationHash(domain.SecurityNotificationEmail, " User@Example.COM ")
	if err != nil {
		t.Fatal(err)
	}
	if normalized != "user@example.com" || emailHash == "" {
		t.Fatalf("unexpected destination normalization/hash: %q %q", normalized, emailHash)
	}
	bindingLogin := protection.BindingHash(domain.VerificationPurposeLogin, "u1", "t1", "flow", domain.SecurityNotificationEmail, emailHash)
	bindingReset := protection.BindingHash(domain.VerificationPurposePasswordRecovery, "u1", "t1", "flow", domain.SecurityNotificationEmail, emailHash)
	if bindingLogin == bindingReset {
		t.Fatal("verification purposes must be cryptographically separated")
	}
	eventID := stableSecurityEventID("business-event-1")
	targetCipher, version, err := protection.ProtectNotification(eventID, "destination", normalized)
	if err != nil {
		t.Fatal(err)
	}
	secretCipher, _, err := protection.ProtectNotification(eventID, "secret", code)
	if err != nil {
		t.Fatal(err)
	}
	target, err := protection.DecryptNotification(eventID, "destination", targetCipher, version)
	if err != nil || target != normalized {
		t.Fatalf("destination round trip failed: %q %v", target, err)
	}
	secret, err := protection.DecryptNotification(eventID, "secret", secretCipher, version)
	if err != nil || secret != code {
		t.Fatalf("secret round trip failed: %q %v", secret, err)
	}
	if _, err := protection.DecryptNotification(eventID, "secret", secretCipher+"x", version); !errors.Is(err, ErrVerificationCipherCorrupt) {
		t.Fatalf("corrupt notification ciphertext accepted: %v", err)
	}
}

func TestEnterprise170VerificationPolicyRequiresExplicitLimits(t *testing.T) {
	valid := domain.VerificationPolicy{
		CodeTTL:              5 * 60 * 1000000000,
		AuthorizationTTL:     5 * 60 * 1000000000,
		ResendInterval:       60 * 1000000000,
		SendLimitWindow:      24 * 60 * 60 * 1000000000,
		MaxSendsPerWindow:    5,
		MaxVerificationTries: 5,
		CodeDigits:           6,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid policy rejected: %v", err)
	}
	invalid := valid
	invalid.ResendInterval = 59 * 1000000000
	if err := invalid.Validate(); err == nil {
		t.Fatal("resend interval below 60s accepted")
	}
}
