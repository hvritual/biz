package application

import (
	"context"

	"github.com/hvritual/biz/internal/access/domain"
)

func (repository *memoryTenantMemberRepository) CountQuotaMembers(_ context.Context, tenantID string) (uint64, error) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	var count uint64
	for _, member := range repository.values {
		if member.TenantID == tenantID && member.Status != domain.TenantMemberStatusRemoved {
			count++
		}
	}
	return count, nil
}
