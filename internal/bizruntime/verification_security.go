package bizruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	accessapp "github.com/hvritual/biz/internal/access/application"
	"github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/access/ports"
	"gorm.io/gorm"
)

type VerificationSecurityConfig struct {
	CodeTTL              time.Duration
	AuthorizationTTL     time.Duration
	ResendInterval       time.Duration
	SendLimitWindow      time.Duration
	MaxSendsPerWindow    int
	MaxVerificationTries int
	CodeDigits           int
	Notification         SecurityNotificationProviderConfig
}

func (config VerificationSecurityConfig) Enabled() bool {
	return config.CodeTTL != 0 || config.AuthorizationTTL != 0 || config.ResendInterval != 0 ||
		config.SendLimitWindow != 0 || config.MaxSendsPerWindow != 0 || config.MaxVerificationTries != 0 ||
		config.CodeDigits != 0 || config.Notification.Enabled()
}

func (config VerificationSecurityConfig) Policy() domain.VerificationPolicy {
	return domain.VerificationPolicy{
		CodeTTL: config.CodeTTL, AuthorizationTTL: config.AuthorizationTTL,
		ResendInterval: config.ResendInterval, SendLimitWindow: config.SendLimitWindow,
		MaxSendsPerWindow: config.MaxSendsPerWindow, MaxVerificationTries: config.MaxVerificationTries,
		CodeDigits: config.CodeDigits,
	}
}

func (config VerificationSecurityConfig) Validate() error {
	if !config.Enabled() {
		return nil
	}
	if err := config.Policy().Validate(); err != nil {
		return err
	}
	return config.Notification.Validate()
}

type SecurityNotificationProviderConfig struct {
	Provider      string
	Endpoint      string
	SenderID      string
	CredentialRef string
}

func (config SecurityNotificationProviderConfig) Enabled() bool {
	return strings.TrimSpace(config.Provider) != ""
}

func (config SecurityNotificationProviderConfig) Validate() error {
	provider := strings.TrimSpace(config.Provider)
	if provider == "" {
		return errors.New("biz runtime: security notification provider must be explicitly configured")
	}
	if provider == "disabled" {
		if strings.TrimSpace(config.Endpoint) != "" || strings.TrimSpace(config.CredentialRef) != "" {
			return errors.New("biz runtime: disabled security notification provider must not carry endpoint or credential reference")
		}
		return nil
	}
	if strings.TrimSpace(config.Endpoint) == "" || strings.TrimSpace(config.SenderID) == "" || strings.TrimSpace(config.CredentialRef) == "" {
		return errors.New("biz runtime: external security notification provider requires endpoint, sender id and credential reference")
	}
	parsed, err := url.Parse(strings.TrimSpace(config.Endpoint))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !(parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname()))) {
		return errors.New("biz runtime: security notification endpoint must use HTTPS or loopback HTTP")
	}
	return nil
}

func BuildVerificationProtection(activeVersion, keysJSON, hmacKeyBase64 string) (*accesspersistence.VerificationProtection, error) {
	activeVersion = strings.TrimSpace(activeVersion)
	keysJSON = strings.TrimSpace(keysJSON)
	hmacKeyBase64 = strings.TrimSpace(hmacKeyBase64)
	if activeVersion == "" && keysJSON == "" && hmacKeyBase64 == "" {
		return nil, nil
	}
	if activeVersion == "" || keysJSON == "" || hmacKeyBase64 == "" {
		return nil, errors.New("biz runtime: verification active version, key set and HMAC key must be configured together")
	}
	var encoded map[string]string
	if err := json.Unmarshal([]byte(keysJSON), &encoded); err != nil || len(encoded) == 0 {
		return nil, errors.New("biz runtime: verification key set must be a non-empty JSON object")
	}
	keys := make(map[string][]byte, len(encoded))
	for version, raw := range encoded {
		version = strings.TrimSpace(version)
		if version == "" {
			return nil, errors.New("biz runtime: verification key version must not be empty")
		}
		decoded, err := decodePIISecret(raw)
		if err != nil {
			return nil, fmt.Errorf("biz runtime: decode verification key %q: %w", version, err)
		}
		keys[version] = decoded
	}
	hmacKey, err := decodePIISecret(hmacKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("biz runtime: decode verification HMAC key: %w", err)
	}
	return accesspersistence.NewVerificationProtection(accesspersistence.VerificationProtectionConfig{
		ActiveVersion: activeVersion,
		Keys:          keys,
		HMACKey:       hmacKey,
	})
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func BuildVerificationService(database *gorm.DB, config VerificationSecurityConfig, protection *accesspersistence.VerificationProtection, sender ports.SecurityNotificationSender) (*accessapp.VerificationService, error) {
	if !config.Enabled() {
		return nil, nil
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if protection == nil {
		return nil, accesspersistence.ErrVerificationKeyUnavailable
	}
	repository, err := accesspersistence.NewVerificationRepository(database, protection)
	if err != nil {
		return nil, err
	}
	return accessapp.NewVerificationService(repository, sender, config.Policy())
}
