//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	accessv1 "github.com/hvritual/biz/contracts/gen/access/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"
)

func seedECIR07AuditPermissions(t *testing.T, db *gorm.DB, tenantID string) {
	t.Helper()
	roleID := tenantID + ":member-admin"
	for _, permission := range []string{"tenant.audit.read", "tenant.audit.export"} {
		if err := db.Exec(
			"INSERT INTO biz_permission_grants (tenant_id,role_id,permission,scope) VALUES (?,?,?,?)",
			tenantID, roleID, permission, "all",
		).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func listECIR07Audit(t *testing.T, base, token, query string) (*accessv1.ListTenantAuditRecordsResponse, int, []byte) {
	t.Helper()
	endpoint := base + "/v1/tenant/audit-logs?page=1&page_size=100"
	if query != "" {
		endpoint += "&query=" + url.QueryEscape(query)
	}
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, body
	}
	var result accessv1.ListTenantAuditRecordsResponse
	if err := protojson.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode audit list: %v body=%s", err, body)
	}
	return &result, response.StatusCode, body
}

func getECIR07Audit(t *testing.T, base, token, auditID string) (*accessv1.TenantAuditRecordDTO, int, []byte) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, base+"/v1/tenant/audit-logs/"+url.PathEscape(auditID), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, body
	}
	var result accessv1.TenantAuditRecordDTO
	if err := protojson.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode audit detail: %v body=%s", err, body)
	}
	return &result, response.StatusCode, body
}

func exportECIR07Audit(t *testing.T, base, token, idempotencyKey string) (*accessv1.ExportTenantAuditRecordsResponse, int, []byte) {
	t.Helper()
	payload, err := protojson.Marshal(&accessv1.ExportTenantAuditRecordsRequest{MaxRows: 1000})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, base+"/v1/tenant/audit-logs/exports", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, body
	}
	var result accessv1.ExportTenantAuditRecordsResponse
	if err := protojson.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode audit export: %v body=%s", err, body)
	}
	return &result, response.StatusCode, body
}

func findECIR07Audit(records []*accessv1.TenantAuditRecordDTO, operationID string) *accessv1.TenantAuditRecordDTO {
	for _, record := range records {
		if record.GetOperationId() == operationID {
			return record
		}
	}
	return nil
}

func TestECIR07AuditTrailIsServerProducedTenantScopedAndExportAudited(t *testing.T) {
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	started := startB123Runtime(t, db)
	base := "http://" + started.HTTPAddress()

	tenantA, tenantB := "ec-ri-07-a-"+stamp, "ec-ri-07-b-"+stamp
	adminA, adminB := "ec-ri-07-admin-a-"+stamp, "ec-ri-07-admin-b-"+stamp
	tokenA, tokenB := "ec-ri-07-token-a-"+stamp, "ec-ri-07-token-b-"+stamp
	seedB123TenantAdmin(t, db, tenantA, adminA, adminA+"@example.invalid", tokenA)
	seedB123TenantAdmin(t, db, tenantB, adminB, adminB+"@example.invalid", tokenB)
	seedECIR07AuditPermissions(t, db, tenantA)
	seedECIR07AuditPermissions(t, db, tenantB)

	inviteKey := "ec-ri-07-invite:" + stamp
	_, statusCode, body := inviteB123HTTP(t, base, tokenA, "audit-target-"+stamp+"@example.invalid", inviteKey)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant A invite status=%d body=%s", statusCode, body)
	}

	listA, statusCode, body := listECIR07Audit(t, base, tokenA, "tenant.member.invite")
	if statusCode != http.StatusOK {
		t.Fatalf("tenant A audit list status=%d body=%s", statusCode, body)
	}
	inviteAudit := findECIR07Audit(listA.GetRecords(), "tenant.member.invite")
	if inviteAudit == nil {
		t.Fatalf("server audit missing tenant.member.invite records=%+v", listA.GetRecords())
	}
	if inviteAudit.GetActorSubject() == "" || inviteAudit.GetActorUserId() != adminA {
		t.Fatalf("audit actor not server-derived: %+v", inviteAudit)
	}
	if inviteAudit.GetAuthChannel() != "api-key" || inviteAudit.GetSessionRef() == "" {
		t.Fatalf("audit session evidence missing: %+v", inviteAudit)
	}
	if inviteAudit.GetRequestId() == "" || inviteAudit.GetIdempotencyRef() == "" || inviteAudit.GetRequestDigest() == "" {
		t.Fatalf("audit request/idempotency evidence missing: %+v", inviteAudit)
	}
	if inviteAudit.GetTarget() != "tenant:"+tenantA || inviteAudit.GetReceiptRef() == "" {
		t.Fatalf("audit target/receipt evidence missing: %+v", inviteAudit)
	}
	if inviteAudit.GetResult() != "success" || inviteAudit.GetRisk() != "medium" || inviteAudit.GetOccurredAt() == "" {
		t.Fatalf("audit outcome/risk/timestamp invalid: %+v", inviteAudit)
	}

	listB, statusCode, body := listECIR07Audit(t, base, tokenB, "tenant.member.invite")
	if statusCode != http.StatusOK {
		t.Fatalf("tenant B audit list status=%d body=%s", statusCode, body)
	}
	for _, record := range listB.GetRecords() {
		if record.GetAuditId() == inviteAudit.GetAuditId() || record.GetActorUserId() == adminA || record.GetTarget() == "tenant:"+tenantA {
			t.Fatalf("cross-tenant audit leak A=%+v B=%+v", inviteAudit, record)
		}
	}
	if _, statusCode, _ := getECIR07Audit(t, base, tokenB, inviteAudit.GetAuditId()); statusCode == http.StatusOK {
		t.Fatalf("tenant B read tenant A audit id=%s", inviteAudit.GetAuditId())
	}

	exportKey := "ec-ri-07-export:" + stamp
	exported, statusCode, body := exportECIR07Audit(t, base, tokenA, exportKey)
	if statusCode != http.StatusOK {
		t.Fatalf("tenant A audit export status=%d body=%s", statusCode, body)
	}
	if exported.GetExportId() == "" || exported.GetGeneratedAt() == "" || len(exported.GetRecords()) == 0 {
		t.Fatalf("unexpected audit export=%+v", exported)
	}

	afterExport, statusCode, body := listECIR07Audit(t, base, tokenA, "access.audit.export")
	if statusCode != http.StatusOK {
		t.Fatalf("tenant A post-export audit list status=%d body=%s", statusCode, body)
	}
	exportAudit := findECIR07Audit(afterExport.GetRecords(), "access.audit.export")
	if exportAudit == nil {
		t.Fatalf("export operation did not audit itself records=%+v", afterExport.GetRecords())
	}
	if exportAudit.GetResult() != "success" || exportAudit.GetRisk() != "high" || exportAudit.GetIdempotencyRef() == "" || exportAudit.GetReceiptRef() == "" {
		t.Fatalf("export audit evidence invalid: %+v", exportAudit)
	}

	var attemptCount, outcomeCount int64
	if err := db.Table("biz_audit_events").Where("tenant_id = ? AND operation_id = ? AND event_type = ?", tenantA, "access.audit.export", "attempt").Count(&attemptCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("biz_audit_events").Where("tenant_id = ? AND operation_id = ? AND event_type = ?", tenantA, "access.audit.export", "outcome").Count(&outcomeCount).Error; err != nil {
		t.Fatal(err)
	}
	if attemptCount != 1 || outcomeCount != 1 {
		t.Fatalf("export audit event pair attempt=%d outcome=%d want=1/1", attemptCount, outcomeCount)
	}
}
