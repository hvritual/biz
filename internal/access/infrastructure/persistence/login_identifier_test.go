package persistence

import "testing"

func TestEnterprise172NormalizeLoginIdentifier(t *testing.T) {
	cases := []struct {
		value string
		kind  LoginIdentifierKind
		want  string
		ok    bool
	}{
		{"User.Name", LoginIdentifierUsername, "user.name", true},
		{"user@example.invalid", LoginIdentifierEmail, "user@example.invalid", true},
		{"+49 170 123-4567", LoginIdentifierPhone, "+491701234567", true},
		{"12345", "", "", false},
		{"abcd", "", "", false},
		{"this-username-is-way-too-long", "", "", false},
		{"bad user", "", "", false},
	}
	for _, tc := range cases {
		kind, normalized, err := NormalizeLoginIdentifier(tc.value)
		if tc.ok {
			if err != nil || kind != tc.kind || normalized != tc.want {
				t.Fatalf("%q => kind=%q normalized=%q err=%v", tc.value, kind, normalized, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("%q unexpectedly accepted as %q/%q", tc.value, kind, normalized)
		}
	}
}
