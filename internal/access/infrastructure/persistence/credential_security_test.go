package persistence

import (
	"errors"
	"testing"
)

func TestEnterprise173UserPasswordPolicy(t *testing.T) {
	valid := []string{"Abcd1234", "Coffee99A", "A234567890123456"}
	for _, candidate := range valid {
		if err := ValidateUserChosenPassword(candidate); err != nil {
			t.Fatalf("valid password %q rejected: %v", candidate, err)
		}
	}
	invalid := []string{"short1A", "abcdefgh", "ABCDEFGH", "Abcdefghijklmnop1", "密码Abc1"}
	for _, candidate := range invalid {
		if err := ValidateUserChosenPassword(candidate); !errors.Is(err, ErrWeakUserPassword) {
			t.Fatalf("invalid password %q accepted: %v", candidate, err)
		}
	}
}
