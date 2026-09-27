//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/bizruntime"
	"github.com/hvritual/biz/modules/deviceops"
	"gorm.io/gorm"
	"yunka.io/framework/platform"
	"yunka.io/gateway/authz"
	"yunka.io/pkg/logExt"
)

type enterprise187Environment struct {
	started       *bizruntime.Started
	db            *gorm.DB
	store         *accesspersistence.Store
	legacyToken   string
	allowedSubject string
	allowedKey    string
	allowedSecret []byte
	rotatedKey    string
	rotatedSecret []byte
	deniedKey     string
	deniedSecret  []byte
	tenantKey     string
	tenantSecret  []byte
	tenantA       string
	tenantB       string
}

func startEnterprise187Runtime(t *testing.T) enterprise187Environment {
	t.Helper()
	db := openDB(t)
	stamp := fmt.Sprint(time.Now().UnixNano())
	cfg := deviceops.DefaultConfig()
	cfg.HTTPListenAddress = "127.0.0.1:0"
	cfg.GRPCListenAddress = "127.0.0.1:0"
	cfg.AutoMigrate = true
	provider, err := platform.New(platform.Options{
		Config: bizruntime.ConfigProvider{DeviceOps: cfg},
		Logger: logExt.NewBaseLogger(),
		Databases: map[string]platform.DatabaseFactory{
			"primary": platform.DatabaseFactoryFunc(func(context.Context, string) (platform.DatabaseResource, error) {
				return platform.BorrowedDatabase(db), nil
			}),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	allowedKey := "e187-key-old-" + stamp
	rotatedKey := "e187-key-new-" + stamp
	deniedKey := "e187-key-denied-" + stamp
	tenantKey := "e187-key-tenant-" + stamp
	allowedSecret := []byte("enterprise187-old-service-secret-" + stamp)
	rotatedSecret := []byte("enterprise187-new-service-secret-" + stamp)
	deniedSecret := []byte("enterprise187-denied-secret-----" + stamp)
	tenantSecret := []byte("enterprise187-tenant-secret-----" + stamp)
	allowedSubject := "e187-service-allowed-" + stamp
	deniedSubject := "e187-service-denied-" + stamp
	tenantSubject := "e187-service-tenant-" + stamp
	legacyToken := "e187-legacy-" + stamp
	tenantA := "e187-tenant-a-" + stamp
	tenantB := "e187-tenant-b-" + stamp

	options := bizruntime.Options{
		DeviceOps: cfg,
		PlatformBootstrap: bizruntime.PlatformBootstrap{
			Subject: allowedSubject,
			Token: legacyToken,
			Permissions: []authz.PermissionKey{"platform.plan.read", "platform.module.read"},
		},
		ServiceAPIAuth: bizruntime.ServiceAPIAuthConfig{
			ClockSkew: 30 * time.Second,
			Credentials: []bizruntime.ServiceAPICredentialConfig{
				{KeyID: allowedKey, Subject: allowedSubject, Secret: allowedSecret, Operations: []string{"commercial.plan.discover"}},
				{KeyID: rotatedKey, Subject: allowedSubject, Secret: rotatedSecret, Operations: []string{"commercial.plan.discover"}},
				{KeyID: deniedKey, Subject: deniedSubject, Secret: deniedSecret, Operations: []string{"commercial.plan.discover"}},
				{KeyID: tenantKey, Subject: tenantSubject, TenantID: tenantA, Secret: tenantSecret, Operations: []string{"commercial.provisioning.task.list"}},
			},
		},
	}
	started, err := bizruntime.BootstrapWithOptions(context.Background(), provider, options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = started.App.Shutdown(ctx)
	})
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.BootstrapPlatform(ctx, accesspersistence.PlatformBootstrap{
		Subject: deniedSubject,
		Token: "e187-denied-bootstrap-" + stamp,
		Permissions: []authz.PermissionKey{"platform.module.read"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.BootstrapPlatform(ctx, accesspersistence.PlatformBootstrap{
		Subject: tenantSubject,
		Token: "e187-tenant-bootstrap-" + stamp,
		Permissions: []authz.PermissionKey{"platform.provisioning.read", "platform.tenant.read"},
	}); err != nil {
		t.Fatal(err)
	}
	for _, tenant := range []string{tenantA, tenantB} {
		if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
			TenantID: tenant, TenantName: tenant, UserID: tenant + "-owner",
			Email: tenant + "@example.invalid", Token: "e187-tenant-token-" + tenant,
		}, nil); err != nil {
			t.Fatal(err)
		}
	}
	return enterprise187Environment{
		started: started, db: db, store: store, legacyToken: legacyToken, allowedSubject: allowedSubject,
		allowedKey: allowedKey, allowedSecret: allowedSecret,
		rotatedKey: rotatedKey, rotatedSecret: rotatedSecret,
		deniedKey: deniedKey, deniedSecret: deniedSecret,
		tenantKey: tenantKey, tenantSecret: tenantSecret,
		tenantA: tenantA, tenantB: tenantB,
	}
}

func TestEnterprise187ServiceAPIHMACReplayAndScope(t *testing.T) {
	e := startEnterprise187Runtime(t)
	base := "http://" + e.started.HTTPAddress()
	now := time.Now().UTC().Truncate(time.Second)

	t.Run("valid-signed-service-request-enters-existing-operation-runtime", func(t *testing.T) {
		status, body, headers := enterprise187Do(t, base, http.MethodGet, "/v1/platform/plans", "/v1/platform/plans", nil, nil, e.allowedKey, e.allowedSecret, now, "nonce-valid-0000000001")
		if status != http.StatusOK {
			t.Fatalf("status=%d body=%s failure=%s", status, body, headers.Get("X-Biz-Auth-Failure"))
		}
	})

	t.Run("raw-platform-bearer-cannot-downgrade-a-signature-required-operation", func(t *testing.T) {
		request, err := http.NewRequest(http.MethodGet, base+"/v1/platform/plans", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+e.legacyToken)
		response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		if response.StatusCode != http.StatusUnauthorized || response.Header.Get("X-Biz-Auth-Failure") != "signature_required" {
			t.Fatalf("downgrade status=%d body=%s failure=%s", response.StatusCode, body, response.Header.Get("X-Biz-Auth-Failure"))
		}
		if response.Header.Get("X-Biz-Trace-Id") == "" {
			t.Fatal("service authentication denial has no correlation trace")
		}
	})

	t.Run("legacy-api-key-remains-valid-on-unconfigured-operation", func(t *testing.T) {
		request, err := http.NewRequest(http.MethodGet, base+"/v1/platform/modules", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+e.legacyToken)
		response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(response.Body)
			t.Fatalf("unconfigured legacy API operation regressed: %d %s", response.StatusCode, body)
		}
	})

	t.Run("query-token-is-never-authority", func(t *testing.T) {
		request, err := http.NewRequest(http.MethodGet, base+"/v1/platform/plans?access_token="+url.QueryEscape(e.legacyToken), nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("query token status=%d", response.StatusCode)
		}
	})

	t.Run("signature-does-not-create-operation-permission", func(t *testing.T) {
		status, body, _ := enterprise187Do(t, base, http.MethodGet, "/v1/platform/plans", "/v1/platform/plans", nil, nil, e.deniedKey, e.deniedSecret, now, "nonce-denied-00000001")
		if status != http.StatusForbidden {
			t.Fatalf("signed underprivileged principal status=%d body=%s", status, body)
		}
	})

	t.Run("tenant-bound-key-cannot-cross-tenant-path-authority", func(t *testing.T) {
		pathA := "/v1/platform/tenants/" + e.tenantA + "/provisioning/tasks"
		status, body, _ := enterprise187Do(t, base, http.MethodGet, pathA, pathA, nil, nil, e.tenantKey, e.tenantSecret, now, "nonce-tenant-a-0000001")
		if status != http.StatusOK {
			t.Fatalf("tenant A signed request status=%d body=%s", status, body)
		}
		pathB := "/v1/platform/tenants/" + e.tenantB + "/provisioning/tasks"
		status, _, _ = enterprise187Do(t, base, http.MethodGet, pathB, pathB, nil, nil, e.tenantKey, e.tenantSecret, now, "nonce-tenant-b-0000001")
		if status != http.StatusUnauthorized {
			t.Fatalf("cross-tenant signed request status=%d", status)
		}
	})

	t.Run("same-nonce-concurrently-allows-exactly-one-request", func(t *testing.T) {
		const workers = 8
		statuses := make(chan int, workers)
		var wg sync.WaitGroup
		for index := 0; index < workers; index++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				status, _, _ := enterprise187Do(t, base, http.MethodGet, "/v1/platform/plans", "/v1/platform/plans", nil, nil, e.rotatedKey, e.rotatedSecret, now, "nonce-race-000000000001")
				statuses <- status
			}()
		}
		wg.Wait()
		close(statuses)
		okCount, replayCount := 0, 0
		for status := range statuses {
			switch status {
			case http.StatusOK:
				okCount++
			case http.StatusUnauthorized:
				replayCount++
			default:
				t.Fatalf("unexpected concurrent status=%d", status)
			}
		}
		if okCount != 1 || replayCount != workers-1 {
			t.Fatalf("concurrent nonce results success=%d replay=%d", okCount, replayCount)
		}
	})

	t.Run("method-path-query-and-body-tamper-are-rejected", func(t *testing.T) {
		tests := []struct {
			name, method, requestPath, signingPath string
			requestBody, signingBody               []byte
		}{
			{"method", http.MethodPost, "/v1/platform/plans", "/v1/platform/plans", nil, nil},
			{"path", http.MethodGet, "/v1/platform/plans/extra", "/v1/platform/plans", nil, nil},
			{"query", http.MethodGet, "/v1/platform/plans?page_size=1", "/v1/platform/plans", nil, nil},
			{"body", http.MethodGet, "/v1/platform/plans", "/v1/platform/plans", []byte("{}"), []byte{}},
		}
		for index, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				status, _, _ := enterprise187Do(t, base, test.method, test.requestPath, test.signingPath, test.requestBody, test.signingBody, e.rotatedKey, e.rotatedSecret, now, fmt.Sprintf("nonce-tamper-%02d-000001", index))
				if status != http.StatusUnauthorized {
					t.Fatalf("tamper %s status=%d", test.name, status)
				}
			})
		}
	})

	t.Run("past-and-future-timestamps-are-rejected", func(t *testing.T) {
		for index, timestamp := range []time.Time{now.Add(-2 * time.Minute), now.Add(2 * time.Minute)} {
			status, _, headers := enterprise187Do(t, base, http.MethodGet, "/v1/platform/plans", "/v1/platform/plans", nil, nil, e.rotatedKey, e.rotatedSecret, timestamp, fmt.Sprintf("nonce-time-%02d-00000001", index))
			if status != http.StatusUnauthorized || headers.Get("X-Biz-Auth-Failure") != "timestamp_invalid" {
				t.Fatalf("timestamp status=%d failure=%s", status, headers.Get("X-Biz-Auth-Failure"))
			}
		}
	})

	t.Run("nonce-store-failure-is-fail-closed", func(t *testing.T) {
		name := "enterprise187:nonce-failure"
		if err := e.db.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
			table := tx.Statement.Table
			if table == "" && tx.Statement.Schema != nil {
				table = tx.Statement.Schema.Table
			}
			if table == "biz_service_api_nonces" {
				tx.AddError(fmt.Errorf("enterprise187 forced nonce storage failure"))
			}
		}); err != nil {
			t.Fatal(err)
		}
		status, body, headers := enterprise187Do(t, base, http.MethodGet, "/v1/platform/plans", "/v1/platform/plans", nil, nil, e.rotatedKey, e.rotatedSecret, now, "nonce-storage-fail-00001")
		_ = e.db.Callback().Create().Remove(name)
		if status != http.StatusServiceUnavailable || headers.Get("X-Biz-Auth-Failure") != "authentication_unavailable" {
			t.Fatalf("nonce storage failure status=%d body=%s failure=%s", status, body, headers.Get("X-Biz-Auth-Failure"))
		}
	})

	t.Run("rotation-window-and-revocation-are-immediate-and-restart-safe", func(t *testing.T) {
		oldStatus, _, _ := enterprise187Do(t, base, http.MethodGet, "/v1/platform/plans", "/v1/platform/plans", nil, nil, e.allowedKey, e.allowedSecret, now, "nonce-old-before-000001")
		newStatus, _, _ := enterprise187Do(t, base, http.MethodGet, "/v1/platform/plans", "/v1/platform/plans", nil, nil, e.rotatedKey, e.rotatedSecret, now, "nonce-new-before-000001")
		if oldStatus != http.StatusOK || newStatus != http.StatusOK {
			t.Fatalf("rotation overlap old=%d new=%d", oldStatus, newStatus)
		}
		if err := e.store.DisableServiceAPICredential(context.Background(), e.allowedKey); err != nil {
			t.Fatal(err)
		}
		oldStatus, body, _ := enterprise187Do(t, base, http.MethodGet, "/v1/platform/plans", "/v1/platform/plans", nil, nil, e.allowedKey, e.allowedSecret, now, "nonce-old-after-0000001")
		if oldStatus != http.StatusUnauthorized {
			t.Fatalf("revoked key status=%d body=%s", oldStatus, body)
		}
		if err := e.store.BootstrapServiceAPICredentials(context.Background(), []accesspersistence.ServiceAPICredentialBootstrap{{
			KeyID: e.allowedKey, Subject: e.allowedSubject,
			SecretDigest: accesspersistence.ServiceAPISecretDigest(e.allowedSecret), Operations: []string{"commercial.plan.discover"},
		}}); err != nil {
			t.Fatal(err)
		}
		var disabled bool
		if err := e.db.Table("biz_service_api_credentials").Select("disabled").Where("key_id = ?", e.allowedKey).Scan(&disabled).Error; err != nil {
			t.Fatal(err)
		}
		if !disabled {
			t.Fatal("bootstrap resurrected a revoked service key")
		}
		newStatus, body, _ = enterprise187Do(t, base, http.MethodGet, "/v1/platform/plans", "/v1/platform/plans", nil, nil, e.rotatedKey, e.rotatedSecret, now, "nonce-new-after-0000001")
		if newStatus != http.StatusOK {
			t.Fatalf("rotated key status=%d body=%s", newStatus, body)
		}
	})

	t.Run("failure-output-does-not-leak-key-or-secret", func(t *testing.T) {
		status, body, headers := enterprise187Do(t, base, http.MethodGet, "/v1/platform/plans", "/v1/platform/plans", nil, nil, e.rotatedKey, []byte(strings.Repeat("x", 40)), now, "nonce-secret-probe-00001")
		if status != http.StatusUnauthorized {
			t.Fatalf("bad secret status=%d", status)
		}
		material := string(body) + headers.Get("X-Biz-Auth-Failure") + headers.Get("X-Biz-Trace-Id")
		if strings.Contains(material, e.rotatedKey) || strings.Contains(material, string(e.rotatedSecret)) {
			t.Fatal("authentication denial leaked service credential material")
		}
	})
}

