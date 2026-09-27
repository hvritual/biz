package domain

import (
	"errors"
	"time"
)

var (
	ErrInboxInvalid     = errors.New("notification: invalid inbox request")
	ErrInboxUnavailable = errors.New("notification: inbox unavailable")
)

type InboxOwner struct {
	TenantID string `json:"tenant_id"`
	UserID   string `json:"user_id"`
}

func (owner InboxOwner) Validate() error {
	if !ValidConfigurationID(owner.TenantID, 64) || !ValidConfigurationID(owner.UserID, 64) {
		return ErrInboxInvalid
	}
	return nil
}

type InAppMessage struct {
	MessageID     string       `json:"message_id"`
	TypeCode      string       `json:"type_code"`
	Level         MessageLevel `json:"level"`
	ReferenceKind string       `json:"reference_kind"`
	ReferenceID   string       `json:"reference_id"`
	CreatedAt     time.Time    `json:"created_at"`
}

type InboxSnapshot struct {
	TenantID    string         `json:"tenant_id"`
	UserID      string         `json:"user_id"`
	UnreadCount uint64         `json:"unread_count"`
	Messages    []InAppMessage `json:"messages"`
	AsOf        time.Time      `json:"as_of"`
}

type MarkAllReadReceipt struct {
	ReceiptID   string    `json:"receipt_id"`
	TenantID    string    `json:"tenant_id"`
	UserID      string    `json:"user_id"`
	MarkedCount uint64    `json:"marked_count"`
	ReadAt      time.Time `json:"read_at"`
}

func InboxMarkAllCommandID(owner InboxOwner, idempotencyKey string) (string, error) {
	if err := owner.Validate(); err != nil || len(idempotencyKey) == 0 || len(idempotencyKey) > 256 {
		return "", ErrInboxInvalid
	}
	for i := 0; i < len(idempotencyKey); i++ {
		if idempotencyKey[i] < 0x21 || idempotencyKey[i] > 0x7e {
			return "", ErrInboxInvalid
		}
	}
	return StableRoutingID("inbox_mark_all", owner.TenantID, owner.UserID, idempotencyKey), nil
}
