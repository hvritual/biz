package ports

import (
	"context"

	"github.com/hvritual/biz/internal/notification/domain"
)

type InboxRepository interface {
	ReadUnreadInbox(context.Context, domain.InboxOwner, int) (domain.InboxSnapshot, error)
	MarkAllRead(context.Context, domain.InboxOwner, string) (domain.MarkAllReadReceipt, error)
}
