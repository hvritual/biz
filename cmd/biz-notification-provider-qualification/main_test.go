package main

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func setValidProviderQualificationEnv(t *testing.T) {
	t.Helper()
	t.Setenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_PROVIDER_ID", "sandbox-email")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_ENDPOINT", "https://provider.example.com/v1/send")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_BEARER_TOKEN", "provider-token")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_IDEMPOTENT", "true")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CHANNEL", "email")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_DESTINATION", "qualification@example.com")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CALLBACK_LISTEN", "0.0.0.0:18086")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CALLBACK_PUBLIC_URL", "https://callback.example.com/callbacks/notification/provider")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_CALLBACK_HMAC_KEY_B64", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("H", 32))))
	t.Setenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_REQUEST_TIMEOUT", "5s")
	t.Setenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_TIMEOUT", "30s")
}

func TestEnterprise185ProviderQualificationEnvironmentIsExplicit(t *testing.T) {
	setValidProviderQualificationEnv(t)
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.ProviderID != "sandbox-email" || !config.ProviderIdempotent ||
		config.Channel != "email" || config.RequestTimeout != 5*time.Second ||
		config.Timeout != 30*time.Second || len(config.CallbackHMACSecret) != 32 {
		t.Fatalf("config=%+v", config)
	}

	t.Setenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_IDEMPOTENT", "")
	if _, err := loadConfig(); err == nil {
		t.Fatal("missing provider idempotency declaration accepted")
	}
}

func TestEnterprise185ProviderQualificationEnvironmentRejectsSecretAndEndpointErrors(t *testing.T) {
	setValidProviderQualificationEnv(t)
	t.Setenv("YUNKA_BIZ_NOTIFICATION_CALLBACK_HMAC_KEY_B64", "not-base64")
	if _, err := loadConfig(); err == nil {
		t.Fatal("invalid callback secret encoding accepted")
	}

	setValidProviderQualificationEnv(t)
	t.Setenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_ENDPOINT", "http://127.0.0.1:18085/send")
	if _, err := loadConfig(); err == nil {
		t.Fatal("loopback provider qualified as external")
	}

	setValidProviderQualificationEnv(t)
	t.Setenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CALLBACK_PUBLIC_URL", "https://callback.example.com/wrong")
	if _, err := loadConfig(); err == nil {
		t.Fatal("wrong callback path accepted")
	}
}
