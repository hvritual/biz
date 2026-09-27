package main

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestCE16GraceDurationRejectsInvalidAndNegativeValues(t *testing.T) {
	t.Setenv("YUNKA_BIZ_COMMERCIAL_GRACE_DURATION", "bad")
	if _, err := lifecycleConfiguration(); err == nil {
		t.Fatal("invalid grace accepted")
	}
	t.Setenv("YUNKA_BIZ_COMMERCIAL_GRACE_DURATION", "-1h")
	if _, err := lifecycleConfiguration(); err == nil {
		t.Fatal("negative grace accepted")
	}
	t.Setenv("YUNKA_BIZ_COMMERCIAL_GRACE_DURATION", "0")
	if policy, err := lifecycleConfiguration(); err != nil || policy.GraceDuration != 0 {
		t.Fatalf("zero grace=%v err=%v", policy.GraceDuration, err)
	}
}

func TestEnterprise185NotificationRuntimeEnvironmentIsExplicitAndFailClosed(t *testing.T) {
	t.Setenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_ENDPOINT", "")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_BEARER_TOKEN", "partial")
	if _, err := notificationRuntimeConfiguration(); err == nil {
		t.Fatal("partial notification provider configuration accepted")
	}

	t.Setenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_ENDPOINT", "http://127.0.0.1:18085/send")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_BEARER_TOKEN", "provider-token")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_IDEMPOTENT", "true")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_CHANNELS", "email, sms")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_CALLBACK_HMAC_KEY_B64", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("H", 32))))
	t.Setenv("YUNKA_BIZ_NOTIFICATION_POLL_INTERVAL", "25ms")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_ROUTING_LEASE", "30s")
	config, err := notificationRuntimeConfiguration()
	if err != nil {
		t.Fatal(err)
	}
	if !config.Enabled() || !config.ProviderIdempotent || len(config.Channels) != 2 ||
		config.PollInterval != 25*time.Millisecond || config.RoutingLeaseDuration != 30*time.Second ||
		len(config.CallbackHMACSecret) != 32 {
		t.Fatalf("notification runtime config=%+v", config)
	}

	t.Setenv("YUNKA_BIZ_NOTIFICATION_CALLBACK_HMAC_KEY_B64", "not-base64")
	if _, err := notificationRuntimeConfiguration(); err == nil {
		t.Fatal("invalid callback HMAC key encoding accepted")
	}
}
