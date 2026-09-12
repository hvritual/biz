package main

import "testing"

func TestValidateLocalDSNRequiresExactEndpointAndDatabase(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want bool
	}{
		{name: "designated database", dsn: "user:password@tcp(127.0.0.1:13316)/biz_evolution?parseTime=true", want: true},
		{name: "database prefix is not enough", dsn: "user:password@tcp(127.0.0.1:13316)/biz_evolution_other", want: false},
		{name: "database in password is not enough", dsn: "user:biz_evolution@tcp(127.0.0.1:13316)/other", want: false},
		{name: "port must match", dsn: "user:password@tcp(127.0.0.1:3306)/biz_evolution", want: false},
		{name: "host must match", dsn: "user:password@tcp(localhost:13316)/biz_evolution", want: false},
		{name: "network must be tcp", dsn: "user:password@unix(/tmp/mysql.sock)/biz_evolution", want: false},
		{name: "malformed", dsn: "not a mysql dsn", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateLocalDSN(test.dsn)
			if (err == nil) != test.want {
				t.Fatalf("validateLocalDSN(%q) error=%v, want success=%v", test.dsn, err, test.want)
			}
		})
	}
}
