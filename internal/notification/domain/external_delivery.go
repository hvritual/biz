package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	ExternalTaskStateLeased           = "LEASED"
	ExternalTaskStateRetryWait        = "RETRY_WAIT"
	ExternalTaskStateProviderAccepted = "PROVIDER_ACCEPTED"
	ExternalTaskStateDelivered        = "DELIVERED"
	ExternalTaskStateCancelled        = "CANCELLED"
	ExternalTaskStateManualReview     = "MANUAL_REVIEW"
)

var (
	ErrExternalDeliveryInvalid     = errors.New("notification: invalid external delivery")
	ErrExternalDeliveryLease       = errors.New("notification: external delivery lease invalid")
	ErrExternalDeliveryUnavailable = errors.New("notification: external delivery unavailable")
)

type ExternalDeliveryPolicy struct {
	MaxAttempts   uint32
	Backoff       []time.Duration
	LeaseDuration time.Duration
	SendTimeout   time.Duration
}

func EnterpriseExternalDeliveryPolicy() ExternalDeliveryPolicy {
	return ExternalDeliveryPolicy{
		MaxAttempts:   5,
		Backoff:       []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute},
		LeaseDuration: time.Minute,
		SendTimeout:   15 * time.Second,
	}
}

func (policy ExternalDeliveryPolicy) Validate() error {
	if policy.MaxAttempts < 1 || policy.MaxAttempts > 20 || len(policy.Backoff) != int(policy.MaxAttempts-1) {
		return ErrExternalDeliveryInvalid
	}
	if policy.LeaseDuration < 5*time.Second || policy.LeaseDuration > 5*time.Minute ||
		policy.SendTimeout < time.Second || policy.SendTimeout >= policy.LeaseDuration {
		return ErrExternalDeliveryInvalid
	}
	var previous time.Duration
	for _, delay := range policy.Backoff {
		if delay <= 0 || delay > 24*time.Hour || delay < previous {
			return ErrExternalDeliveryInvalid
		}
		previous = delay
	}
	return nil
}

func (policy ExternalDeliveryPolicy) NextDelay(attempt uint32) (time.Duration, bool) {
	if policy.Validate() != nil || attempt == 0 || attempt >= policy.MaxAttempts {
		return 0, false
	}
	return policy.Backoff[attempt-1], true
}

type ExternalTaskClaim struct {
	TaskID               string
	TenantID             string
	UserID               string
	EventID              string
	Channel              string
	ConfigurationID      string
	ConfigurationVersion uint64
	GroupID              string
	TypeCode             string
	Level                MessageLevel
	TraceID              string
	ReferenceKind        string
	ReferenceID          string
	Attempt              uint32
	WorkerID             string
	LeaseToken           uint64
	LeaseUntil           time.Time
	RecoveredUnknownOutcome bool
}

func (claim ExternalTaskClaim) Validate() error {
	if !ValidConfigurationID(claim.TaskID, 64) || !ValidConfigurationID(claim.TenantID, 64) ||
		!ValidConfigurationID(claim.UserID, 64) || !ValidConfigurationID(claim.EventID, 160) ||
		!validCode(claim.Channel) || !ValidConfigurationID(claim.ConfigurationID, 64) ||
		claim.ConfigurationVersion == 0 || !ValidConfigurationID(claim.GroupID, 64) ||
		!validCode(claim.TypeCode) || !claim.Level.Valid() || !ValidConfigurationID(claim.TraceID, 160) ||
		!ValidConfigurationID(claim.ReferenceKind, 64) || !ValidConfigurationID(claim.ReferenceID, 160) ||
		claim.Attempt == 0 || !ValidConfigurationID(claim.WorkerID, 96) ||
		claim.LeaseToken == 0 || claim.LeaseUntil.IsZero() {
		return ErrExternalDeliveryInvalid
	}
	return nil
}

type ExternalTaskReceipt struct {
	TaskID          string
	State           string
	Attempt         uint32
	FailureCode     string
	ProviderReceipt string
	NextAttemptAt   *time.Time
}

type ExternalProviderStatus string

const (
	ExternalProviderAccepted  ExternalProviderStatus = "accepted"
	ExternalProviderDelivered ExternalProviderStatus = "delivered"
)

type ExternalProviderRequest struct {
	TaskID        string
	EventID       string
	TenantID      string
	UserID        string
	Channel       string
	Destination   string
	TypeCode      string
	Level         MessageLevel
	TraceID       string
	ReferenceKind string
	ReferenceID   string
}

func (request ExternalProviderRequest) Validate() error {
	if !ValidConfigurationID(request.TaskID, 64) || !ValidConfigurationID(request.EventID, 160) ||
		!ValidConfigurationID(request.TenantID, 64) || !ValidConfigurationID(request.UserID, 64) ||
		!validCode(request.Channel) || strings.TrimSpace(request.Destination) == "" ||
		!validCode(request.TypeCode) || !request.Level.Valid() || !ValidConfigurationID(request.TraceID, 160) ||
		!ValidConfigurationID(request.ReferenceKind, 64) || !ValidConfigurationID(request.ReferenceID, 160) {
		return ErrExternalDeliveryInvalid
	}
	return nil
}

type ExternalProviderResult struct {
	ReceiptID string
	Status    ExternalProviderStatus
}

func (result ExternalProviderResult) Validate() error {
	if strings.TrimSpace(result.ReceiptID) == "" || len(result.ReceiptID) > 200 {
		return ErrExternalDeliveryInvalid
	}
	if result.Status != ExternalProviderAccepted && result.Status != ExternalProviderDelivered {
		return ErrExternalDeliveryInvalid
	}
	return nil
}
