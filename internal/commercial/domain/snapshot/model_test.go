package snapshot

import (
	"encoding/json"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"testing"
	"time"
)

func TestCE06SnapshotImmutableBounds(t *testing.T) {
	at := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	end := at.Add(time.Second)
	base := entitlement.Result{TenantID: "a", SourceVersion: 3, EntitlementVersion: 4, CatalogRevision: 2, ResolverVersion: 1, EvaluatedAt: at, ValidUntil: &end}
	b, _ := json.Marshal(base)
	hash := Digest(b)
	for _, tc := range []struct {
		name, tenant             string
		version, source, catalog uint64
		at                       time.Time
		data                     []byte
		ok                       bool
	}{
		{"valid", "a", 4, 3, 2, at, b, true}, {"before_start", "a", 4, 3, 2, at.Add(-time.Microsecond), b, false}, {"at_expiry", "a", 4, 3, 2, end, b, false}, {"after_expiry", "a", 4, 3, 2, end.Add(time.Microsecond), b, false}, {"other_tenant", "b", 4, 3, 2, at, b, false}, {"old_version", "a", 3, 3, 2, at, b, false}, {"old_source", "a", 4, 2, 2, at, b, false}, {"old_catalog", "a", 4, 3, 1, at, b, false}, {"corrupted", "a", 4, 3, 2, at, []byte(`{"allowed":true}`), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode(tc.data, tc.tenant, tc.version, tc.source, tc.catalog, hash, tc.at)
			if (err == nil) != tc.ok {
				t.Fatal(err)
			}
		})
	}
	t.Run("permission_never_in_tenant_cache", func(t *testing.T) {
		v := base
		v.PermissionVersion = "other-member"
		p, _ := json.Marshal(v)
		if _, err := Decode(p, "a", 4, 3, 2, Digest(p), at); err == nil {
			t.Fatal("principal data cached")
		}
	})
	t.Run("unknown_request_denied_once", func(t *testing.T) {
		r := Requested(base, []string{"unknown", "unknown"})
		if len(r.Decisions) != 1 || r.Decisions[0].Allowed {
			t.Fatal(r)
		}
	})
}
