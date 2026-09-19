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
	"strings"
	"unicode"
)

var (
	ErrSensitiveDataKeyUnavailable = errors.New("access: sensitive data key unavailable")
	ErrSensitiveDataCorrupt        = errors.New("access: sensitive data ciphertext invalid")
	ErrInvalidContact              = errors.New("access: invalid contact value")
)

// ContactProtection protects reversible contact values while preserving a
// deterministic, purpose-separated lookup index. Keys are runtime secrets and
// never persisted with the encrypted value.
type ContactProtection struct {
	activeVersion string
	keys          map[string][]byte
	lookupKey     []byte
}

type ContactProtectionConfig struct {
	ActiveVersion string
	Keys          map[string][]byte
	LookupKey     []byte
}

func NewContactProtection(config ContactProtectionConfig) (*ContactProtection, error) {
	active := strings.TrimSpace(config.ActiveVersion)
	if active == "" {
		return nil, ErrSensitiveDataKeyUnavailable
	}
	if len(config.LookupKey) < 32 {
		return nil, errors.New("access: sensitive contact lookup key must contain at least 32 bytes")
	}
	keys := make(map[string][]byte, len(config.Keys))
	for version, raw := range config.Keys {
		version = strings.TrimSpace(version)
		if version == "" || (len(raw) != 16 && len(raw) != 24 && len(raw) != 32) {
			return nil, errors.New("access: sensitive contact AES key must have a version and 16, 24, or 32 bytes")
		}
		keys[version] = append([]byte(nil), raw...)
	}
	if len(keys[active]) == 0 {
		return nil, fmt.Errorf("%w: active version %q", ErrSensitiveDataKeyUnavailable, active)
	}
	return &ContactProtection{activeVersion: active, keys: keys, lookupKey: append([]byte(nil), config.LookupKey...)}, nil
}

func (protection *ContactProtection) ActiveVersion() string {
	if protection == nil {
		return ""
	}
	return protection.activeVersion
}

func (protection *ContactProtection) ProtectEmail(value string) (ciphertext, lookup, version string, err error) {
	normalized, err := NormalizeEmail(value)
	if err != nil {
		return "", "", "", err
	}
	return protection.protect("email", normalized)
}

func (protection *ContactProtection) ProtectPhone(value string) (ciphertext, lookup, version string, err error) {
	normalized, err := NormalizePhone(value)
	if err != nil {
		return "", "", "", err
	}
	return protection.protect("phone", normalized)
}

func (protection *ContactProtection) LookupEmail(value string) (string, error) {
	normalized, err := NormalizeEmail(value)
	if err != nil {
		return "", err
	}
	return protection.lookup("email", normalized)
}

func (protection *ContactProtection) LookupPhone(value string) (string, error) {
	normalized, err := NormalizePhone(value)
	if err != nil {
		return "", err
	}
	return protection.lookup("phone", normalized)
}

func (protection *ContactProtection) DecryptEmail(ciphertext, version string) (string, error) {
	return protection.decrypt("email", ciphertext, version)
}

func (protection *ContactProtection) DecryptPhone(ciphertext, version string) (string, error) {
	return protection.decrypt("phone", ciphertext, version)
}

func (protection *ContactProtection) protect(purpose, value string) (string, string, string, error) {
	if protection == nil {
		return "", "", "", ErrSensitiveDataKeyUnavailable
	}
	key := protection.keys[protection.activeVersion]
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", "", "", err
	}
	aad := []byte("biz/contact/v1/" + purpose + "/" + protection.activeVersion)
	sealed := gcm.Seal(nil, nonce, []byte(value), aad)
	payload := append(nonce, sealed...)
	lookup, err := protection.lookup(purpose, value)
	if err != nil {
		return "", "", "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), lookup, protection.activeVersion, nil
}

func (protection *ContactProtection) decrypt(purpose, ciphertext, version string) (string, error) {
	if protection == nil {
		return "", ErrSensitiveDataKeyUnavailable
	}
	version = strings.TrimSpace(version)
	key := protection.keys[version]
	if len(key) == 0 {
		return "", fmt.Errorf("%w: version %q", ErrSensitiveDataKeyUnavailable, version)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(ciphertext))
	if err != nil {
		return "", ErrSensitiveDataCorrupt
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
		return "", ErrSensitiveDataCorrupt
	}
	aad := []byte("biz/contact/v1/" + purpose + "/" + version)
	plain, err := gcm.Open(nil, payload[:gcm.NonceSize()], payload[gcm.NonceSize():], aad)
	if err != nil {
		return "", ErrSensitiveDataCorrupt
	}
	return string(plain), nil
}

func (protection *ContactProtection) lookup(purpose, value string) (string, error) {
	if protection == nil || len(protection.lookupKey) < 32 {
		return "", ErrSensitiveDataKeyUnavailable
	}
	mac := hmac.New(sha256.New, protection.lookupKey)
	_, _ = mac.Write([]byte("biz/contact/lookup/v1/" + purpose + "\x00" + value))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func NormalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	at := strings.LastIndexByte(value, '@')
	if at <= 0 || at == len(value)-1 || len(value) > 320 || strings.ContainsAny(value, "\r\n\t ") {
		return "", ErrInvalidContact
	}
	return value, nil
}

func NormalizePhone(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	var builder strings.Builder
	for i, r := range value {
		switch {
		case unicode.IsDigit(r):
			builder.WriteRune(r)
		case r == '+' && i == 0:
			builder.WriteRune(r)
		case r == ' ' || r == '-' || r == '(' || r == ')':
			continue
		default:
			return "", ErrInvalidContact
		}
	}
	normalized := builder.String()
	digits := strings.TrimPrefix(normalized, "+")
	if len(digits) < 6 || len(digits) > 20 {
		return "", ErrInvalidContact
	}
	return normalized, nil
}

func MaskEmail(value string) string {
	value = strings.TrimSpace(value)
	at := strings.LastIndexByte(value, '@')
	if at <= 0 || at == len(value)-1 {
		return "***"
	}
	local, domain := value[:at], value[at+1:]
	if len([]rune(local)) == 1 {
		return local + "***@" + domain
	}
	runes := []rune(local)
	return string(runes[0]) + "***@" + domain
}

func MaskPhone(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	digits := []rune(strings.TrimPrefix(value, "+"))
	if len(digits) <= 4 {
		return "****"
	}
	prefix := ""
	if strings.HasPrefix(value, "+") {
		prefix = "+"
	}
	visibleLeft := 3
	if len(digits) < 7 {
		visibleLeft = 1
	}
	return prefix + string(digits[:visibleLeft]) + "****" + string(digits[len(digits)-4:])
}

func IsMaskedContact(value string) bool {
	return strings.Contains(value, "*")
}
