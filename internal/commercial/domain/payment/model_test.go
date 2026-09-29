package payment

import (
	"testing"
	"time"
)

func TestOrderPaidCallbackIsBoundToCommercialFacts(t *testing.T) {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	order, err := NewOrder("ord-20260929-001", "tenant-a", "chg-001", "growth", 2, "price-growth", "CNY", 19900, WeChatNative, now, now.Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := order.RecordPaid(ProviderReceipt{Provider: WeChatNative, ProviderTransactionID: "wx-txn-1", Currency: "CNY", AmountMinor: 19900, PaidAt: now.Add(time.Minute)}, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if order.State != Paid || order.ProviderTransactionID != "wx-txn-1" || order.Revision != 2 {
		t.Fatalf("paid order = %+v", order)
	}
	if _, err := order.RecordPaid(ProviderReceipt{Provider: WeChatNative, ProviderTransactionID: "wx-txn-1", Currency: "CNY", AmountMinor: 19900, PaidAt: now.Add(time.Minute)}, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("exact replay must be safe: %v", err)
	}
	if _, err := order.RecordPaid(ProviderReceipt{Provider: WeChatNative, ProviderTransactionID: "wx-txn-2", Currency: "CNY", AmountMinor: 19900, PaidAt: now.Add(time.Minute)}, now.Add(2*time.Minute)); err == nil {
		t.Fatal("different provider transaction replay accepted")
	}
}

func TestOrderRejectsProviderOrSettlementMismatch(t *testing.T) {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	order, err := NewOrder("ord-20260929-002", "tenant-a", "chg-002", "growth", 2, "price-growth", "CNY", 19900, AlipayPage, now, now.Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	cases := []ProviderReceipt{
		{Provider: WeChatNative, ProviderTransactionID: "wx-txn", Currency: "CNY", AmountMinor: 19900, PaidAt: now.Add(time.Minute)},
		{Provider: AlipayPage, ProviderTransactionID: "ali-txn", Currency: "USD", AmountMinor: 19900, PaidAt: now.Add(time.Minute)},
		{Provider: AlipayPage, ProviderTransactionID: "ali-txn", Currency: "CNY", AmountMinor: 1, PaidAt: now.Add(time.Minute)},
	}
	for _, receipt := range cases {
		if _, err := order.RecordPaid(receipt, now.Add(time.Minute)); err == nil {
			t.Fatalf("mismatched receipt accepted: %+v", receipt)
		}
	}
}
