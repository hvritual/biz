package persistence

import (
	"encoding/hex"
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


func TestPBKDF2SHA256KnownVectors(t *testing.T) {
	tests := []struct {
		iterations int
		expected   string
	}{
		{iterations: 1, expected: "120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b"},
		{iterations: 2, expected: "ae4d0c95af6b46d32d0adff928f06dd02a303f8ef3c251dfd6e2d85a95474c43"},
	}
	for _, test := range tests {
		actual := hex.EncodeToString(pbkdf2SHA256([]byte("password"), []byte("salt"), test.iterations, 32))
		if actual != test.expected {
			t.Fatalf("iterations=%d got=%s want=%s", test.iterations, actual, test.expected)
		}
	}
}
