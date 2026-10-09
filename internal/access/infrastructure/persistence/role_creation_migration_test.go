package persistence

import "testing"

func TestCE293RoleReceiptCheckClauseRejectsWeakerOrDifferentPredicates(t *testing.T) {
	tests := []struct {
		name   string
		clause string
		allow  bool
	}{
		{"versioned SQL", "payload IS NULL OR JSON_VALID(payload)", true},
		{"mysql normalized clause", "((`payload` is null) or json_valid(`payload`))", true},
		{"gorm nested grouping", "(( ( `payload` IS NULL ) OR (JSON_VALID(`payload`)) ))", true},
		{"constant true", "1 = 1", false},
		{"extra OR bypass", "payload IS NULL OR JSON_VALID(payload) OR 1=1", false},
		{"different field", "payload IS NULL OR JSON_VALID(fingerprint)", false},
		{"inverted condition", "payload IS NULL OR NOT JSON_VALID(payload)", false},
		{"incomplete condition", "JSON_VALID(payload)", false},
		{"different predicate", "payload IS NOT NULL OR JSON_VALID(payload)", false},
		{"empty metadata", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validRoleReceiptCheckClause(tt.clause); got != tt.allow {
				t.Fatalf("unexpected schema predicate acceptance: clause=%q got=%t want=%t", tt.clause, got, tt.allow)
			}
		})
	}
}
