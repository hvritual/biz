package domain

import (
	"errors"
	"strings"
	"time"
)

type VerificationPurpose string

const (
	VerificationPurposeLogin            VerificationPurpose = "login"
	VerificationPurposePasswordRecovery VerificationPurpose = "password_recovery"
	VerificationPurposeContactChange    VerificationPurpose = "contact_change"
	VerificationPurposeAccountDeletion  VerificationPurpose = "account_deletion"
	VerificationPurposeMemberActivation VerificationPurpose = "member_activation"
	VerificationPurposeMemberLifecycle  VerificationPurpose = "member_lifecycle"
	VerificationPurposeMemberAppeal     VerificationPurpose = "member_appeal"
)

type SecurityNotificationChannel string

const (
	SecurityNotificationEmail SecurityNotificationChannel = "email"
	SecurityNotificationSMS   SecurityNotificationChannel = "sms"
)

type SecurityNotificationKind string

const (
	SecurityNotificationVerificationCode  SecurityNotificationKind = "verification_code"
	SecurityNotificationLoginLock         SecurityNotificationKind = "login_lock"
	SecurityNotificationInitialCredential SecurityNotificationKind = "initial_credential"
	SecurityNotificationPasswordReset     SecurityNotificationKind = "password_reset"
	SecurityNotificationRecoveryRequest   SecurityNotificationKind = "recovery_request"
	SecurityNotificationMemberLifecycle   SecurityNotificationKind = "member_lifecycle"
	SecurityNotificationMemberAppeal      SecurityNotificationKind = "member_appeal"
)

const (
	NotificationStatePending   = "PENDING"
	NotificationStateSending   = "SENDING"
	NotificationStateDelivered = "DELIVERED"
	NotificationStateFailed    = "FAILED"
	NotificationStateCancelled = "CANCELLED"
)

var (
	ErrVerificationInvalid     = errors.New("access: verification invalid")
	ErrVerificationExpired     = errors.New("access: verification expired")
	ErrVerificationConsumed    = errors.New("access: verification consumed")
	ErrVerificationRateLimited = errors.New("access: verification rate limited")
	ErrVerificationConflict    = errors.New("access: verification conflict")
	ErrNotificationUnavailable = errors.New("access: security notification unavailable")
	ErrNotificationInFlight    = errors.New("access: security notification in flight")
)

type VerificationPolicy struct {
	CodeTTL              time.Duration
	AuthorizationTTL     time.Duration
	ResendInterval       time.Duration
	SendLimitWindow      time.Duration
	MaxSendsPerWindow    int
	MaxVerificationTries int
	CodeDigits           int
}

func (policy VerificationPolicy) Validate() error {
	if policy.CodeTTL <= 0 || policy.AuthorizationTTL <= 0 || policy.SendLimitWindow <= 0 {
		return errors.New("access: verification TTL and send window must be configured")
	}
	if policy.ResendInterval < time.Minute {
		return errors.New("access: verification resend interval must be at least 60 seconds")
	}
	if policy.MaxSendsPerWindow < 1 || policy.MaxSendsPerWindow > 1000 {
		return errors.New("access: verification send limit must be configured")
	}
	if policy.MaxVerificationTries < 1 || policy.MaxVerificationTries > 100 {
		return errors.New("access: verification attempt limit must be configured")
	}
	if policy.CodeDigits < 4 || policy.CodeDigits > 10 {
		return errors.New("access: verification code digit count must be between 4 and 10")
	}
	return nil
}

func (purpose VerificationPurpose) Valid() bool {
	switch purpose {
	case VerificationPurposeLogin, VerificationPurposePasswordRecovery, VerificationPurposeContactChange, VerificationPurposeAccountDeletion, VerificationPurposeMemberActivation, VerificationPurposeMemberLifecycle, VerificationPurposeMemberAppeal:
		return true
	default:
		return false
	}
}

func (channel SecurityNotificationChannel) Valid() bool {
	return channel == SecurityNotificationEmail || channel == SecurityNotificationSMS
}

func (kind SecurityNotificationKind) Valid() bool {
	switch kind {
	case SecurityNotificationVerificationCode, SecurityNotificationLoginLock, SecurityNotificationInitialCredential, SecurityNotificationPasswordReset, SecurityNotificationRecoveryRequest, SecurityNotificationMemberLifecycle, SecurityNotificationMemberAppeal:
		return true
	default:
		return false
	}
}

type VerificationChallengeRequest struct {
	BusinessEventID string
	FlowID          string
	Purpose         VerificationPurpose
	UserID          string
	TenantID        string
	Channel         SecurityNotificationChannel
	Destination     string
}

func (request VerificationChallengeRequest) Validate() error {
	if strings.TrimSpace(request.BusinessEventID) == "" || strings.TrimSpace(request.FlowID) == "" || strings.TrimSpace(request.UserID) == "" {
		return ErrVerificationInvalid
	}
	if !request.Purpose.Valid() || !request.Channel.Valid() || strings.TrimSpace(request.Destination) == "" {
		return ErrVerificationInvalid
	}
	return nil
}

type VerificationChallengeReceipt struct {
	ChallengeID         string
	NotificationEventID string
	MaskedDestination   string
	ExpiresAt           time.Time
	DeliveryState       string
}

type VerifyChallengeRequest struct {
	ChallengeID string
	FlowID      string
	Purpose     VerificationPurpose
	UserID      string
	TenantID    string
	Channel     SecurityNotificationChannel
	Destination string
	Code        string
}

type OneTimeAuthorization struct {
	Code      string
	ExpiresAt time.Time
}

type ConsumeAuthorizationRequest struct {
	Code        string
	FlowID      string
	Purpose     VerificationPurpose
	UserID      string
	TenantID    string
	Channel     SecurityNotificationChannel
	Destination string
}

type AuthorizationConsumptionReceipt struct {
	ChallengeID string
	ConsumedAt  time.Time
}

type SecurityNotificationRequest struct {
	BusinessEventID string
	Kind            SecurityNotificationKind
	Purpose         VerificationPurpose
	UserID          string
	TenantID        string
	FlowID          string
	Channel         SecurityNotificationChannel
	Destination     string
	Secret          string
	ExpiresAt       time.Time
}

type SecurityNotificationClaim struct {
	EventID         string
	BusinessEventID string
	Kind            SecurityNotificationKind
	Purpose         VerificationPurpose
	UserID          string
	TenantID        string
	FlowID          string
	Channel         SecurityNotificationChannel
	Destination     string
	Secret          string
	ExpiresAt       time.Time
	Attempt         uint32
}

type NotificationDeliveryReceipt struct {
	EventID         string
	State           string
	ProviderReceipt string
	FailureCode     string
	Attempt         uint32
	DeliveredAt     *time.Time
}

type RateLimitError struct {
	RetryAfter time.Duration
	Reason     string
}

func (err RateLimitError) Error() string {
	if err.Reason == "" {
		return ErrVerificationRateLimited.Error()
	}
	return ErrVerificationRateLimited.Error() + ": " + err.Reason
}

func (err RateLimitError) Unwrap() error { return ErrVerificationRateLimited }
