package delivery

import (
	"context"
	"errors"
	"sync"

	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
)

type MemoryProvider struct {
	mu       sync.Mutex
	requests map[string]domain.ExternalProviderRequest
	result   domain.ExternalProviderResult
	failErr  error
}

var _ ports.ExternalNotificationProvider = (*MemoryProvider)(nil)
var _ ports.ExternalNotificationProviderRetrySafety = (*MemoryProvider)(nil)

func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{
		requests: map[string]domain.ExternalProviderRequest{},
		result:   domain.ExternalProviderResult{ReceiptID: "memory-accepted", Status: domain.ExternalProviderAccepted},
	}
}

func (provider *MemoryProvider) ExternalNotificationIdempotent() bool { return true }

func (provider *MemoryProvider) SetFailure(err error) {
	provider.mu.Lock()
	provider.failErr = err
	provider.mu.Unlock()
}

func (provider *MemoryProvider) SetResult(result domain.ExternalProviderResult) {
	provider.mu.Lock()
	provider.result = result
	provider.mu.Unlock()
}

func (provider *MemoryProvider) SendExternalNotification(_ context.Context, request domain.ExternalProviderRequest) (domain.ExternalProviderResult, error) {
	if err := request.Validate(); err != nil {
		return domain.ExternalProviderResult{}, err
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.failErr != nil {
		return domain.ExternalProviderResult{}, provider.failErr
	}
	if previous, ok := provider.requests[request.TaskID]; ok && previous != request {
		return domain.ExternalProviderResult{}, errors.New("notification delivery: idempotency key reused with different payload")
	}
	provider.requests[request.TaskID] = request
	result := provider.result
	if result.ReceiptID == "memory-accepted" {
		result.ReceiptID = "memory:" + request.TaskID
	}
	return result, result.Validate()
}

func (provider *MemoryProvider) Request(taskID string) (domain.ExternalProviderRequest, bool) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	request, ok := provider.requests[taskID]
	return request, ok
}

func (provider *MemoryProvider) Count() int {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	return len(provider.requests)
}
