package application

import (
	"context"

	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
)

const defaultInboxPageSize = 50

type InboxService struct {
	repository ports.InboxRepository
}

func NewInboxService(repository ports.InboxRepository) (*InboxService, error) {
	if repository == nil {
		return nil, domain.ErrInboxUnavailable
	}
	return &InboxService{repository: repository}, nil
}

func (service *InboxService) ReadUnread(ctx context.Context, owner domain.InboxOwner) (domain.InboxSnapshot, error) {
	if service == nil || service.repository == nil {
		return domain.InboxSnapshot{}, domain.ErrInboxUnavailable
	}
	if err := owner.Validate(); err != nil {
		return domain.InboxSnapshot{}, err
	}
	return service.repository.ReadUnreadInbox(ctx, owner, defaultInboxPageSize)
}

func (service *InboxService) MarkAllRead(ctx context.Context, owner domain.InboxOwner, idempotencyKey string) (domain.MarkAllReadReceipt, error) {
	if service == nil || service.repository == nil {
		return domain.MarkAllReadReceipt{}, domain.ErrInboxUnavailable
	}
	commandID, err := domain.InboxMarkAllCommandID(owner, idempotencyKey)
	if err != nil {
		return domain.MarkAllReadReceipt{}, err
	}
	return service.repository.MarkAllRead(ctx, owner, commandID)
}
