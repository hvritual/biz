//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

// A delivered OTP notification deliberately erases its ciphertext. The challenge
// still binds purpose, tenant, user, flow, channel and the normalized destination.
func TestEnterprise182DeliveredContactChallengeIsIndependentOfNotificationStorage(t *testing.T) {
	db := openDB(t)
	runtime := startEnterprise182Runtime(t, db)
	ctx := context.Background()
	for _, channel := range []accessdomain.SecurityNotificationChannel{accessdomain.SecurityNotificationEmail, accessdomain.SecurityNotificationSMS} {
		t.Run(string(channel), func(t *testing.T) {
			stamp := fmt.Sprint(time.Now().UnixNano())
			tenantID, userID := "e182-bind-"+stamp, "e182-bind-user-"+stamp
			password := "CoffeePass9A"
			if err := runtime.store.Bootstrap(ctx, accesspersistence.Bootstrap{
				TenantID: tenantID, TenantName: "Binding regression", UserID: userID,
				Email: "account-" + stamp + "@example.invalid", Token: "test-" + stamp,
			}, nil); err != nil {
				t.Fatal(err)
			}
			if err := runtime.store.SetUserPassword(ctx, userID, password); err != nil {
				t.Fatal(err)
			}
			version := enterprise182SetTenantContacts(t, db, runtime.contactProtection, tenantID, userID, "old-"+stamp+"@example.invalid", "+491701234567")
			service, err := accesspersistence.NewTenantSelfSecurityService(db, runtime.contactProtection, runtime.verificationProtection, accessdomain.VerificationPolicy{
				CodeTTL: 5 * time.Minute, AuthorizationTTL: 5 * time.Minute,
				ResendInterval: time.Minute, SendLimitWindow: 24 * time.Hour,
				MaxSendsPerWindow: 5, MaxVerificationTries: 3, CodeDigits: 6,
			})
			if err != nil {
				t.Fatal(err)
			}
			destination, substituted := "new-"+stamp+"@example.invalid", "other-"+stamp+"@example.invalid"
			if channel == accessdomain.SecurityNotificationSMS {
				destination, substituted = "+491709876543", "+491709876544"
			}
			challenge, err := service.RequestContactChange(ctx, userID, tenantID, password, channel, destination, "flow-"+stamp, "event-"+stamp)
			if err != nil {
				t.Fatal(err)
			}
			eventID := enterprise182ChallengeEventID(t, db, challenge.ChallengeID)
			otp := enterprise182DeliverOTP(t, runtime.verificationRepository, eventID)
			var pending struct{ DestinationCiphertext, SecretCiphertext string }
			if err := db.Table("biz_security_notification_outbox").Where("event_id = ?", eventID).First(&pending).Error; err != nil {
				t.Fatal(err)
			}
			if pending.DestinationCiphertext != "" || pending.SecretCiphertext != "" {
				t.Fatal("delivered OTP still retained transport secrets")
			}
			// Even removing the delivered notification cannot destroy OTP authority.
			if err := db.Exec("DELETE FROM biz_security_notification_outbox WHERE event_id = ?", eventID).Error; err != nil {
				t.Fatal(err)
			}
			for _, invalid := range []string{"", "***", substituted} {
				_, err := service.CompleteContactChange(ctx, userID, tenantID, channel, challenge.ChallengeID, challenge.FlowID, otp, invalid, version, "invalid-"+stamp)
				if !errors.Is(err, accessdomain.ErrVerificationInvalid) {
					t.Fatalf("invalid destination returned %v; want verification invalid", err)
				}
			}
			for _, binding := range []struct{ user, tenant, flow string }{
				{userID + "-other", tenantID, challenge.FlowID},
				{userID, tenantID + "-other", challenge.FlowID},
				{userID, tenantID, challenge.FlowID + "-other"},
			} {
				_, err := service.CompleteContactChange(ctx, binding.user, binding.tenant, channel, challenge.ChallengeID, binding.flow, otp, destination, version, "binding-"+stamp)
				if !errors.Is(err, accessdomain.ErrVerificationInvalid) {
					t.Fatalf("changed identity/flow returned %v; want verification invalid", err)
				}
			}
			var state struct{ ConsumedAt *time.Time }
			if err := db.Table("biz_verification_challenges").Where("challenge_id = ?", challenge.ChallengeID).First(&state).Error; err != nil {
				t.Fatal(err)
			}
			if state.ConsumedAt != nil {
				t.Fatal("rejected destination or identity consumed challenge")
			}
			repository, err := accesspersistence.NewTenantMemberRepositoryWithContactProtection(db, runtime.contactProtection)
			if err != nil {
				t.Fatal(err)
			}
			before, err := repository.Get(ctx, tenantID, userID)
			if err != nil || before.Version != version {
				t.Fatalf("rejected completion changed membership version: %d %v", before.Version, err)
			}
			// Equivalent normalization is accepted, not raw-string equality.
			input := strings.ToUpper(destination)
			if channel == accessdomain.SecurityNotificationSMS {
				input = "+49 170 9876543"
			}
			receipt, err := service.CompleteContactChange(ctx, userID, tenantID, channel, challenge.ChallengeID, challenge.FlowID, otp, input, version, "valid-"+stamp)
			if err != nil || receipt.Version != version+1 {
				t.Fatalf("completion after delivery cleanup failed: version=%d err=%v", receipt.Version, err)
			}
			if _, err := service.CompleteContactChange(ctx, userID, tenantID, channel, challenge.ChallengeID, challenge.FlowID, otp, input, version+1, "replay-"+stamp); !errors.Is(err, accessdomain.ErrVerificationConsumed) {
				t.Fatalf("consumed challenge replay returned %v", err)
			}
		})
	}
}
