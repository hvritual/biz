package paymentchannel

import (
	"testing"
	"time"

	"github.com/hvritual/biz/internal/commercial/domain/payment"
)

func paidOrder(t *testing.T, provider string) payment.Order {
	t.Helper()
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	order, err := payment.NewOrder("ord-001", "tenant-a", "chg-001", "growth", 2, "price-growth", "CNY", 19900, provider, now, now.Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func TestWechatNativeAdapterBuildsOnlyServerOwnedSettlementRequest(t *testing.T) {
	adapter, err := NewWechatNative(Config{NotifyURL: "https://merchant.example/payments/wechat/notify"})
	if err != nil {
		t.Fatal(err)
	}
	instruction, err := adapter.CreateInstruction(paidOrder(t, payment.WeChatNative))
	if err != nil {
		t.Fatal(err)
	}
	if instruction.Provider != payment.WeChatNative || instruction.Mode != QRCode || instruction.OrderID != "ord-001" || instruction.AmountMinor != 19900 || instruction.Currency != "CNY" || instruction.NotifyURL != "https://merchant.example/payments/wechat/notify" {
		t.Fatalf("instruction = %+v", instruction)
	}
}

func TestAlipayPageAdapterRejectsDifferentProviderOrder(t *testing.T) {
	adapter, err := NewAlipayPage(Config{NotifyURL: "https://merchant.example/payments/alipay/notify", ReturnURL: "https://merchant.example/subscription/result"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.CreateInstruction(paidOrder(t, payment.WeChatNative)); err == nil {
		t.Fatal("wechat order accepted by alipay adapter")
	}
	instruction, err := adapter.CreateInstruction(paidOrder(t, payment.AlipayPage))
	if err != nil {
		t.Fatal(err)
	}
	if instruction.Mode != Redirect || instruction.ReturnURL == "" || instruction.AmountMinor != 19900 {
		t.Fatalf("instruction = %+v", instruction)
	}
}
