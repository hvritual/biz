package notification

import (
	"context"

	"github.com/hvritual/biz/internal/access/domain"
	"github.com/hvritual/biz/internal/access/ports"
)

// AdmitOptionalNotification calls enqueue only after a valid, current preference
// permits this channel. The caller must resolve the recipient through trusted
// authority first. No importance flag or security-message exemption exists here.
//
// This is an admission seam, not a delivery receipt. The later reliable outbox
// consumer must serialize its own admission and recheck before external delivery;
// a preference can change after this read. Never reuse this result as a cache.
func AdmitOptionalNotification(ctx context.Context, reader ports.OptionalNotificationPreferenceReader,
	owner domain.NotificationPreferenceOwner, channel domain.NotificationPreferenceChannel,
	enqueue func(context.Context) error,
) (bool, error) {
	if err := owner.Validate(); err != nil {
		return false, err
	}
	if !channel.Valid() || reader == nil || enqueue == nil {
		return false, domain.ErrNotificationPreferenceInvalid
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	preference, err := reader.ReadNotificationPreference(ctx, owner, channel)
	if err != nil {
		return false, err
	}
	if preference.Channel != channel {
		return false, domain.ErrNotificationPreferenceInvalid
	}
	allowed, err := preference.OptionalAllowed()
	if err != nil || !allowed {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := enqueue(ctx); err != nil {
		return false, err
	}
	return true, nil
}
