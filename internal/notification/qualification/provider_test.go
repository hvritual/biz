package qualification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/notification/domain"
)

func validProviderConfig() ProviderConfig {
	return ProviderConfig{
		ProviderID:            "sandbox-email",
		ProviderEndpoint:      "https://provider.example.com/v1/send",
		ProviderBearerToken:   "provider-token",
		ProviderIdempotent:    true,
		Channel:               "email",
		Destination:           "qualification@example.com",
		CallbackListenAddress: "0.0.0.0:18086",
		CallbackPublicURL:     "https://callback.example.com" + CallbackPath,
		CallbackHMACSecret:    []byte(strings.Repeat("H", 32)),
		RequestTimeout:        5 * time.Second,
		Timeout:               30 * time.Second,
	}
}

func TestEnterprise185ProviderQualificationRejectsMockAndIncompleteEndpoints(t *testing.T) {
	cases := []ProviderConfig{
		func() ProviderConfig {
			c := validProviderConfig()
			c.ProviderEndpoint = "http://127.0.0.1:18085/send"
			return c
		}(),
		func() ProviderConfig {
			c := validProviderConfig()
			c.ProviderEndpoint = "https://127.0.0.1:18085/send"
			return c
		}(),
		func() ProviderConfig {
			c := validProviderConfig()
			c.CallbackPublicURL = "https://127.0.0.1:18086" + CallbackPath
			return c
		}(),
		func() ProviderConfig {
			c := validProviderConfig()
			c.CallbackPublicURL = "https://callback.example.com/wrong"
			return c
		}(),
		func() ProviderConfig {
			c := validProviderConfig()
			c.ProviderBearerToken = ""
			return c
		}(),
		func() ProviderConfig {
			c := validProviderConfig()
			c.CallbackHMACSecret = []byte("short")
			return c
		}(),
	}
	for i, config := range cases {
		if err := config.Validate(); err == nil {
			t.Fatalf("case %d unexpectedly qualified", i)
		}
	}
	if err := validProviderConfig().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestEnterprise185QualificationCallbackWaitsForExactProviderReceipt(t *testing.T) {
	store := newCallbackStore("provider-qualification-task")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result := make(chan error, 1)
	go func() {
		_, err := store.ApplyExternalProviderCallback(ctx, domain.ExternalProviderCallback{
			TaskID:    "provider-qualification-task",
			ReceiptID: "provider-receipt",
			Status:    domain.ExternalProviderCallbackDelivered,
		})
		result <- err
	}()

	select {
	case err := <-result:
		t.Fatalf("callback returned before provider receipt was armed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	store.arm("provider-receipt")
	if err := <-result; err != nil {
		t.Fatal(err)
	}

	duplicate, err := store.ApplyExternalProviderCallback(ctx, domain.ExternalProviderCallback{
		TaskID:    "provider-qualification-task",
		ReceiptID: "provider-receipt",
		Status:    domain.ExternalProviderCallbackDelivered,
	})
	if err != nil || duplicate.State != domain.ExternalTaskStateDelivered {
		t.Fatalf("duplicate receipt=%+v err=%v", duplicate, err)
	}

	_, err = store.ApplyExternalProviderCallback(ctx, domain.ExternalProviderCallback{
		TaskID:      "provider-qualification-task",
		ReceiptID:   "provider-receipt",
		Status:      domain.ExternalProviderCallbackFailed,
		FailureCode: "PROVIDER_BOUNCED",
	})
	if !errors.Is(err, domain.ErrExternalDeliveryLease) {
		t.Fatalf("conflicting terminal callback err=%v", err)
	}

	other := newCallbackStore("provider-qualification-other")
	other.arm("expected")
	_, err = other.ApplyExternalProviderCallback(ctx, domain.ExternalProviderCallback{
		TaskID:    "provider-qualification-other",
		ReceiptID: "wrong",
		Status:    domain.ExternalProviderCallbackDelivered,
	})
	if !errors.Is(err, domain.ErrExternalDeliveryLease) {
		t.Fatalf("mismatched provider receipt err=%v", err)
	}
}

func TestEnterprise185QualificationReceiptContainsHashesNotSecrets(t *testing.T) {
	config := validProviderConfig()
	receipt := Receipt{
		SchemaVersion:       1,
		State:               "PASS",
		ProviderID:          config.ProviderID,
		ProviderHost:        hostOnly(config.ProviderEndpoint),
		ProviderIdempotent:  config.ProviderIdempotent,
		Channel:             config.Channel,
		DestinationHash:     sha256Hex(config.Destination),
		ProviderReceiptHash: sha256Hex("provider-receipt"),
		CallbackHost:        hostOnly(config.CallbackPublicURL),
	}
	dump := strings.Join([]string{
		receipt.ProviderID, receipt.ProviderHost, receipt.Channel,
		receipt.DestinationHash, receipt.ProviderReceiptHash, receipt.CallbackHost,
	}, "|")
	for _, secret := range []string{config.Destination, config.ProviderBearerToken, string(config.CallbackHMACSecret), "provider-receipt"} {
		if strings.Contains(dump, secret) {
			t.Fatalf("qualification receipt leaked protected value")
		}
	}
}
