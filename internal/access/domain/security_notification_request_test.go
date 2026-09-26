package domain

import (
	"errors"
	"testing"
	"time"
)

func TestSecurityNotificationRequestKeepsCredentialMaterialContract(t *testing.T) {
	base := SecurityNotificationRequest{
		BusinessEventID: "notification-material-contract", Purpose: VerificationPurposePasswordRecovery,
		UserID: "user", Channel: SecurityNotificationEmail,
		Destination: "user@example.invalid", ExpiresAt: time.Now().Add(time.Hour),
	}
	for _, kind := range []SecurityNotificationKind{
		SecurityNotificationVerificationCode, SecurityNotificationInitialCredential, SecurityNotificationPasswordReset,
	} {
		t.Run(string(kind), func(t *testing.T) {
			request := base
			request.Kind = kind
			for _, material := range []string{"", " \t\n"} {
				request.Secret = material
				if !errors.Is(request.Validate(), ErrVerificationInvalid) {
					t.Fatal("credential-carrying notification accepted missing material")
				}
			}
			request.Secret = "test-only-credential"
			if err := request.Validate(); err != nil {
				t.Fatalf("existing material contract rejected: %v", err)
			}
		})
	}
	for _, kind := range []SecurityNotificationKind{
		SecurityNotificationLoginLock, SecurityNotificationRecoveryRequest, SecurityNotificationMemberLifecycle,
		SecurityNotificationMemberAppeal, SecurityNotificationContactChanged, SecurityNotificationTenantDeletion,
	} {
		t.Run(string(kind), func(t *testing.T) {
			request := base
			request.Kind = kind
			if err := request.Validate(); err != nil {
				t.Fatalf("existing informational kind rejected: %v", err)
			}
		})
	}
}

func TestPasswordResetCompletionRejectsSecretsAndInvalidContext(t *testing.T) {
	base := SecurityNotificationRequest{
		BusinessEventID: "password-reset-complete/challenge", Kind: SecurityNotificationPasswordResetCompleted,
		Purpose: VerificationPurposePasswordRecovery, UserID: "user", Channel: SecurityNotificationEmail,
		Destination: "user@example.invalid", ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, channel := range []SecurityNotificationChannel{SecurityNotificationEmail, SecurityNotificationSMS} {
		request := base
		request.Channel = channel
		if err := request.Validate(); err != nil {
			t.Fatalf("completion channel rejected: %v", err)
		}
	}
	cases := []struct {
		name   string
		change func(*SecurityNotificationRequest)
	}{
		{"password", func(r *SecurityNotificationRequest) { r.Secret = "NeverTransmit9A" }},
		{"otp", func(r *SecurityNotificationRequest) { r.Secret = "123456" }},
		{"whitespace-material", func(r *SecurityNotificationRequest) { r.Secret = " " }},
		{"wrong-purpose", func(r *SecurityNotificationRequest) { r.Purpose = VerificationPurposeLogin }},
		{"missing-event", func(r *SecurityNotificationRequest) { r.BusinessEventID = "" }},
		{"missing-user", func(r *SecurityNotificationRequest) { r.UserID = "" }},
		{"missing-destination", func(r *SecurityNotificationRequest) { r.Destination = " " }},
		{"missing-expiry", func(r *SecurityNotificationRequest) { r.ExpiresAt = time.Time{} }},
		{"unknown-kind", func(r *SecurityNotificationRequest) { r.Kind = "unknown" }},
		{"unknown-channel", func(r *SecurityNotificationRequest) { r.Channel = "unknown" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := base
			tc.change(&request)
			if !errors.Is(request.Validate(), ErrVerificationInvalid) {
				t.Fatal("invalid completion notification accepted")
			}
		})
	}
}
