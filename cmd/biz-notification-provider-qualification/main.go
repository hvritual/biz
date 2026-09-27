package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/notification/qualification"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	receipt, runErr := qualification.RunProvider(context.Background(), config)
	if receipt.SchemaVersion != 0 {
		if err := writeReceipt(receipt); err != nil {
			return err
		}
	}
	return runErr
}

func loadConfig() (qualification.ProviderConfig, error) {
	idempotentRaw := strings.TrimSpace(os.Getenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_IDEMPOTENT"))
	if idempotentRaw == "" {
		return qualification.ProviderConfig{}, errors.New("YUNKA_BIZ_NOTIFICATION_PROVIDER_IDEMPOTENT must be explicitly true or false")
	}
	idempotent, err := strconv.ParseBool(idempotentRaw)
	if err != nil {
		return qualification.ProviderConfig{}, errors.New("invalid YUNKA_BIZ_NOTIFICATION_PROVIDER_IDEMPOTENT")
	}
	secretRaw := strings.TrimSpace(os.Getenv("YUNKA_BIZ_NOTIFICATION_CALLBACK_HMAC_KEY_B64"))
	if secretRaw == "" {
		return qualification.ProviderConfig{}, errors.New("YUNKA_BIZ_NOTIFICATION_CALLBACK_HMAC_KEY_B64 is required")
	}
	secret, err := base64.StdEncoding.DecodeString(secretRaw)
	if err != nil {
		return qualification.ProviderConfig{}, errors.New("invalid YUNKA_BIZ_NOTIFICATION_CALLBACK_HMAC_KEY_B64")
	}
	requestTimeout, err := envDuration("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_REQUEST_TIMEOUT", 15*time.Second)
	if err != nil {
		return qualification.ProviderConfig{}, err
	}
	timeout, err := envDuration("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_TIMEOUT", 2*time.Minute)
	if err != nil {
		return qualification.ProviderConfig{}, err
	}
	config := qualification.ProviderConfig{
		ProviderID:            strings.TrimSpace(os.Getenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_PROVIDER_ID")),
		ProviderEndpoint:      strings.TrimSpace(os.Getenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_ENDPOINT")),
		ProviderBearerToken:   os.Getenv("YUNKA_BIZ_NOTIFICATION_PROVIDER_BEARER_TOKEN"),
		ProviderIdempotent:    idempotent,
		Channel:               strings.TrimSpace(os.Getenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CHANNEL")),
		Destination:           strings.TrimSpace(os.Getenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_DESTINATION")),
		CallbackListenAddress: strings.TrimSpace(os.Getenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CALLBACK_LISTEN")),
		CallbackPublicURL:     strings.TrimSpace(os.Getenv("YUNKA_BIZ_NOTIFICATION_QUALIFICATION_CALLBACK_PUBLIC_URL")),
		CallbackHMACSecret:    secret,
		RequestTimeout:        requestTimeout,
		Timeout:               timeout,
	}
	if err := config.Validate(); err != nil {
		return qualification.ProviderConfig{}, err
	}
	return config, nil
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return value, nil
}

func writeReceipt(receipt qualification.Receipt) error {
	document := map[string]any{
		"schema_version":          receipt.SchemaVersion,
		"state":                   receipt.State,
		"provider_id":             receipt.ProviderID,
		"provider_host":           receipt.ProviderHost,
		"provider_idempotent":     receipt.ProviderIdempotent,
		"channel":                 receipt.Channel,
		"destination_sha256":      receipt.DestinationHash,
		"task_id":                 receipt.TaskID,
		"trace_id":                receipt.TraceID,
		"callback_host":           receipt.CallbackHost,
		"elapsed_ms":              receipt.ElapsedMillis,
		"observed_at":             receipt.ObservedAt.UTC().Format(time.RFC3339Nano),
	}
	if receipt.ProviderReceiptHash != "" {
		document["provider_receipt_sha256"] = receipt.ProviderReceiptHash
	}
	if receipt.ProviderStatus != "" {
		document["provider_status"] = receipt.ProviderStatus
	}
	if receipt.TerminalStatus != "" {
		document["terminal_status"] = receipt.TerminalStatus
	}
	if receipt.FailureCode != "" {
		document["failure_code"] = receipt.FailureCode
	}
	if !receipt.CallbackObservedAt.IsZero() {
		document["callback_observed_at"] = receipt.CallbackObservedAt.UTC().Format(time.RFC3339Nano)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(document)
}
