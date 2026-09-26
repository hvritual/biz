package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrCatalogUnavailable   = errors.New("notification: catalog unavailable")
	ErrCatalogEntryInvalid  = errors.New("notification: invalid catalog entry")
	ErrCatalogCodeDuplicate = errors.New("notification: duplicate catalog code")
	ErrCatalogQueryInvalid  = errors.New("notification: invalid catalog query")
)

// MessageLevel describes presentation priority, never an authorization or a
// personal-preference exemption. The order follows the source requirements.
type MessageLevel string

const (
	LevelUrgent        MessageLevel = "urgent"
	LevelImportant     MessageLevel = "important"
	LevelGeneral       MessageLevel = "general"
	MaxCatalogPageSize              = 100
)

func (level MessageLevel) Valid() bool {
	return level == LevelUrgent || level == LevelImportant || level == LevelGeneral
}

func messageLevels() []MessageLevel {
	return []MessageLevel{LevelUrgent, LevelImportant, LevelGeneral}
}

func levelRank(level MessageLevel) int {
	switch level {
	case LevelUrgent:
		return 0
	case LevelImportant:
		return 1
	default:
		return 2
	}
}

// MessageType is a registered subtype. Its code is unique across all levels.
// Names are supplied by the registration owner, not inferred from the code.
type MessageType struct {
	Code  string
	Name  string
	Level MessageLevel
}

// MessageTypeQuery uses literal, case-sensitive substring filters combined
// with AND. Percent and underscore have no wildcard meaning. Page is one-based;
// callers must choose an explicit PageSize rather than silently loading all rows.
type MessageTypeQuery struct {
	CodeContains string
	NameContains string
	Level        MessageLevel
	Page         int
	PageSize     int
}

func (query MessageTypeQuery) Validate() error {
	if query.Page < 1 || query.PageSize < 1 || query.PageSize > MaxCatalogPageSize ||
		(query.Level != "" && !query.Level.Valid()) ||
		!validFilter(query.CodeContains, 64) || !validFilter(query.NameContains, 256) {
		return ErrCatalogQueryInvalid
	}
	return nil
}

// LevelSummary counts the full filtered result, not only the visible page.
type LevelSummary struct {
	Level MessageLevel
	Total int
}

// MessageTypePage owns its result slices. Empty successful results have empty
// slices and three zero-count groups; unavailable catalogs return an error.
type MessageTypePage struct {
	Items    []MessageType
	Groups   []LevelSummary
	Total    int
	Page     int
	PageSize int
}

// MessageTypeCatalog is an immutable snapshot suitable for concurrent readers.
// Build a replacement snapshot to change registrations; there is no in-place
// mutation that can make a page and its total observe different registries.
type MessageTypeCatalog struct {
	ready bool
	types []MessageType
}

// NewMessageTypeCatalog distinguishes nil (no registration snapshot supplied)
// from an explicit empty slice (the authoritative catalog currently has no types).
// No partial catalog is returned if any registration is invalid.
func NewMessageTypeCatalog(registrations []MessageType) (*MessageTypeCatalog, error) {
	if registrations == nil {
		return nil, ErrCatalogUnavailable
	}
	types := make([]MessageType, len(registrations))
	copy(types, registrations)
	seen := make(map[string]struct{}, len(types))
	for _, entry := range types {
		if !validCode(entry.Code) || !validText(entry.Name, 256) || !entry.Level.Valid() {
			return nil, ErrCatalogEntryInvalid
		}
		if _, exists := seen[entry.Code]; exists {
			return nil, fmt.Errorf("%w: %s", ErrCatalogCodeDuplicate, entry.Code)
		}
		seen[entry.Code] = struct{}{}
	}
	sort.Slice(types, func(i, j int) bool {
		if types[i].Level != types[j].Level {
			return levelRank(types[i].Level) < levelRank(types[j].Level)
		}
		return types[i].Code < types[j].Code
	})
	return &MessageTypeCatalog{ready: true, types: types}, nil
}

// Lookup resolves one exact registered type code without substring semantics.
func (catalog *MessageTypeCatalog) Lookup(code string) (MessageType, error) {
	if catalog == nil || !catalog.ready {
		return MessageType{}, ErrCatalogUnavailable
	}
	if !validCode(code) {
		return MessageType{}, ErrCatalogEntryInvalid
	}
	for _, entry := range catalog.types {
		if entry.Code == code {
			return entry, nil
		}
	}
	return MessageType{}, ErrCatalogEntryInvalid
}

// List groups and counts the same immutable filtered snapshot before applying
// pagination. Sorting is deterministic: urgent, important, general, then code.
func (catalog *MessageTypeCatalog) List(query MessageTypeQuery) (MessageTypePage, error) {
	if catalog == nil || !catalog.ready {
		return MessageTypePage{}, ErrCatalogUnavailable
	}
	if err := query.Validate(); err != nil {
		return MessageTypePage{}, err
	}
	result := MessageTypePage{
		Items: make([]MessageType, 0), Groups: make([]LevelSummary, 0, 3),
		Page: query.Page, PageSize: query.PageSize,
	}
	for _, level := range messageLevels() {
		result.Groups = append(result.Groups, LevelSummary{Level: level})
	}
	matches := make([]MessageType, 0)
	for _, entry := range catalog.types {
		if (query.Level != "" && entry.Level != query.Level) ||
			!strings.Contains(entry.Code, query.CodeContains) ||
			!strings.Contains(entry.Name, query.NameContains) {
			continue
		}
		matches = append(matches, entry)
		result.Groups[levelRank(entry.Level)].Total++
	}
	result.Total = len(matches)
	if result.Total == 0 || query.Page-1 > result.Total/query.PageSize {
		return result, nil
	}
	start := (query.Page - 1) * query.PageSize
	if start >= result.Total {
		return result, nil
	}
	count := query.PageSize
	if count > result.Total-start {
		count = result.Total - start
	}
	result.Items = append(result.Items, matches[start:start+count]...)
	return result, nil
}

func validCode(value string) bool {
	if len(value) < 1 || len(value) > 64 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') &&
			char != '.' && char != '_' && char != '-' {
			return false
		}
	}
	return true
}

func validText(value string, maxRunes int) bool {
	return value != "" && strings.TrimSpace(value) == value && validFilter(value, maxRunes)
}

func validFilter(value string, maxRunes int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= maxRunes &&
		strings.IndexFunc(value, unicode.IsControl) < 0
}
