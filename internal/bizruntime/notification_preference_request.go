package bizruntime

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/hvritual/biz/internal/access/domain"
)

// Require all three exact keys, reject unknown/duplicate/null fields and trailing
// JSON, and distinguish omitted allowed=false or expected_version=0 from values.
func decodeNotificationPreferenceChange(writer http.ResponseWriter, request *http.Request, key string) (domain.NotificationPreferenceChange, error) {
	invalid := domain.ErrNotificationPreferenceInvalid
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 4096))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return domain.NotificationPreferenceChange{}, invalid
	}
	values := make(map[string]json.RawMessage, 3)
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return domain.NotificationPreferenceChange{}, invalid
		}
		name, ok := token.(string)
		if !ok || (name != "channel" && name != "allowed" && name != "expected_version") {
			return domain.NotificationPreferenceChange{}, invalid
		}
		if _, exists := values[name]; exists {
			return domain.NotificationPreferenceChange{}, invalid
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil || string(value) == "null" {
			return domain.NotificationPreferenceChange{}, invalid
		}
		values[name] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') || len(values) != 3 {
		return domain.NotificationPreferenceChange{}, invalid
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return domain.NotificationPreferenceChange{}, invalid
	}
	var change domain.NotificationPreferenceChange
	if json.Unmarshal(values["channel"], &change.Channel) != nil ||
		json.Unmarshal(values["allowed"], &change.Allowed) != nil ||
		json.Unmarshal(values["expected_version"], &change.ExpectedVersion) != nil {
		return domain.NotificationPreferenceChange{}, invalid
	}
	change.IdempotencyKey = key
	return change, change.Validate()
}
