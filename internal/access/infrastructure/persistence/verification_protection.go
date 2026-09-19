package persistence

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/access/domain"
)

var (
	ErrVerificationKeyUnavailable = errors.New("access: verification key unavailable")
	ErrVerificationCipherCorrupt  = errors.New("access: verification ciphertext invalid")
)

type VerificationProtectionConfig struct {
	ActiveVersion string
	Keys          map[string][]byte
	HMACKey       []byte
}

type VerificationProtection struct {
	activeVersion string
	keys          map[string][]byte
	hmacKey       []byte
}

func NewVerificationProtection(config VerificationProtectionConfig) (*VerificationProtection, error) {
	active := strings.TrimSpace(config.ActiveVersion)
	if active == "" || len(config.HMACKey) < 32 {
		return nil, ErrVerificationKeyUnavailable
	}
	keys := make(map[string][]byte, len(config.Keys))
	for version, raw := range config.Keys {
		version = strings.TrimSpace(version)
		if version == "" || (len(raw) != 16 && len(raw) != 24 && len(raw) != 32) {
			return nil, errors.New("access: verification AES key must have a version and 16, 24, or 32 bytes")
		}
		keys[version] = append([]byte(nil), raw...)
	}
	if len(keys[active]) == 0 {
		return nil, fmt.Errorf("%w: active version %q", ErrVerificationKeyUnavailable, active)
	}
	return &VerificationProtection{activeVersion: active, keys: keys, hmacKey: append([]byte(nil), config.HMACKey...)}, nil
}

func (protection *VerificationProtection) ActiveVersion() string {
	if protection == nil {
		return ""
	}
	return protection.activeVersion
}

func (protection *VerificationProtection) DestinationHash(channel domain.SecurityNotificationChannel, destination string) (string, string, error) {
	normalized, err := normalizeVerificationDestination(channel, destination)
	if err != nil {
		return "", "", err
	}
	return protection.mac("destination/"+string(channel), normalized), normalized, nil
}

func (protection *VerificationProtection) MaskDestination(channel domain.SecurityNotificationChannel, normalized string) string {
	switch channel {
	case domain.SecurityNotificationEmail:
		return MaskEmail(normalized)
	case domain.SecurityNotificationSMS:
		return MaskPhone(normalized)
	default:
		return "***"
	}
}

func (protection *VerificationProtection) HashCode(challengeID, code string) string {
	return protection.mac("otp/"+strings.TrimSpace(challengeID), strings.TrimSpace(code))
}

func (protection *VerificationProtection) HashAuthorization(raw string) string {
	return protection.mac("authorization", strings.TrimSpace(raw))
}

func (protection *VerificationProtection) BindingHash(purpose domain.VerificationPurpose, userID, tenantID, flowID string, channel domain.SecurityNotificationChannel, destinationHash string) string {
	value := strings.Join([]string{
		string(purpose),
		strings.TrimSpace(userID),
		strings.TrimSpace(tenantID),
		strings.TrimSpace(flowID),
		string(channel),
		strings.TrimSpace(destinationHash),
	}, "\x00")
	return protection.mac("binding", value)
}

func (protection *VerificationProtection) NotificationBindingHash(kind domain.SecurityNotificationKind, purpose domain.VerificationPurpose, userID, tenantID, flowID string, channel domain.SecurityNotificationChannel, destinationHash, secret string, expiresAt time.Time) string {
	value := strings.Join([]string{
		string(kind),
		string(purpose),
		strings.TrimSpace(userID),
		strings.TrimSpace(tenantID),
		strings.TrimSpace(flowID),
		string(channel),
		strings.TrimSpace(destinationHash),
		strings.TrimSpace(secret),
		strconv.FormatInt(expiresAt.UTC().UnixMicro(), 10),
	}, "\x00")
	return protection.mac("notification-binding", value)
}

func (protection *VerificationProtection) ProtectNotification(eventID, field, value string) (ciphertext, version string, err error) {
	if protection == nil {
		return "", "", ErrVerificationKeyUnavailable
	}
	key := protection.keys[protection.activeVersion]
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", err
	}
	aad := []byte("biz/security-notification/v1/" + strings.TrimSpace(eventID) + "/" + strings.TrimSpace(field) + "/" + protection.activeVersion)
	sealed := gcm.Seal(nil, nonce, []byte(value), aad)
	payload := append(nonce, sealed...)
	return base64.RawURLEncoding.EncodeToString(payload), protection.activeVersion, nil
}

func (protection *VerificationProtection) DecryptNotification(eventID, field, ciphertext, version string) (string, error) {
	if protection == nil {
		return "", ErrVerificationKeyUnavailable
	}
	version = strings.TrimSpace(version)
	key := protection.keys[version]
	if len(key) == 0 {
		return "", fmt.Errorf("%w: version %q", ErrVerificationKeyUnavailable, version)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(ciphertext))
	if err != nil {
		return "", ErrVerificationCipherCorrupt
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(payload) <= gcm.NonceSize() {
		return "", ErrVerificationCipherCorrupt
	}
	aad := []byte("biz/security-notification/v1/" + strings.TrimSpace(eventID) + "/" + strings.TrimSpace(field) + "/" + version)
	plain, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], aad)
	if err != nil {
		return "", ErrVerificationCipherCorrupt
	}
	return string(plain), nil
}

func (protection *VerificationProtection) GenerateCode(digits int) (string, error) {
	if protection == nil || digits < 4 || digits > 10 {
		return "", domain.ErrVerificationInvalid
	}
	var builder strings.Builder
	builder.Grow(digits)
	ten := big.NewInt(10)
	for i := 0; i < digits; i++ {
		value, err := rand.Int(rand.Reader, ten)
		if err != nil {
			return "", err
		}
		builder.WriteByte(byte('0' + value.Int64()))
	}
	return builder.String(), nil
}

func randomVerificationSecret(size int) (string, error) {
	if size < 16 {
		return "", domain.ErrVerificationInvalid
	}
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func stableSecurityEventID(businessEventID string) string {
	sum := sha256.Sum256([]byte("biz/security-event/v1/" + strings.TrimSpace(businessEventID)))
	return "sec-" + base64.RawURLEncoding.EncodeToString(sum[:])[:43]
}

func normalizeVerificationDestination(channel domain.SecurityNotificationChannel, value string) (string, error) {
	switch channel {
	case domain.SecurityNotificationEmail:
		return NormalizeEmail(value)
	case domain.SecurityNotificationSMS:
		return NormalizePhone(value)
	default:
		return "", domain.ErrVerificationInvalid
	}
}

func (protection *VerificationProtection) mac(purpose, value string) string {
	if protection == nil || len(protection.hmacKey) < 32 {
		return ""
	}
	mac := hmac.New(sha256.New, protection.hmacKey)
	_, _ = mac.Write([]byte("biz/verification/v1/" + purpose + "\x00" + value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
