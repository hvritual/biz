package main

import "testing"

func validAudits() []audit {
	return []audit{
		{ID: 1, ModuleCode: "core", Actor: "platform-admin:ce02", Action: "create", Reason: "create", RequestID: "core"},
		{ID: 3, ModuleCode: "future", Actor: "platform-admin:ce02", Action: "create", Reason: "registered but unavailable", RequestID: "future"},
	}
}

func TestValidateCE02Audits(t *testing.T) {
	if err := validateCE02Audits(validAudits()); err != nil {
		t.Fatalf("valid CE02 audit rejected: %v", err)
	}
	for name, mutate := range map[string]func([]audit){
		"wrong actor":   func(rows []audit) { rows[0].Actor = "other" },
		"wrong reason":  func(rows []audit) { rows[1].Reason = "other" },
		"wrong request": func(rows []audit) { rows[0].RequestID = "future" },
		"wrong code":    func(rows []audit) { rows[0].ModuleCode = "other" },
		"wrong count":   func(rows []audit) { rows = rows[:1] },
	} {
		t.Run(name, func(t *testing.T) {
			rows := validAudits()
			if name == "wrong count" {
				rows = rows[:1]
			} else {
				mutate(rows)
			}
			if err := validateCE02Audits(rows); err == nil {
				t.Fatal("invalid CE02 audit accepted")
			}
		})
	}
}

func TestValidateDSN(t *testing.T) {
	if err := validateDSN("u:p@tcp(127.0.0.1:13316)/biz_evolution?parseTime=true"); err != nil {
		t.Fatalf("valid DSN rejected: %v", err)
	}
	for _, dsn := range []string{"u:p@tcp(localhost:13316)/biz_evolution", "u:p@tcp(127.0.0.1:13317)/biz_evolution", "u:p@tcp(127.0.0.1:13316)/other"} {
		if err := validateDSN(dsn); err == nil {
			t.Fatalf("wrong DSN accepted: %q", dsn)
		}
	}
}