func enterprise187Do(t *testing.T, base, method, requestPath, signingPath string, requestBody, signingBody []byte, keyID string, secret []byte, timestamp time.Time, nonce string) (int, []byte, http.Header) {
	t.Helper()
	if signingPath == "" {
		signingPath = requestPath
	}
	if signingBody == nil {
		signingBody = requestBody
	}
	request, err := http.NewRequest(method, base+requestPath, bytes.NewReader(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	signURL, err := url.Parse(signingPath)
	if err != nil {
		t.Fatal(err)
	}
	query, err := url.ParseQuery(signURL.RawQuery)
	if err != nil {
		t.Fatal(err)
	}
	for key := range query {
		sort.Strings(query[key])
	}
	bodyHash := sha256.Sum256(signingBody)
	timestampRaw := strconv.FormatInt(timestamp.Unix(), 10)
	canonical := strings.Join([]string{
		"BIZ-HMAC-SHA256-V1",
		strings.ToUpper(method),
		signURL.EscapedPath(),
		query.Encode(),
		timestampRaw,
		nonce,
		hex.EncodeToString(bodyHash[:]),
	}, "\n")
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(canonical))
	request.Header.Set("X-Biz-Service-Key-Id", keyID)
	request.Header.Set("X-Biz-Service-Timestamp", timestampRaw)
	request.Header.Set("X-Biz-Service-Nonce", nonce)
	request.Header.Set("X-Biz-Service-Signature", hex.EncodeToString(mac.Sum(nil)))
	response, err := (&http.Client{Timeout: 8 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, body, response.Header.Clone()
}
