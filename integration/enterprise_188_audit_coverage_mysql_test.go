//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"gorm.io/gorm"
)

func TestEnterprise188AuditCoverageContractAndTenantAPI(t *testing.T) {
	enterprise188ValidateCoverageMatrix(t)

	db := openDB(t)
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()
	ctx := context.Background()
	stamp := fmt.Sprint(time.Now().UnixNano())

	tenantA, tenantB := "e188-a-"+stamp, "e188-b-"+stamp
	adminA, adminB := "e188-admin-a-"+stamp, "e188-admin-b-"+stamp
	tokenA, tokenB := "e188-token-a-"+stamp, "e188-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, adminA, adminA+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, adminB, adminB+"@example.invalid", tokenB)
	seedECIR07AuditPermissions(t, db, tenantA)
	seedECIR07AuditPermissions(t, db, tenantB)

	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureFirstPartyIDPSecuritySchema(ctx); err != nil {
		t.Fatal(err)
	}

	userID := "e188-shared-" + stamp
	email := "e188-shared-" + stamp + "@example.invalid"
	oldPassword := "AuditOld9A"
	newPassword := "AuditNew9A"
	if err := db.Exec("INSERT INTO biz_users (id,email,status,created_at) VALUES (?,?,?,NOW(6))", userID, email, "active").Error; err != nil {
		t.Fatal(err)
	}
	for _, tenantID := range []string{tenantA, tenantB} {
		if err := db.Exec("INSERT INTO biz_memberships (tenant_id,user_id,status,email,version,created_at,updated_at) VALUES (?,?,?,?,1,NOW(6),NOW(6))",
			tenantID, userID, "active", email).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetUserPassword(ctx, userID, oldPassword); err != nil {
		t.Fatal(err)
	}

	identity, loginAuditID, err := store.AuthenticateFirstPartyLoginWithAudit(
		ctx, email, oldPassword, "127.0.0.1:18800", accesspersistence.DefaultFirstPartyLoginPolicy(),
	)
	if err != nil || identity.UserID != userID || loginAuditID == 0 {
		t.Fatalf("trusted login identity=%+v audit=%d err=%v", identity, loginAuditID, err)
	}
	for _, tenantID := range []string{tenantA, tenantB} {
		enterprise188RequireAuditOutcome(t, db, tenantID, "identity.login.password", "success", 1)
	}

	wrongRef := "e188-password-wrong-" + stamp
	for range 2 {
		err := store.ChangeOwnPasswordAudited(ctx, tenantA, userID, wrongRef, "wrong-password", newPassword, newPassword)
		if !errors.Is(err, accesspersistence.ErrCurrentPasswordInvalid) {
			t.Fatalf("wrong current password result=%v", err)
		}
	}
	if _, err := store.AuthenticateUserPassword(ctx, email, oldPassword); err != nil {
		t.Fatalf("rejected password change mutated credential: %v", err)
	}
	var wrongOutcomeCount int64
	if err := db.Table("biz_audit_events").
		Where("tenant_id=? AND operation_id=? AND request_id=? AND event_type=? AND outcome=?",
			tenantA, "identity.password.change", wrongRef, "outcome", "failure").
		Count(&wrongOutcomeCount).Error; err != nil {
		t.Fatal(err)
	}
	if wrongOutcomeCount != 1 {
		t.Fatalf("idempotent rejection audit outcomes=%d want=1", wrongOutcomeCount)
	}

	const auditFailureCallback = "enterprise188:force-audit-rollback"
	if err := db.Callback().Create().Before("gorm:create").Register(auditFailureCallback, func(tx *gorm.DB) {
		table := tx.Statement.Table
		if table == "" && tx.Statement.Schema != nil {
			table = tx.Statement.Schema.Table
		}
		if table == "biz_audit_events" {
			tx.AddError(errors.New("enterprise188 forced audit failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	err = store.ChangeOwnPasswordAudited(ctx, tenantA, userID, "e188-password-rollback-"+stamp, oldPassword, newPassword, newPassword)
	if removeErr := db.Callback().Create().Remove(auditFailureCallback); removeErr != nil {
		t.Fatal(removeErr)
	}
	if err == nil {
		t.Fatal("password change survived forced audit failure")
	}
	if _, err := store.AuthenticateUserPassword(ctx, email, oldPassword); err != nil {
		t.Fatalf("audit failure did not roll password transaction back: %v", err)
	}

	successRef := "e188-password-success-" + stamp
	if err := store.ChangeOwnPasswordAudited(ctx, tenantA, userID, successRef, oldPassword, newPassword, newPassword); err != nil {
		t.Fatal(err)
	}
	enterprise188RequireAuditOutcome(t, db, tenantA, "identity.password.change", "success", 1)
	if count := enterprise188AuditOutcomeCount(t, db, tenantB, "identity.password.change", "success"); count != 0 {
		t.Fatalf("tenant-local password action leaked to tenant B audit count=%d", count)
	}

	if _, status, body := inviteB123HTTP(t, base, tokenA, "e188-invite-"+stamp+"@example.invalid", "e188-invite:"+stamp); status != http.StatusOK {
		t.Fatalf("automatic operation audit fixture status=%d body=%s", status, body)
	}
	listA, status, body := listECIR07Audit(t, base, tokenA, "tenant.member.invite")
	if status != http.StatusOK || findECIR07Audit(listA.GetRecords(), "tenant.member.invite") == nil {
		t.Fatalf("operation runtime audit missing status=%d body=%s", status, body)
	}
	loginA, status, body := listECIR07Audit(t, base, tokenA, "identity.login.password")
	if status != http.StatusOK || findECIR07Audit(loginA.GetRecords(), "identity.login.password") == nil {
		t.Fatalf("projected login audit unavailable through existing API status=%d body=%s", status, body)
	}
	loginB, status, body := listECIR07Audit(t, base, tokenB, "identity.login.password")
	if status != http.StatusOK || findECIR07Audit(loginB.GetRecords(), "identity.login.password") == nil {
		t.Fatalf("tenant B projected account audit missing status=%d body=%s", status, body)
	}

	forgedEndpoint := base + "/v1/tenant/audit-logs?page=1&page_size=100&query=" +
		url.QueryEscape("identity.password.change") + "&tenant_id=" + url.QueryEscape(tenantB)
	request, _ := http.NewRequest(http.MethodGet, forgedEndpoint, nil)
	request.Header.Set("Authorization", "Bearer "+tokenA)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusBadRequest {
		t.Fatalf("forged tenant query did not fail closed status=%d", response.StatusCode)
	}
	if count := enterprise188AuditOutcomeCount(t, db, tenantB, "identity.password.change", "success"); count != 0 {
		t.Fatalf("forged tenant query changed password audit authority count=%d", count)
	}

	noAuditUser, noAuditToken := "e188-no-audit-"+stamp, "e188-no-audit-token-"+stamp
	roleID := tenantA + ":e188-no-audit"
	if err := db.Exec("INSERT INTO biz_users (id,email,status,created_at) VALUES (?,?,?,NOW(6))", noAuditUser, noAuditUser+"@example.invalid", "active").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_memberships (tenant_id,user_id,status,version,created_at,updated_at) VALUES (?,?,?,1,NOW(6),NOW(6))", tenantA, noAuditUser, "active").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_roles (id,tenant_id,name,status,version) VALUES (?,?,?,?,1)", roleID, tenantA, "e188-no-audit", "active").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_member_roles (tenant_id,user_id,role_id) VALUES (?,?,?)", tenantA, noAuditUser, roleID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO biz_api_tokens (token_hash,tenant_id,user_id,disabled,created_at) VALUES (?,?,?,?,NOW(6))", accesspersistence.TokenHash(noAuditToken), tenantA, noAuditUser, false).Error; err != nil {
		t.Fatal(err)
	}
	noPermission, _ := http.NewRequest(http.MethodGet, base+"/v1/tenant/audit-logs?page=1&page_size=20", nil)
	noPermission.Header.Set("Authorization", "Bearer "+noAuditToken)
	noPermissionResponse, err := (&http.Client{Timeout: 5 * time.Second}).Do(noPermission)
	if err != nil {
		t.Fatal(err)
	}
	_ = noPermissionResponse.Body.Close()
	if noPermissionResponse.StatusCode != http.StatusForbidden {
		t.Fatalf("audit read without tenant.audit.read status=%d want=403", noPermissionResponse.StatusCode)
	}

	var auditDump string
	query := "SELECT COALESCE(GROUP_CONCAT(CONCAT_WS('|',actor_subject,actor_user_id,operation_id,target,decision_reason,request_digest,receipt_ref,reason) SEPARATOR '\\n'),'') FROM biz_audit_events WHERE tenant_id IN (?,?)"
	if err := db.Raw(query, tenantA, tenantB).Scan(&auditDump).Error; err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{oldPassword, newPassword, email} {
		if strings.Contains(auditDump, secret) {
			t.Fatalf("audit persisted sensitive material %q", secret)
		}
	}
}

func enterprise188RequireAuditOutcome(t *testing.T, db *gorm.DB, tenantID, operationID, outcome string, minimum int64) {
	t.Helper()
	if count := enterprise188AuditOutcomeCount(t, db, tenantID, operationID, outcome); count < minimum {
		t.Fatalf("audit outcome tenant=%s operation=%s outcome=%s count=%d want>=%d", tenantID, operationID, outcome, count, minimum)
	}
}

func enterprise188AuditOutcomeCount(t *testing.T, db *gorm.DB, tenantID, operationID, outcome string) int64 {
	t.Helper()
	var count int64
	if err := db.Table("biz_audit_events").
		Where("tenant_id=? AND operation_id=? AND event_type=? AND outcome=?", tenantID, operationID, "outcome", outcome).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func enterprise188ValidateCoverageMatrix(t *testing.T) {
	t.Helper()
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate enterprise188 test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), ".."))
	payload, err := os.ReadFile(filepath.Join(root, "docs", "enterprise-center", "audit-event-coverage.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var matrix map[string]any
	if err := json.Unmarshal(payload, &matrix); err != nil {
		t.Fatal(err)
	}
	if matrix["schema_version"] != float64(1) {
		t.Fatalf("coverage schema=%v", matrix["schema_version"])
	}
	required := map[string]bool{
		"login": false, "login_lock": false, "privacy_consent": false, "password": false,
		"contact_and_self_deletion": false, "member_lifecycle": false, "member_appeal": false,
		"role_authorization": false, "message_configuration": false, "message_delivery": false,
	}
	plansPayload, err := os.ReadFile(filepath.Join(root, "contracts", "generated", "operation-plans.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plansRoot map[string]any
	if err := json.Unmarshal(plansPayload, &plansRoot); err != nil {
		t.Fatal(err)
	}
	plans := map[string]map[string]any{}
	for _, raw := range plansRoot["operations"].([]any) {
		plan := raw.(map[string]any)
		plans[plan["operationId"].(string)] = plan
	}
	for _, raw := range matrix["events"].([]any) {
		event := raw.(map[string]any)
		class := event["class"].(string)
		if _, ok := required[class]; !ok {
			t.Fatalf("unknown coverage class %q", class)
		}
		required[class] = true
		sourceKind := event["source"].(string)
		var operationIDs []string
		for _, value := range event["operation_ids"].([]any) {
			operationIDs = append(operationIDs, value.(string))
		}
		var evidence []string
		for _, value := range event["evidence"].([]any) {
			evidence = append(evidence, value.(string))
		}
		switch sourceKind {
		case "operation_runtime":
			for _, operationID := range operationIDs {
				plan, ok := plans[operationID]
				if !ok {
					t.Fatalf("operation audit source missing plan %s", operationID)
				}
				security := plan["security"].(map[string]any)
				execution := plan["execution"].(map[string]any)
				if security["tenantRequired"] != true || execution["transaction"] != "local" {
					t.Fatalf("operation audit source not tenant-local %s", operationID)
				}
			}
		case "transactional_projection", "existing_transactional_projection":
			evidenceText := ""
			for _, relative := range evidence {
				source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
				if err != nil {
					t.Fatal(err)
				}
				evidenceText += string(source)
			}
			for _, operationID := range operationIDs {
				if !strings.Contains(evidenceText, operationID) {
					t.Fatalf("coverage operation %s missing from declared transactional evidence", operationID)
				}
			}
		default:
			t.Fatalf("unsupported coverage source %q", sourceKind)
		}
	}
	for class, present := range required {
		if !present {
			t.Fatalf("required audit coverage class missing: %s", class)
		}
	}
	if len(matrix["sensitive_data_forbidden"].([]any)) < 7 || len(matrix["required_audit_fields"].([]any)) < 9 {
		t.Fatal("coverage matrix does not freeze sensitive/field contract")
	}
}
