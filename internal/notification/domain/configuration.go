package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrConfigurationInvalid        = errors.New("notification: invalid configuration")
	ErrConfigurationConflict       = errors.New("notification: configuration version conflict")
	ErrConfigurationDuplicate      = errors.New("notification: duplicate group and level configuration")
	ErrConfigurationNotFound       = errors.New("notification: configuration not found")
	ErrConfigurationReplayConflict = errors.New("notification: idempotency key payload conflict")
	ErrConfigurationUnavailable    = errors.New("notification: configuration unavailable")
)

// Configuration is Notification-owned data. GroupID references a resource-domain
// site; user IDs reference Access members. It contains no contact destinations.
type Configuration struct {
	ID                string       `json:"id"`
	TenantID          string       `json:"tenant_id"`
	GroupID           string       `json:"group_id"`
	Level             MessageLevel `json:"level"`
	Channels          []string     `json:"channels"`
	PrimaryUserID     string       `json:"primary_user_id"`
	SecondaryUserID   string       `json:"secondary_user_id"`
	AdditionalUserIDs []string     `json:"additional_user_ids"`
	Notes             string       `json:"notes"`
	Version           uint64       `json:"version,string"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	Deleted           bool         `json:"deleted"`
}

// ConfigurationValues holds editable fields only. Identity, tenant, group and
// level are deliberately absent so an edit cannot move an existing record.
type ConfigurationValues struct {
	Channels          []string `json:"channels"`
	PrimaryUserID     string   `json:"primary_user_id"`
	SecondaryUserID   string   `json:"secondary_user_id"`
	AdditionalUserIDs []string `json:"additional_user_ids"`
	Notes             string   `json:"notes"`
}

// Canonical validates the complete input without silently removing duplicates.
// The caller must separately resolve current member and group authority inside
// the same root transaction; metadata validation is never authorization.
func (v ConfigurationValues) Canonical(registry *ChannelRegistry) (ConfigurationValues, error) {
	if err := v.Validate(); err != nil {
		return ConfigurationValues{}, err
	}
	selected, err := registry.ResolveSelection(v.Channels)
	if err != nil {
		return ConfigurationValues{}, err
	}
	out := ConfigurationValues{Channels: make([]string, 0, len(selected)), PrimaryUserID: v.PrimaryUserID, SecondaryUserID: v.SecondaryUserID, AdditionalUserIDs: make([]string, 0, len(v.AdditionalUserIDs)), Notes: v.Notes}
	for _, channel := range selected {
		out.Channels = append(out.Channels, channel.Code)
	}
	seen := map[string]bool{v.PrimaryUserID: true}
	if v.SecondaryUserID != "" {
		if seen[v.SecondaryUserID] {
			return ConfigurationValues{}, ErrConfigurationInvalid
		}
		seen[v.SecondaryUserID] = true
	}
	for _, id := range v.AdditionalUserIDs {
		if !ValidConfigurationID(id, 64) || seen[id] {
			return ConfigurationValues{}, ErrConfigurationInvalid
		}
		seen[id] = true
		out.AdditionalUserIDs = append(out.AdditionalUserIDs, id)
	}
	sort.Strings(out.AdditionalUserIDs)
	return out, nil
}

func (v ConfigurationValues) RecipientIDs() []string {
	ids := []string{v.PrimaryUserID}
	if v.SecondaryUserID != "" {
		ids = append(ids, v.SecondaryUserID)
	}
	ids = append(ids, v.AdditionalUserIDs...)
	sort.Strings(ids)
	return ids
}
func (c Configuration) Values() ConfigurationValues {
	return ConfigurationValues{Channels: append([]string{}, c.Channels...), PrimaryUserID: c.PrimaryUserID, SecondaryUserID: c.SecondaryUserID, AdditionalUserIDs: append([]string{}, c.AdditionalUserIDs...), Notes: c.Notes}
}

func CanonicalConfigurationLevels(levels []MessageLevel) ([]MessageLevel, error) {
	if len(levels) == 0 || len(levels) > 3 {
		return nil, ErrConfigurationInvalid
	}
	out := append([]MessageLevel{}, levels...)
	seen := map[MessageLevel]bool{}
	for _, level := range out {
		if !level.Valid() || seen[level] {
			return nil, ErrConfigurationInvalid
		}
		seen[level] = true
	}
	sort.Slice(out, func(i, j int) bool { return levelRank(out[i]) < levelRank(out[j]) })
	return out, nil
}

func ValidConfigurationID(value string, maxBytes int) bool {
	if value == "" || len(value) > maxBytes || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return false
		}
	}
	return true
}

// DigestConfigurationRequest binds the actual request, including collection
// order and expected version. Reusing a key with a changed request is a conflict.
func DigestConfigurationRequest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

type ConfigurationReceipt struct {
	ReceiptID      string          `json:"receipt_id"`
	TenantID       string          `json:"tenant_id"`
	Configurations []Configuration `json:"configurations"`
}

type ConfigurationFilter struct {
	GroupIDs    []string
	Level       MessageLevel
	RecipientID string
	Page        int
	PageSize    int
}

func (f ConfigurationFilter) Validate() error {
	if f.Page < 1 || f.Page > 1000000 || f.PageSize < 1 || f.PageSize > 100 ||
		(f.Level != "" && !f.Level.Valid()) || (f.RecipientID != "" && !ValidConfigurationID(f.RecipientID, 64)) {
		return ErrConfigurationInvalid
	}
	for _, id := range f.GroupIDs {
		if !ValidConfigurationID(id, 64) {
			return ErrConfigurationInvalid
		}
	}
	return nil
}

// Validate checks shape only; callers still require Canonical against a current
// channel registry and the trusted recipient directory before any business write.
func (v ConfigurationValues) Validate() error {
	if len(v.Channels) == 0 || len(v.Channels) > 3 || !ValidConfigurationID(v.PrimaryUserID, 64) ||
		(v.SecondaryUserID != "" && !ValidConfigurationID(v.SecondaryUserID, 64)) || len(v.AdditionalUserIDs) > 100 ||
		!utf8.ValidString(v.Notes) || utf8.RuneCountInString(v.Notes) > 1000 ||
		strings.IndexFunc(v.Notes, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' }) >= 0 {
		return ErrConfigurationInvalid
	}
	seenChannels := map[string]bool{}
	for _, code := range v.Channels {
		if !validCode(code) {
			return ErrCatalogEntryInvalid
		}
		if seenChannels[code] {
			return ErrChannelSelectionDuplicate
		}
		seenChannels[code] = true
	}
	seen := map[string]bool{v.PrimaryUserID: true}
	if v.SecondaryUserID != "" {
		if seen[v.SecondaryUserID] {
			return ErrConfigurationInvalid
		}
		seen[v.SecondaryUserID] = true
	}
	for _, id := range v.AdditionalUserIDs {
		if !ValidConfigurationID(id, 64) || seen[id] {
			return ErrConfigurationInvalid
		}
		seen[id] = true
	}
	return nil
}
