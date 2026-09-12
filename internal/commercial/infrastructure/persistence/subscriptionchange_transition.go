package persistence

import (
	"context"
	"encoding/json"

	change "github.com/hvritual/biz/internal/commercial/domain/subscriptionchange"
)

// UpdateReceipt advances the same immutable commercial approval through its
// execution state. The request/fingerprint/preview authority may not change.
func (r *subscriptionChangeRepository) UpdateReceipt(ctx context.Context, before, after change.Receipt) error {
	if before.ChangeID != after.ChangeID || before.TenantID != after.TenantID || before.ActorID != after.ActorID || before.RequestID != after.RequestID || before.Fingerprint != after.Fingerprint || before.PreviewHash != after.PreviewHash || before.Action != after.Action || before.Mode != after.Mode || !before.ConfirmedAt.Equal(after.ConfirmedAt) || before.PricingAuthority != after.PricingAuthority || after.Integrity() != nil {
		return change.ErrConflict
	}
	payload, err := json.Marshal(after)
	if err != nil {
		return err
	}
	result := r.tx.WithContext(ctx).Model(&changeReceiptRow{}).
		Where("change_id=? AND tenant_id=? AND payload_sha256=? AND status=?", before.ChangeID, before.TenantID, before.Hash, before.Status).
		Updates(map[string]any{"payload_sha256": after.Hash, "payload": string(payload), "status": after.Status})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return change.ErrConflict
	}
	return nil
}
