//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"testing"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/bizruntime"
	notificationdomain "github.com/hvritual/biz/internal/notification/domain"
	notificationpersistence "github.com/hvritual/biz/internal/notification/infrastructure/persistence"
	"github.com/hvritual/biz/modules/deviceops"
	"gorm.io/gorm"
	"yunka.io/framework/platform"
	"yunka.io/gateway/authz"
	"yunka.io/pkg/logExt"
)

const (
	enterprise185InAppSampleSize   = 1000
	enterprise185InAppThreshold    = 60 * time.Second
	enterprise185InAppThresholdPPM = 999000
)

func TestEnterprise185InAppLatency1000Within60Seconds(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	stamp := fmt.Sprint(time.Now().UnixNano())
	tenant, user, site := "n185-latency-"+stamp, "n185-latency-owner-"+stamp, "n185-latency-site-"+stamp
	token := "n185-latency-token-" + stamp

	config := deviceops.DefaultConfig()
	config.HTTPListenAddress = "127.0.0.1:0"
	config.GRPCListenAddress = "127.0.0.1:0"
	config.AutoMigrate = true
	config.Bootstrap = deviceops.BootstrapConfig{
		TenantID: tenant, TenantName: tenant, UserID: user,
		Email: user + "@example.invalid", Token: token, SiteID: site, SiteName: site,
	}
	platformProvider, err := platform.New(platform.Options{
		Config: bizruntime.ConfigProvider{DeviceOps: config},
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

	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	started, err := bizruntime.BootstrapWithOptions(runtimeCtx, platformProvider, bizruntime.Options{
		DeviceOps: config,
		NotificationRuntime: bizruntime.NotificationRuntimeOptions{
			PollInterval:         10 * time.Millisecond,
			RoutingLeaseDuration: 5 * time.Second,
		},
	})
	if err != nil {
		runtimeCancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		runtimeCancel()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = started.App.Shutdown(shutdown)
	})

	ce05LegacyFixtureGrants(t, db, tenant, []string{"access-management", "device-operations"})
	store, err := accesspersistence.New(db)
	if err != nil {
		t.Fatal(err)
	}
	permissions := []authz.PermissionKey{
		"tenant.notification.read", "tenant.notification.create", "tenant.notification.update", "tenant.notification.delete",
	}
	if err := store.Bootstrap(ctx, accesspersistence.Bootstrap{
		TenantID: tenant, TenantName: tenant, UserID: user,
		Email: user + "@example.invalid", Token: token,
	}, permissions); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(
		"INSERT IGNORE INTO biz_member_sites (tenant_id,user_id,site_id) VALUES (?,?,?)",
		tenant, user, site,
	).Error; err != nil {
		t.Fatal(err)
	}

	base := "http://" + started.HTTPAddress()
	createInAppConfiguration(t, ctx, base, token, stamp, site, user)

	callbackRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		base+"/callbacks/notification/provider",
		bytes.NewReader([]byte("{}")),
	)
	if err != nil {
		t.Fatal(err)
	}
	callbackRequest.Header.Set("Content-Type", "application/json")
	callbackResponse, err := (&http.Client{Timeout: 5 * time.Second}).Do(callbackRequest)
	if err != nil {
		t.Fatal(err)
	}
	_ = callbackResponse.Body.Close()
	if callbackResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("providerless runtime exposed callback status=%d", callbackResponse.StatusCode)
	}

	eventPrefix := "n185-latency-event-" + stamp + "-"
	publishStarted := time.Now()
	if err := db.Transaction(func(tx *gorm.DB) error {
		publisher, err := notificationpersistence.NewEventPublisher(tx)
		if err != nil {
			return err
		}
		for i := 0; i < enterprise185InAppSampleSize; i++ {
			suffix := fmt.Sprintf("%04d", i)
			event := notificationdomain.BusinessEvent{
				EventID:       eventPrefix + suffix,
				TenantID:      tenant,
				GroupID:       site,
				TypeCode:      "system.announcement",
				Level:         notificationdomain.LevelGeneral,
				TraceID:       "n185-latency-trace-" + stamp + "-" + suffix,
				ReferenceKind: "latency_probe",
				ReferenceID:   "n185-latency-ref-" + stamp + "-" + suffix,
				OccurredAt:    time.Now().UTC(),
			}
			if err := publisher.AppendBusinessEvent(ctx, event); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	committedAt := time.Now()
	publishMillis := committedAt.Sub(publishStarted).Milliseconds()

	observationDeadline := committedAt.Add(75 * time.Second)
	var generated int64
	for {
		if err := db.Table("biz_notification_in_app").
			Where("tenant_id=? AND event_id LIKE ?", tenant, eventPrefix+"%").
			Count(&generated).Error; err != nil {
			t.Fatal(err)
		}
		if generated >= enterprise185InAppSampleSize || !time.Now().Before(observationDeadline) {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	observedAt := time.Now()

	type latencyRow struct {
		EventID          string
		EventCreatedAt   time.Time
		MessageCreatedAt *time.Time
	}
	var rows []latencyRow
	if err := db.Table("biz_notification_events e").
		Select("e.event_id, e.created_at AS event_created_at, m.created_at AS message_created_at").
		Joins("LEFT JOIN biz_notification_in_app m ON m.tenant_id=e.tenant_id AND m.event_id=e.event_id AND m.user_id=?", user).
		Where("e.tenant_id=? AND e.event_id LIKE ?", tenant, eventPrefix+"%").
		Order("e.event_id ASC").
		Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != enterprise185InAppSampleSize {
		t.Fatalf("event sample rows=%d want=%d", len(rows), enterprise185InAppSampleSize)
	}

	latencies := make([]time.Duration, 0, enterprise185InAppSampleSize)
	timely, missing := 0, 0
	for _, row := range rows {
		if row.MessageCreatedAt == nil {
			missing++
			continue
		}
		latency := row.MessageCreatedAt.UTC().Sub(row.EventCreatedAt.UTC())
		if latency < 0 {
			t.Fatalf("negative persisted latency event=%s latency=%s", row.EventID, latency)
		}
		latencies = append(latencies, latency)
		if latency <= enterprise185InAppThreshold {
			timely++
		}
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var duplicateCount int64
	if err := db.Raw(
		"SELECT COUNT(*)-COUNT(DISTINCT event_id) FROM biz_notification_in_app WHERE tenant_id=? AND event_id LIKE ?",
		tenant, eventPrefix+"%",
	).Scan(&duplicateCount).Error; err != nil {
		t.Fatal(err)
	}
	var externalTasks int64
	if err := db.Table("biz_notification_external_tasks").
		Where("tenant_id=? AND event_id LIKE ?", tenant, eventPrefix+"%").
		Count(&externalTasks).Error; err != nil {
		t.Fatal(err)
	}

	ratePPM := 0
	if enterprise185InAppSampleSize > 0 {
		ratePPM = timely * 1000000 / enterprise185InAppSampleSize
	}
	sourceSHA, candidateSHA := enterprise185GitHubSourceIdentity()
	receipt := map[string]any{
		"schema_version":                1,
		"state":                         "PASS",
		"source_sha":                    sourceSHA,
		"candidate_sha":                 candidateSHA,
		"run_id":                        os.Getenv("GITHUB_RUN_ID"),
		"run_attempt":                   os.Getenv("GITHUB_RUN_ATTEMPT"),
		"sample_size":                   enterprise185InAppSampleSize,
		"generated_count":               len(latencies),
		"timely_count":                  timely,
		"timely_rate_ppm":               ratePPM,
		"required_rate_ppm":             enterprise185InAppThresholdPPM,
		"threshold_ms":                  enterprise185InAppThreshold.Milliseconds(),
		"missing_count":                 missing,
		"duplicate_count":               duplicateCount,
		"external_task_count":           externalTasks,
		"publish_transaction_ms":        publishMillis,
		"all_generated_after_commit_ms": observedAt.Sub(committedAt).Milliseconds(),
		"measurement_basis":             "biz_notification_events.created_at_to_biz_notification_in_app.created_at",
		"measurement_is_conservative":   true,
		"observed_at":                   observedAt.UTC().Format(time.RFC3339Nano),
	}
	if len(latencies) > 0 {
		receipt["p50_ms"] = percentileDuration(latencies, 0.50).Milliseconds()
		receipt["p95_ms"] = percentileDuration(latencies, 0.95).Milliseconds()
		receipt["p99_ms"] = percentileDuration(latencies, 0.99).Milliseconds()
		receipt["max_ms"] = latencies[len(latencies)-1].Milliseconds()
	}
	if missing != 0 || generated != enterprise185InAppSampleSize ||
		len(latencies) != enterprise185InAppSampleSize || duplicateCount != 0 || externalTasks != 0 ||
		ratePPM < enterprise185InAppThresholdPPM {
		receipt["state"] = "FAIL"
	}
	rawReceipt, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ENTERPRISE185_IN_APP_LATENCY=%s", rawReceipt)

	if receipt["state"] != "PASS" {
		t.Fatalf(
			"in-app latency failed generated=%d timely=%d rate_ppm=%d missing=%d duplicates=%d external=%d",
			len(latencies), timely, ratePPM, missing, duplicateCount, externalTasks,
		)
	}
}

func createInAppConfiguration(
	t *testing.T,
	ctx context.Context,
	base, token, stamp, site, user string,
) {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"groupId":       site,
		"levels":        []string{"general"},
		"channels":      []string{"in_app"},
		"primaryUserId": user,
		"notes":         "1000-message in-app latency qualification",
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		base+"/v1/tenant/notification/configurations",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Idempotency-Key", "n185-latency-config-"+stamp)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("in-app configuration status=%d", response.StatusCode)
	}
}

func percentileDuration(sorted []time.Duration, quantile float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	index := int(float64(len(sorted)-1) * quantile)
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func enterprise185GitHubSourceIdentity() (string, string) {
	source := os.Getenv("GITHUB_SHA")
	candidate := source
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return source, candidate
	}
	data, err := os.ReadFile(eventPath)
	if err != nil {
		return source, candidate
	}
	var event struct {
		PullRequest struct {
			Head struct {
				SHA string `json:"sha"`
			} `json:"head"`
		} `json:"pull_request"`
	}
	if json.Unmarshal(data, &event) == nil && len(event.PullRequest.Head.SHA) == 40 {
		candidate = event.PullRequest.Head.SHA
	}
	return source, candidate
}
