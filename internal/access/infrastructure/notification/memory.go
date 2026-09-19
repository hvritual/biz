package notification

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
)

type MemorySender struct {
	mu       sync.Mutex
	messages map[string]domain.SecurityNotificationClaim
	failErr  error
}

var _ ports.SecurityNotificationSender = (*MemorySender)(nil)

func NewMemorySender() *MemorySender {
	return &MemorySender{messages: make(map[string]domain.SecurityNotificationClaim)}
}

func (sender *MemorySender) SetFailure(err error) {
	sender.mu.Lock()
	sender.failErr = err
	sender.mu.Unlock()
}

func (sender *MemorySender) SendSecurityNotification(_ context.Context, claim domain.SecurityNotificationClaim) (string, error) {
	if sender == nil || claim.EventID == "" {
		return "", errors.New("access notification: invalid test delivery")
	}
	sender.mu.Lock()
	defer sender.mu.Unlock()
	if sender.failErr != nil {
		return "", sender.failErr
	}
	if previous, ok := sender.messages[claim.EventID]; ok {
		if previous.BusinessEventID != claim.BusinessEventID || previous.Kind != claim.Kind || previous.Channel != claim.Channel || previous.Destination != claim.Destination || previous.Secret != claim.Secret {
			return "", errors.New("access notification: idempotency key reused with different payload")
		}
		return "memory:" + claim.EventID, nil
	}
	sender.messages[claim.EventID] = claim
	return "memory:" + claim.EventID, nil
}

func (sender *MemorySender) Message(eventID string) (domain.SecurityNotificationClaim, bool) {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	value, ok := sender.messages[eventID]
	return value, ok
}

func (sender *MemorySender) Count() int {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	return len(sender.messages)
}

func (sender *MemorySender) String() string {
	sender.mu.Lock()
	defer sender.mu.Unlock()
	return fmt.Sprintf("MemorySender(messages=%d)", len(sender.messages))
}
