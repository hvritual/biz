package bizruntime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

type enterprise172VerificationSpy struct {
	queued    []domain.SecurityNotificationRequest
	delivered []string
}

func (spy *enterprise172VerificationSpy) SendVerificationCode(context.Context, domain.VerificationChallengeRequest) (domain.VerificationChallengeReceipt, domain.NotificationDeliveryReceipt, error) {
	return domain.VerificationChallengeReceipt{}, domain.NotificationDeliveryReceipt{}, errors.New("not used")
}

func (spy *enterprise172VerificationSpy) VerifyCode(context.Context, domain.VerifyChallengeRequest) (domain.OneTimeAuthorization, error) {
	return domain.OneTimeAuthorization{}, errors.New("not used")
}

func (spy *enterprise172VerificationSpy) ConsumeAuthorization(context.Context, domain.ConsumeAuthorizationRequest) (domain.AuthorizationConsumptionReceipt, error) {
	return domain.AuthorizationConsumptionReceipt{}, errors.New("not used")
}

func (spy *enterprise172VerificationSpy) QueueSecurityNotification(_ context.Context, request domain.SecurityNotificationRequest) (domain.NotificationDeliveryReceipt, error) {
	spy.queued = append(spy.queued, request)
	return domain.NotificationDeliveryReceipt{EventID: "lock-event", State: domain.NotificationStatePending}, nil
}

func (spy *enterprise172VerificationSpy) DeliverSecurityNotification(_ context.Context, eventID string) (domain.NotificationDeliveryReceipt, error) {
	spy.delivered = append(spy.delivered, eventID)
	now := time.Now().UTC()
	return domain.NotificationDeliveryReceipt{EventID: eventID, State: domain.NotificationStateDelivered, DeliveredAt: &now}, nil
}

func TestEnterprise172LoginLockQueuesSecurityNotificationWithoutLeakingCredential(t *testing.T) {
	spy := &enterprise172VerificationSpy{}
	idp := &runtimeFirstPartyIdP{verification: spy}
	blockedUntil := time.Now().UTC().Add(15 * time.Minute)
	idp.maybeNotifyLoginLock(context.Background(), accesspersistence.LoginIdentifierResolution{
		Identity:       accesspersistence.LocalUserIdentity{UserID: "user-172", Email: "user172@example.invalid"},
		Kind:           accesspersistence.LoginIdentifierEmail,
		Normalized:     "user172@example.invalid",
		OTPChannel:     domain.SecurityNotificationEmail,
		OTPDestination: "user172@example.invalid",
	}, "request-172", &blockedUntil)

	if len(spy.queued) != 1 || len(spy.delivered) != 1 {
		t.Fatalf("lock notification queue/delivery evidence missing: queued=%d delivered=%d", len(spy.queued), len(spy.delivered))
	}
	request := spy.queued[0]
	if request.Kind != domain.SecurityNotificationLoginLock || request.Purpose != domain.VerificationPurposeLogin {
		t.Fatalf("unexpected notification kind/purpose: %+v", request)
	}
	if request.UserID != "user-172" || request.Destination != "user172@example.invalid" || request.Secret != "" {
		t.Fatalf("unexpected lock notification payload: %+v", request)
	}
	if request.FlowID != "request-172" || request.ExpiresAt.IsZero() {
		t.Fatalf("lock notification lacks flow/expiry evidence: %+v", request)
	}
}
