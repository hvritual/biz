package domain

import (
	"errors"
	"strings"
	"time"
)

const (
	NotificationStateRetryWait    = "RETRY_WAIT"
	NotificationStateManualReview = "MANUAL_REVIEW"
)

var (
	ErrNotificationLease          = errors.New("access: security notification lease invalid")
	ErrNotificationRetryExhausted = errors.New("access: security notification retries exhausted")
)

type NotificationRetryPolicy struct {
	MaxAttempts   uint32
	Backoff       []time.Duration
	LeaseDuration time.Duration
	SendTimeout   time.Duration
}

func EnterpriseNotificationRetryPolicy() NotificationRetryPolicy {
	return NotificationRetryPolicy{
		MaxAttempts:   5,
		Backoff:       []time.Duration{30 * time.Second, 2 * time.Minute, 10 * time.Minute, 30 * time.Minute},
		LeaseDuration: time.Minute,
		SendTimeout:   15 * time.Second,
	}
}

func (policy NotificationRetryPolicy) Validate() error {
	if policy.MaxAttempts < 1 || policy.MaxAttempts > 20 || len(policy.Backoff) != int(policy.MaxAttempts-1) {
		return ErrVerificationInvalid
	}
	if policy.LeaseDuration < 5*time.Second || policy.LeaseDuration > 5*time.Minute ||
		policy.SendTimeout < time.Second || policy.SendTimeout >= policy.LeaseDuration {
		return ErrVerificationInvalid
	}
	var previous time.Duration
	for _, delay := range policy.Backoff {
		if delay <= 0 || delay > 24*time.Hour || delay < previous {
			return ErrVerificationInvalid
		}
		previous = delay
	}
	return nil
}

func (policy NotificationRetryPolicy) NextDelay(attempt uint32) (time.Duration, bool) {
	if policy.Validate() != nil || attempt == 0 || attempt >= policy.MaxAttempts {
		return 0, false
	}
	return policy.Backoff[attempt-1], true
}

type ReliableSecurityNotificationClaim struct {
	SecurityNotificationClaim
	WorkerID   string
	LeaseToken uint64
	LeaseUntil time.Time
}

func (claim ReliableSecurityNotificationClaim) Validate() error {
	if strings.TrimSpace(claim.EventID) == "" || strings.TrimSpace(claim.WorkerID) == "" ||
		claim.Attempt == 0 || claim.LeaseToken == 0 || claim.LeaseUntil.IsZero() {
		return ErrNotificationLease
	}
	return nil
}

type NotificationDeliveryAttemptResult struct {
	EventID       string
	State         string
	FailureCode   string
	Attempt       uint32
	NextAttemptAt *time.Time
}
