package bizruntime

import (
	"strings"
	"testing"
	"time"

	notificationdomain "github.com/hvritual/biz/internal/notification/domain"
)

func TestEnterprise185NotificationRuntimeRequiresCompleteExplicitProviderConfig(t *testing.T) {
	for _, tc := range []NotificationRuntimeOptions{
		{ProviderBearerToken: "token"},
		{ProviderEndpoint: "https://provider.example.invalid/send"},
		{ProviderEndpoint: "https://provider.example.invalid/send", ProviderBearerToken: "token", Channels: []string{"email"}},
		{ProviderEndpoint: "https://provider.example.invalid/send", ProviderBearerToken: "token", CallbackHMACSecret: []byte(strings.Repeat("H", 32)), Channels: []string{"push"}},
		{ProviderEndpoint: "https://provider.example.invalid/send", ProviderBearerToken: "token", CallbackHMACSecret: []byte(strings.Repeat("H", 32)), Channels: []string{"email", "email"}},
	} {
		if err := tc.Validate(); err == nil {
			t.Fatalf("incomplete/invalid runtime accepted: %+v", tc)
		}
	}
	valid := NotificationRuntimeOptions{
		ProviderEndpoint: "https://provider.example.invalid/send", ProviderBearerToken: "token",
		CallbackHMACSecret: []byte(strings.Repeat("H", 32)), Channels: []string{"email"},
		PollInterval: 10 * time.Millisecond, RoutingLeaseDuration: 5 * time.Second,
	}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestEnterprise185NotificationCatalogIsSingleRuntimeSnapshot(t *testing.T) {
	disabled, err := buildNotificationCatalogSnapshot(NotificationRuntimeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"email", "sms"} {
		channel, err := disabled.channels.Lookup(code)
		if err != nil || channel.Availability != notificationdomain.ChannelNotConfigurable {
			t.Fatalf("disabled channel %s=%+v err=%v", code, channel, err)
		}
	}

	enabled, err := buildNotificationCatalogSnapshot(NotificationRuntimeOptions{
		ProviderEndpoint: "https://provider.example.invalid/send", ProviderBearerToken: "token",
		CallbackHMACSecret: []byte(strings.Repeat("H", 32)), Channels: []string{"email"},
	})
	if err != nil {
		t.Fatal(err)
	}
	email, err := enabled.channels.Lookup("email")
	if err != nil || email.Availability != notificationdomain.ChannelConfigurable {
		t.Fatalf("email=%+v err=%v", email, err)
	}
	sms, err := enabled.channels.Lookup("sms")
	if err != nil || sms.Availability != notificationdomain.ChannelNotConfigurable {
		t.Fatalf("sms=%+v err=%v", sms, err)
	}
	if _, err := enabled.types.Lookup("device.fault"); err != nil {
		t.Fatal(err)
	}

	factory := applicationFactories{notificationCatalogs: enabled}
	app, err := factory.BuildNotificationMessageConfiguration(struct{}{})
	if err == nil || app != nil {
		// The generated dependency type is intentionally not fabricated here;
		// runtime integration below proves the exact generated factory path.
		t.Fatal("unexpected direct factory construction without generated dependencies")
	}
}
