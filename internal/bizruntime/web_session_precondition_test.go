package bizruntime

import (
	"encoding/json"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"testing"
)

func TestWebSessionPreconditionRejectsCrossTabTenantAndActorChanges(t *testing.T) {
	actual := accesspersistence.WebSessionContext{ActorKind: "user", UserID: "user-a", ActiveTenantID: "tenant-b"}
	encode := func(kind, subject, user, tenant string) string {
		b, _ := json.Marshal(map[string]string{"actor_kind": kind, "platform_subject": subject, "user_id": user, "active_tenant_id": tenant})
		return string(b)
	}
	if validateExpectedWebSession(encode("user", "", "user-a", "tenant-a"), actual) == nil {
		t.Fatal("old tenant draft accepted after session switch")
	}
	if validateExpectedWebSession(encode("user", "", "user-a", "tenant-b"), actual) != nil {
		t.Fatal("matching tenant context rejected")
	}
	actual = accesspersistence.WebSessionContext{ActorKind: "platform", PlatformSubject: "platform-b"}
	if validateExpectedWebSession(encode("platform", "platform-a", "", ""), actual) == nil {
		t.Fatal("old platform operator draft accepted")
	}
	if validateExpectedWebSession(encode("platform", "platform-b", "", ""), actual) != nil {
		t.Fatal("matching platform rejected")
	}
	if validateExpectedWebSession("{", actual) == nil {
		t.Fatal("invalid expectation accepted")
	}
	// Absence does not replace authentication; existing API callers retain their auth path.
	if validateExpectedWebSession("", actual) != nil {
		t.Fatal("legacy authenticated callers broken")
	}
}
