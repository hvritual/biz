package bizruntime

import (
	"encoding/json"
	"errors"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

var errWebSessionContextChanged = errors.New("web session context changed")

// A client expectation is only a comparison precondition, never an identity source.
func validateExpectedWebSession(raw string, actual accesspersistence.WebSessionContext) error {
	if raw == "" {
		return nil
	}
	if len(raw) > 2048 {
		return errWebSessionContextChanged
	}
	var expected struct {
		ActorKind       string `json:"actor_kind"`
		PlatformSubject string `json:"platform_subject"`
		UserID          string `json:"user_id"`
		ActiveTenantID  string `json:"active_tenant_id"`
	}
	if json.Unmarshal([]byte(raw), &expected) != nil || expected.ActorKind != actual.ActorKind || expected.PlatformSubject != actual.PlatformSubject || expected.UserID != actual.UserID || expected.ActiveTenantID != actual.ActiveTenantID {
		return errWebSessionContextChanged
	}
	return nil
}
