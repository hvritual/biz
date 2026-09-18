package bizruntime

import (
	"encoding/json"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"testing"
)

func TestWebSessionPreconditionRejectsCrossTabTenantAndActorChanges(t *testing.T) {
	actual := accesspersistence.WebSessionContext{ActorKind: "user", UserID: "user-a", ActiveTenantID: "tenant-b", ContextVersion: 2}
	encode := func(kind, subject, user, tenant string, version uint64) string {
		b, _ := json.Marshal(map[string]any{"actor_kind": kind, "platform_subject": subject, "user_id": user, "active_tenant_id": tenant, "context_version": version})
		return string(b)
	}
	if validateExpectedWebSession(encode("user", "", "user-a", "tenant-a", 1), actual) == nil {
		t.Fatal("old tenant draft accepted after session switch")
	}
	if validateExpectedWebSession(encode("user", "", "user-a", "tenant-b", 2), actual) != nil {
		t.Fatal("matching tenant context rejected")
	}
	actual = accesspersistence.WebSessionContext{ActorKind: "platform", PlatformSubject: "platform-b", ContextVersion: 1}
	if validateExpectedWebSession(encode("platform", "platform-a", "", "", 1), actual) == nil {
		t.Fatal("old platform operator draft accepted")
	}
	if validateExpectedWebSession(encode("platform", "platform-b", "", "", 1), actual) != nil {
		t.Fatal("matching platform rejected")
	}
	if validateExpectedWebSession(encode("user", "", "user-a", "tenant-b", 1), accesspersistence.WebSessionContext{ActorKind: "user", UserID: "user-a", ActiveTenantID: "tenant-b", ContextVersion: 2}) == nil {
		t.Fatal("stale context version accepted after tenant switch")
	}
	if validateExpectedWebSession("{", actual) == nil {
		t.Fatal("invalid expectation accepted")
	}
	// Absence does not replace authentication; existing API callers retain their auth path.
	if validateExpectedWebSession("", actual) != nil {
		t.Fatal("legacy authenticated callers broken")
	}
}
