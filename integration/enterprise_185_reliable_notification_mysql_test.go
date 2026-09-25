//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
)

func TestEnterprise185ReliableSecurityNotificationLeasesRetriesAndRecovery(t *testing.T) {
	db := ce08FreshFixtureDB(t)
	repository := enterprise170Repository(t, db)
	if err := repository.EnsureReliableSecurityNotificationSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	policy := domain.EnterpriseNotificationRetryPolicy()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	stamp := fmt.Sprint(time.Now().UnixNano())
	enqueue := func(suffix string, expiry time.Time) domain.NotificationDeliveryReceipt {
		t.Helper()
		receipt, err := repository.EnqueueSecurityNotification(ctx, domain.SecurityNotificationRequest{
			BusinessEventID: "enterprise185-" + suffix + "-" + stamp,
			Kind: domain.SecurityNotificationMemberLifecycle,
			Purpose: domain.VerificationPurposeMemberLifecycle,
			UserID: "user-" + stamp,
			TenantID: "tenant-" + stamp,
			FlowID: "flow-" + suffix,
			Channel: domain.SecurityNotificationEmail,
			Destination: suffix + "@example.invalid",
			ExpiresAt: expiry,
		})
		if err != nil {
			t.Fatal(err)
		}
		return receipt
	}
	forceDue := func(eventID string) {
		t.Helper()
		if err := db.Exec("UPDATE biz_security_notification_outbox SET next_attempt_at=DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 SECOND) WHERE event_id=?", eventID).Error; err != nil {
			t.Fatal(err)
		}
	}

	t.Run("retry-wait-stale-lease-and-idempotent-completion", func(t *testing.T) {
		pending := enqueue("recovery", time.Now().Add(time.Hour))
		first, _, err := repository.ClaimNextSecurityNotification(ctx, "worker-a", policy)
		if err != nil || first.EventID != pending.EventID || first.Attempt != 1 || first.LeaseToken == 0 {
			t.Fatalf("first claim=%+v err=%v", first, err)
		}
		secondLive, _, err := repository.ClaimNextSecurityNotification(ctx, "worker-b", policy)
		if err != nil || secondLive.EventID != "" {
			t.Fatalf("live lease was stolen: %+v %v", secondLive, err)
		}
		failed, err := repository.FailReliableSecurityNotification(ctx, first, "PROVIDER_TEMPORARY", true, policy)
		if err != nil || failed.State != domain.NotificationStateRetryWait || failed.Attempt != 1 {
			t.Fatalf("retry receipt=%+v err=%v", failed, err)
		}
		forceDue(first.EventID)
		second, _, err := repository.ClaimNextSecurityNotification(ctx, "worker-b", policy)
		if err != nil || second.EventID != first.EventID || second.Attempt != 2 || second.LeaseToken <= first.LeaseToken {
			t.Fatalf("second claim=%+v err=%v", second, err)
		}
		if _, err := repository.CompleteReliableSecurityNotification(ctx, first, "stale-provider-receipt"); !errors.Is(err, domain.ErrNotificationLease) {
			t.Fatalf("stale completion accepted: %v", err)
		}
		if err := db.Exec("UPDATE biz_security_notification_outbox SET lease_until=DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 SECOND) WHERE event_id=?", second.EventID).Error; err != nil {
			t.Fatal(err)
		}
		third, _, err := repository.ClaimNextSecurityNotification(ctx, "worker-c", policy)
		if err != nil || third.EventID != second.EventID || third.Attempt != 3 || third.LeaseToken <= second.LeaseToken {
			t.Fatalf("crash recovery claim=%+v err=%v", third, err)
		}
		delivered, err := repository.CompleteReliableSecurityNotification(ctx, third, "provider:"+third.EventID)
		if err != nil || delivered.State != domain.NotificationStateDelivered || delivered.Attempt != 3 {
			t.Fatalf("delivered=%+v err=%v", delivered, err)
		}
		duplicate, err := repository.CompleteReliableSecurityNotification(ctx, third, "provider:"+third.EventID)
		if err != nil || duplicate.State != domain.NotificationStateDelivered || duplicate.Attempt != delivered.Attempt {
			t.Fatalf("duplicate callback changed outcome: %+v %v", duplicate, err)
		}
		var row struct {
			DestinationCiphertext string
			SecretCiphertext string
			LeaseOwner string
			LeaseUntil *time.Time
		}
		if err := db.Table("biz_security_notification_outbox").Where("event_id=?", third.EventID).Scan(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.DestinationCiphertext != "" || row.SecretCiphertext != "" || row.LeaseOwner != "" || row.LeaseUntil != nil {
			t.Fatalf("delivered row retained sensitive/lease state: %+v", row)
		}
	})

	t.Run("retry-budget-terminalizes-and-clears-material", func(t *testing.T) {
		pending := enqueue("exhaust", time.Now().Add(3*time.Hour))
		var last domain.NotificationDeliveryReceipt
		for attempt := uint32(1); attempt <= policy.MaxAttempts; attempt++ {
			claim, _, err := repository.ClaimNextSecurityNotification(ctx, "worker-exhaust", policy)
			if err != nil || claim.EventID != pending.EventID || claim.Attempt != attempt {
				t.Fatalf("attempt %d claim=%+v err=%v", attempt, claim, err)
			}
			last, err = repository.FailReliableSecurityNotification(ctx, claim, "PROVIDER_TEMPORARY", true, policy)
			if err != nil {
				t.Fatal(err)
			}
			if attempt < policy.MaxAttempts {
				if last.State != domain.NotificationStateRetryWait {
					t.Fatalf("attempt %d state=%s", attempt, last.State)
				}
				forceDue(claim.EventID)
			}
		}
		if last.State != domain.NotificationStateManualReview || last.FailureCode != "DELIVERY_RETRIES_EXHAUSTED" || last.Attempt != policy.MaxAttempts {
			t.Fatalf("terminal receipt=%+v", last)
		}
		var row struct{ DestinationCiphertext, SecretCiphertext string }
		if err := db.Table("biz_security_notification_outbox").Where("event_id=?", pending.EventID).Scan(&row).Error; err != nil {
			t.Fatal(err)
		}
		if row.DestinationCiphertext != "" || row.SecretCiphertext != "" {
			t.Fatal("terminal security notification retained reversible material")
		}
	})

	t.Run("expired-pending-is-terminal-not-invisible", func(t *testing.T) {
		pending := enqueue("expired", time.Now().Add(time.Hour))
		if err := db.Exec("UPDATE biz_security_notification_outbox SET expires_at=DATE_SUB(UTC_TIMESTAMP(6), INTERVAL 1 SECOND) WHERE event_id=?", pending.EventID).Error; err != nil {
			t.Fatal(err)
		}
		claim, receipt, err := repository.ClaimNextSecurityNotification(ctx, "worker-expired", policy)
		if err != nil || claim.EventID != "" || receipt.EventID != pending.EventID ||
			receipt.State != domain.NotificationStateManualReview || receipt.FailureCode != "NOTIFICATION_EXPIRED" {
			t.Fatalf("expired claim=%+v receipt=%+v err=%v", claim, receipt, err)
		}
	})
}
