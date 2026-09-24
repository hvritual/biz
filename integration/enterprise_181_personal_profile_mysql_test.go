//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"google.golang.org/protobuf/encoding/protojson"
)

func enterprise181HTTP(t *testing.T, method, endpoint, token, key string, payload []byte) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, endpoint, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if len(payload) > 0 {
		request.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, body
}

func enterprise181Profile(t *testing.T, base, token string) (*accessv1.TenantPersonalProfileDTO, int, []byte) {
	t.Helper()
	statusCode, body := enterprise181HTTP(t, http.MethodGet, base+"/v1/tenant/me/profile", token, "", nil)
	if statusCode != http.StatusOK {
		return nil, statusCode, body
	}
	var profile accessv1.TenantPersonalProfileDTO
	if err := protojson.Unmarshal(body, &profile); err != nil {
		t.Fatal(err)
	}
	return &profile, statusCode, body
}

func enterprise181UpdateAvatar(t *testing.T, base, token, key string, version uint64, avatar string) (*accessv1.TenantPersonalProfileDTO, int, []byte) {
	t.Helper()
	payload, err := protojson.Marshal(&accessv1.UpdateMyPersonalAvatarRequest{Version: version, AvatarAssetRef: avatar})
	if err != nil {
		t.Fatal(err)
	}
	statusCode, body := enterprise181HTTP(t, http.MethodPatch, base+"/v1/tenant/me/avatar", token, key, payload)
	if statusCode != http.StatusOK {
		return nil, statusCode, body
	}
	var profile accessv1.TenantPersonalProfileDTO
	if err := protojson.Unmarshal(body, &profile); err != nil {
		t.Fatal(err)
	}
	return &profile, statusCode, body
}

