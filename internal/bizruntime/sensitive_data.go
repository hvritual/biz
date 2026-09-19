package bizruntime

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

// BuildContactProtection parses the process-level PII key configuration used by
// both the Biz API runtime and the first-party IdP. All values empty means the
// legacy compatibility mode; any partial configuration fails closed.
func BuildContactProtection(activeVersion, keysJSON, lookupKeyBase64 string) (*accesspersistence.ContactProtection, error) {
	activeVersion = strings.TrimSpace(activeVersion)
	keysJSON = strings.TrimSpace(keysJSON)
	lookupKeyBase64 = strings.TrimSpace(lookupKeyBase64)
	if activeVersion == "" && keysJSON == "" && lookupKeyBase64 == "" {
		return nil, nil
	}
	if activeVersion == "" || keysJSON == "" || lookupKeyBase64 == "" {
		return nil, errors.New("biz runtime: PII active version, key set and lookup key must be configured together")
	}
	var encoded map[string]string
	if err := json.Unmarshal([]byte(keysJSON), &encoded); err != nil || len(encoded) == 0 {
		return nil, errors.New("biz runtime: YUNKA_BIZ_PII_KEYS_JSON must be a non-empty JSON object")
	}
	keys := make(map[string][]byte, len(encoded))
	for version, value := range encoded {
		version = strings.TrimSpace(version)
		if version == "" {
			return nil, errors.New("biz runtime: PII key version must not be empty")
		}
		decoded, err := decodePIISecret(value)
		if err != nil {
			return nil, fmt.Errorf("biz runtime: decode PII key %q: %w", version, err)
		}
		keys[version] = decoded
	}
	lookupKey, err := decodePIISecret(lookupKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("biz runtime: decode PII lookup key: %w", err)
	}
	return accesspersistence.NewContactProtection(accesspersistence.ContactProtectionConfig{
		ActiveVersion: activeVersion,
		Keys:          keys,
		LookupKey:     lookupKey,
	})
}

func decodePIISecret(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("empty base64 secret")
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := encoding.DecodeString(value); err == nil {
			return decoded, nil
		}
	}
	return nil, errors.New("invalid base64 secret")
}
