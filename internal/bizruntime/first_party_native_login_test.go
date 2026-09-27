package bizruntime

import (
	"testing"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
)

func TestEnterprise172LoginLockQueuesSecurityNotificationWithoutLeakingCredential(t *testing.T) {
	request := domain.SecurityNotificationRequest{
		BusinessEventID: "idp-login-lock/user-172/1",
		Kind:            domain.SecurityNotificationLoginLock,
		Purpose:         domain.VerificationPurposeLogin,
		UserID:          "user-172",
		FlowID:          "request-172",
		Channel:         domain.SecurityNotificationEmail,
		Destination:     "user172@example.invalid",
		ExpiresAt:       time.Now().UTC().Add(time.Hour),
	}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	if request.Secret != "" {
		t.Fatal("login lock notification must remain informational")
	}
}