func TestEnterprise181PersonalProfileIsSelfOnlyTenantScopedAndAvatarControlled(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB, tenantDenied := "e181-a-"+stamp, "e181-b-"+stamp, "e181-denied-"+stamp
	sharedUser, otherUser := "e181-user-"+stamp, "e181-other-"+stamp
	tokenA, tokenB, tokenDenied := "e181-token-a-"+stamp, "e181-token-b-"+stamp, "e181-token-denied-"+stamp
	now := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Millisecond)
	exec := func(query string, args ...any) {
		t.Helper()
		if err := db.Exec(query, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, tenant := range []string{tenantA, tenantB, tenantDenied} {
		exec("INSERT INTO biz_tenants(id,name,status,version,created_at,updated_at) VALUES(?,?,?,1,NOW(3),NOW(3))", tenant, "Tenant "+tenant, "active")
	}
	username := "e181-" + stamp[len(stamp)-8:]
	exec("INSERT INTO biz_users(id,username,email,status,created_at) VALUES(?,?,?,?,?)", sharedUser, username, "global-"+stamp+"@example.invalid", "active", now)
	exec("INSERT INTO biz_users(id,email,status,created_at) VALUES(?,?,?,NOW(3))", otherUser, "other-"+stamp+"@example.invalid", "active")
	for _, member := range []struct {
		tenant, name, email, phone string
	}{
		{tenantA, "Alice A", "alice.a-" + stamp + "@example.invalid", "+491701234567"},
		{tenantB, "Alice B", "alice.b-" + stamp + "@example.invalid", "+8613800138000"},
		{tenantDenied, "Alice Denied", "alice.denied-" + stamp + "@example.invalid", ""},
	} {
		exec("INSERT INTO biz_memberships(tenant_id,user_id,status,name,email,phone,avatar_asset_ref,version,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,NOW(3),NOW(3))", member.tenant, sharedUser, "active", member.name, member.email, member.phone, "avatar:coffee-blue", 1)
		roleID := member.tenant + ":member"
		exec("INSERT INTO biz_roles(id,tenant_id,name,status,version) VALUES(?,?,?,?,1)", roleID, member.tenant, "member-"+member.tenant, "active")
		exec("INSERT INTO biz_member_roles(tenant_id,user_id,role_id) VALUES(?,?,?)", member.tenant, sharedUser, roleID)
	}
	exec("INSERT INTO biz_memberships(tenant_id,user_id,status,name,version,created_at,updated_at) VALUES(?,?,?,?,1,NOW(3),NOW(3))", tenantA, otherUser, "active", "Other User")
	for _, token := range []struct{ raw, tenant string }{{tokenA, tenantA}, {tokenB, tenantB}, {tokenDenied, tenantDenied}} {
		exec("INSERT INTO biz_api_tokens(token_hash,tenant_id,user_id,disabled,created_at) VALUES(?,?,?,?,NOW(3))", accesspersistence.TokenHash(token.raw), token.tenant, sharedUser, false)
	}
	ce05LegacyFixtureGrants(t, db, tenantA, []string{"access-management"})
	ce05LegacyFixtureGrants(t, db, tenantB, []string{"access-management"})

	profileA, statusCode, bodyA := enterprise181Profile(t, base, tokenA)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant A profile status=%d body=%s", statusCode, bodyA)
	}
	profileB, statusCode, bodyB := enterprise181Profile(t, base, tokenB)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant B profile status=%d body=%s", statusCode, bodyB)
	}
	if profileA.GetUserId() != sharedUser || profileB.GetUserId() != sharedUser || profileA.GetTenantId() != tenantA || profileB.GetTenantId() != tenantB {
		t.Fatalf("self scope mismatch A=%+v B=%+v", profileA, profileB)
	}
	if profileA.GetName() != "Alice A" || profileB.GetName() != "Alice B" || profileA.GetUsername() != username {
		t.Fatalf("tenant profile facts crossed scope A=%+v B=%+v", profileA, profileB)
	}
	if profileA.GetRegisteredAt() == "" || profileA.GetJoinedAt() == "" || len(profileA.GetRoles()) != 1 {
		t.Fatalf("personal profile missing account/tenant facts: %+v", profileA)
	}
	for label, raw := range map[string]string{"a-email": "alice.a-" + stamp + "@example.invalid", "a-phone": "+491701234567", "b-email": "alice.b-" + stamp + "@example.invalid", "b-phone": "+8613800138000"} {
		if bytes.Contains(bodyA, []byte(raw)) || bytes.Contains(bodyB, []byte(raw)) {
			t.Fatalf("%s leaked raw contact in personal profile response", label)
		}
	}
	if profileA.GetEmail() == "" || !strings.Contains(profileA.GetEmail(), "***") || profileA.GetPhone() == "" || !strings.HasPrefix(profileA.GetPhone(), "***") {
		t.Fatalf("tenant A contacts are not masked: email=%q phone=%q", profileA.GetEmail(), profileA.GetPhone())
	}
	if profileB.GetEmail() == profileA.GetEmail() || profileB.GetPhone() == profileA.GetPhone() {
		t.Fatalf("A/B tenant contacts did not remain tenant-specific: A=%+v B=%+v", profileA, profileB)
	}

	statusCode, injectedBody := enterprise181HTTP(t, http.MethodGet, base+"/v1/tenant/me/profile?user_id="+otherUser+"&tenant_id="+tenantB, tokenA, "", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("query injection should not become identity input: status=%d body=%s", statusCode, injectedBody)
	}
	var injected accessv1.TenantPersonalProfileDTO
	if err := protojson.Unmarshal(injectedBody, &injected); err != nil {
		t.Fatal(err)
	}
	if injected.GetUserId() != sharedUser || injected.GetTenantId() != tenantA {
		t.Fatalf("query parameters changed trusted self scope: %+v", injected)
	}

	optionsStatus, optionsBody := enterprise181HTTP(t, http.MethodGet, base+"/v1/tenant/me/avatar-options", tokenA, "", nil)
	if optionsStatus != http.StatusOK {
		t.Fatalf("avatar options status=%d body=%s", optionsStatus, optionsBody)
	}
	var options accessv1.ListMyPersonalAvatarOptionsResponse
	if err := protojson.Unmarshal(optionsBody, &options); err != nil {
		t.Fatal(err)
	}
	if len(options.GetOptions()) != 4 {
		t.Fatalf("avatar options=%d want=4", len(options.GetOptions()))
	}
	for _, option := range options.GetOptions() {
		if strings.Contains(option.GetAssetRef(), "://") || strings.HasPrefix(option.GetAssetRef(), "data:") || strings.HasPrefix(option.GetAssetRef(), "javascript:") {
			t.Fatalf("avatar option is not a controlled asset key: %+v", option)
		}
	}

	for index, invalid := range []string{"https://evil.invalid/avatar.png", "javascript:alert(1)", "data:image/svg+xml;base64,PHN2Zy8+"} {
		_, statusCode, _ = enterprise181UpdateAvatar(t, base, tokenA, fmt.Sprintf("invalid-%d:%s", index, stamp), profileA.GetVersion(), invalid)
		if statusCode == http.StatusOK {
			t.Fatalf("invalid avatar accepted: %q", invalid)
		}
	}
	profileA, statusCode, bodyA = enterprise181UpdateAvatar(t, base, tokenA, "valid:"+stamp, profileA.GetVersion(), "avatar:coffee-violet")
	if statusCode != http.StatusOK || profileA.GetAvatarAssetRef() != "avatar:coffee-violet" || profileA.GetVersion() != 2 {
		t.Fatalf("valid avatar update status=%d body=%s profile=%+v", statusCode, bodyA, profileA)
	}
	readbackA, _, _ := enterprise181Profile(t, base, tokenA)
	readbackB, _, _ := enterprise181Profile(t, base, tokenB)
	if readbackA.GetAvatarAssetRef() != "avatar:coffee-violet" || readbackA.GetVersion() != 2 {
		t.Fatalf("tenant A avatar readback=%+v", readbackA)
	}
	if readbackB.GetAvatarAssetRef() != "avatar:coffee-blue" || readbackB.GetVersion() != 1 {
		t.Fatalf("tenant A avatar leaked into tenant B: %+v", readbackB)
	}

	_, staleStatus, _ := enterprise181UpdateAvatar(t, base, tokenA, "stale:"+stamp, 1, "avatar:coffee-amber")
	if staleStatus != http.StatusConflict {
		t.Fatalf("stale avatar CAS status=%d want=%d", staleStatus, http.StatusConflict)
	}
	afterStale, _, _ := enterprise181Profile(t, base, tokenA)
	if afterStale.GetAvatarAssetRef() != "avatar:coffee-violet" || afterStale.GetVersion() != 2 {
		t.Fatalf("stale avatar write changed authority: %+v", afterStale)
	}

	payload, err := protojson.Marshal(&accessv1.UpdateMyPersonalAvatarRequest{Version: afterStale.GetVersion(), AvatarAssetRef: "avatar:coffee-emerald"})
	if err != nil {
		t.Fatal(err)
	}
	missingKeyStatus, _ := enterprise181HTTP(t, http.MethodPatch, base+"/v1/tenant/me/avatar", tokenA, "", payload)
	if missingKeyStatus != http.StatusBadRequest {
		t.Fatalf("missing idempotency status=%d want=%d", missingKeyStatus, http.StatusBadRequest)
	}
	key := "replay:" + stamp
	firstStatus, _ := enterprise181HTTP(t, http.MethodPatch, base+"/v1/tenant/me/avatar", tokenA, key, payload)
	if firstStatus != http.StatusOK {
		t.Fatalf("first idempotent avatar write status=%d", firstStatus)
	}
	replayStatus, _ := enterprise181HTTP(t, http.MethodPatch, base+"/v1/tenant/me/avatar", tokenA, key, payload)
	if replayStatus == http.StatusOK {
		t.Fatal("completed idempotency key replay was accepted as a second write")
	}
	afterReplay, _, _ := enterprise181Profile(t, base, tokenA)
	if afterReplay.GetVersion() != 3 || afterReplay.GetAvatarAssetRef() != "avatar:coffee-emerald" {
		t.Fatalf("idempotency replay changed state more than once: %+v", afterReplay)
	}

	for label, tc := range map[string]struct {
		token string
		want  int
	}{
		"unauthenticated": {"", http.StatusUnauthorized},
		"module-not-entitled": {tokenDenied, http.StatusForbidden},
	} {
		_, got, body := enterprise181Profile(t, base, tc.token)
		if got != tc.want {
			t.Fatalf("%s profile status=%d want=%d body=%s", label, got, tc.want, body)
		}
	}

	// Unknown user/tenant fields are not part of the write contract and must not
	// become an alternate identity source.
	injectedWrite, _ := json.Marshal(map[string]any{
		"version": afterReplay.GetVersion(),
		"avatarAssetRef": "avatar:coffee-amber",
		"userId": otherUser,
		"tenantId": tenantB,
	})
	injectedStatus, _ := enterprise181HTTP(t, http.MethodPatch, base+"/v1/tenant/me/avatar", tokenA, "inject:"+stamp, injectedWrite)
	if injectedStatus == http.StatusOK {
		t.Fatal("avatar write accepted user_id/tenant_id injection")
	}
	finalA, _, _ := enterprise181Profile(t, base, tokenA)
	if finalA.GetUserId() != sharedUser || finalA.GetTenantId() != tenantA || finalA.GetAvatarAssetRef() != "avatar:coffee-emerald" {
		t.Fatalf("identity injection changed authoritative self profile: %+v", finalA)
	}
}
