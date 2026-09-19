//go:build qualification

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
)

type qualificationFileSender struct {
	path string
	mu   sync.Mutex
}

var _ ports.SecurityNotificationSender = (*qualificationFileSender)(nil)

func qualificationNotificationSender() ports.SecurityNotificationSender {
	path := strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_QUALIFICATION_NOTIFICATION_FILE"))
	if path == "" {
		return nil
	}
	return &qualificationFileSender{path: path}
}

func (sender *qualificationFileSender) SendSecurityNotification(_ context.Context, claim domain.SecurityNotificationClaim) (string, error) {
	if sender == nil || sender.path == "" || claim.EventID == "" {
		return "", errors.New("qualification notification sink unavailable")
	}
	sender.mu.Lock()
	defer sender.mu.Unlock()
	file, err := os.OpenFile(sender.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	defer file.Close()
	payload := map[string]any{
		"event_id":    claim.EventID,
		"kind":        claim.Kind,
		"purpose":     claim.Purpose,
		"channel":     claim.Channel,
		"destination": claim.Destination,
		"secret":      claim.Secret,
		"attempt":     claim.Attempt,
	}
	if err := json.NewEncoder(file).Encode(payload); err != nil {
		return "", err
	}
	return "qualification:" + claim.EventID, nil
}
