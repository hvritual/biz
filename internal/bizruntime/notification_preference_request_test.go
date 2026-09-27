package bizruntime

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hvritual/biz/internal/access/domain"
)

func TestNotificationPreferenceRequestRequiresExactCompleteCommand(t *testing.T) {
	cases := []struct {
		name, body string
		valid      bool
	}{
		{"explicit-false-zero", `{"channel":"sms","allowed":false,"expected_version":0}`, true},
		{"explicit-true", `{"expected_version":42,"allowed":true,"channel":"email"}`, true},
		{"whitespace", " \n{\"channel\":\"sms\",\"allowed\":false,\"expected_version\":0} \n", true},
		{"missing-allowed", `{"channel":"sms","expected_version":0}`, false},
		{"missing-version", `{"channel":"sms","allowed":false}`, false},
		{"missing-channel", `{"allowed":false,"expected_version":0}`, false},
		{"null-allowed", `{"channel":"sms","allowed":null,"expected_version":0}`, false},
		{"null-version", `{"channel":"sms","allowed":false,"expected_version":null}`, false},
		{"null-channel", `{"channel":null,"allowed":false,"expected_version":0}`, false},
		{"duplicate-allowed", `{"channel":"sms","allowed":true,"allowed":false,"expected_version":0}`, false},
		{"escaped-duplicate", `{"channel":"sms","allowed":true,"\u0061llowed":false,"expected_version":0}`, false},
		{"tenant-injection", `{"channel":"sms","allowed":false,"expected_version":0,"tenant_id":"B"}`, false},
		{"user-injection", `{"channel":"sms","allowed":false,"expected_version":0,"user_id":"other"}`, false},
		{"importance-bypass", `{"channel":"sms","allowed":false,"expected_version":0,"importance":"critical"}`, false},
		{"unknown-channel", `{"channel":"push","allowed":false,"expected_version":0}`, false},
		{"case-sensitive-channel", `{"channel":"SMS","allowed":false,"expected_version":0}`, false},
		{"boolean-string", `{"channel":"sms","allowed":"false","expected_version":0}`, false},
		{"boolean-number", `{"channel":"sms","allowed":1,"expected_version":0}`, false},
		{"negative-version", `{"channel":"sms","allowed":false,"expected_version":-1}`, false},
		{"fractional-version", `{"channel":"sms","allowed":false,"expected_version":1.5}`, false},
		{"overflow-version", `{"channel":"sms","allowed":false,"expected_version":18446744073709551616}`, false},
		{"array", `[]`, false}, {"null", `null`, false}, {"empty", ``, false},
		{"trailing-json", `{"channel":"sms","allowed":false,"expected_version":0}{}`, false},
		{"trailing-garbage", `{"channel":"sms","allowed":false,"expected_version":0}x`, false},
		{"oversize", strings.Repeat(" ", 4097) + `{}`, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			writer := httptest.NewRecorder()
			request := httptest.NewRequest("POST", "/auth/personal/notification-preferences", strings.NewReader(test.body))
			command, err := decodeNotificationPreferenceChange(writer, request, "command-1")
			if test.valid {
				if err != nil {
					t.Fatal(err)
				}
				if command.IdempotencyKey != "command-1" || !command.Channel.Valid() {
					t.Fatalf("invalid command: %+v", command)
				}
				if test.name == "explicit-false-zero" && (command.Allowed || command.ExpectedVersion != 0) {
					t.Fatal("false or version zero lost")
				}
			} else if !errors.Is(err, domain.ErrNotificationPreferenceInvalid) {
				t.Fatalf("want invalid, got %+v / %v", command, err)
			}
		})
	}
}

func TestNotificationPreferenceRequestRejectsInvalidIdempotencyKeys(t *testing.T) {
	for _, key := range []string{"", " space", "with space", "a\nb", strings.Repeat("x", 257), "键"} {
		request := httptest.NewRequest("POST", "/auth/personal/notification-preferences", strings.NewReader(`{"channel":"email","allowed":false,"expected_version":0}`))
		if _, err := decodeNotificationPreferenceChange(httptest.NewRecorder(), request, key); !errors.Is(err, domain.ErrNotificationPreferenceInvalid) {
			t.Fatalf("invalid key accepted: %q", key)
		}
	}
}
