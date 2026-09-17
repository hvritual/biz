package persistence

import (
	"errors"
	"strings"
	"testing"
)

func testProtection(t *testing.T, active string, keys map[string][]byte) *ContactProtection {
	t.Helper()
	protection, err := NewContactProtection(ContactProtectionConfig{ActiveVersion: active, Keys: keys, LookupKey: []byte(strings.Repeat("L", 32))})
	if err != nil {
		t.Fatal(err)
	}
	return protection
}

func TestContactProtectionRoundTripLookupMaskAndRotation(t *testing.T) {
	v1 := []byte(strings.Repeat("1", 32))
	v2 := []byte(strings.Repeat("2", 32))
	oldProtection := testProtection(t, "v1", map[string][]byte{"v1": v1})
	cipherOne, lookupOne, versionOne, err := oldProtection.ProtectEmail(" User@Example.COM ")
	if err != nil {
		t.Fatal(err)
	}
	cipherTwo, lookupTwo, _, err := oldProtection.ProtectEmail("user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if cipherOne == cipherTwo || lookupOne != lookupTwo || versionOne != "v1" {
		t.Fatal("random ciphertext or deterministic lookup contract lost")
	}
	rotated := testProtection(t, "v2", map[string][]byte{"v1": v1, "v2": v2})
	plain, err := rotated.DecryptEmail(cipherOne, "v1")
	if err != nil || plain != "user@example.com" || MaskEmail(plain) != "u***@example.com" {
		t.Fatalf("old key rotation read/mask failed: %q %v", plain, err)
	}
	phoneCipher, _, version, err := rotated.ProtectPhone("+49 (170) 123-4567")
	if err != nil || version != "v2" {
		t.Fatalf("phone protection failed: %v %q", err, version)
	}
	phone, err := rotated.DecryptPhone(phoneCipher, "v2")
	if err != nil || phone != "+491701234567" || !strings.Contains(MaskPhone(phone), "****") {
		t.Fatalf("phone roundtrip/mask failed: %q %v", phone, err)
	}
}

func TestContactProtectionFailsClosed(t *testing.T) {
	protection := testProtection(t, "v1", map[string][]byte{"v1": []byte(strings.Repeat("1", 32))})
	ciphertext, _, _, err := protection.ProtectEmail("user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := protection.DecryptEmail(ciphertext, "missing"); !errors.Is(err, ErrSensitiveDataKeyUnavailable) {
		t.Fatalf("missing key must fail closed: %v", err)
	}
	if _, err := protection.DecryptEmail(ciphertext+"x", "v1"); !errors.Is(err, ErrSensitiveDataCorrupt) {
		t.Fatalf("corrupt ciphertext must fail closed: %v", err)
	}
	for _, value := range []string{"not-an-email", "a @example.com", ""} {
		if _, err := NormalizeEmail(value); !errors.Is(err, ErrInvalidContact) {
			t.Fatalf("invalid email accepted: %q %v", value, err)
		}
	}
	if _, err := NormalizePhone("12x34"); !errors.Is(err, ErrInvalidContact) {
		t.Fatalf("invalid phone accepted: %v", err)
	}
}
