package domain

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// These registrations are fixtures, not a production message-type inventory.
func fixtureTypes() []MessageType {
	return []MessageType{
		{Code: "fixture.general", Name: "库存提醒", Level: LevelGeneral},
		{Code: "fixture.urgent-b", Name: "设备检查", Level: LevelUrgent},
		{Code: "fixture.important", Name: "设备提醒", Level: LevelImportant},
		{Code: "fixture.urgent-a", Name: "设备提醒_100%", Level: LevelUrgent},
	}
}

func newTypeCatalog(t *testing.T, entries []MessageType) *MessageTypeCatalog {
	t.Helper()
	catalog, err := NewMessageTypeCatalog(entries)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func TestMessageTypeLevelsHaveFixedPresentationOrder(t *testing.T) {
	for _, level := range messageLevels() {
		if !level.Valid() {
			t.Fatalf("invalid built-in level: %q", level)
		}
	}
	for _, level := range []MessageLevel{"", "critical", "URGENT", "urgent "} {
		if level.Valid() {
			t.Fatalf("unknown level accepted: %q", level)
		}
	}
	page, err := newTypeCatalog(t, fixtureTypes()).List(MessageTypeQuery{Page: 1, PageSize: 100})
	want := []LevelSummary{{LevelUrgent, 2}, {LevelImportant, 1}, {LevelGeneral, 1}}
	if err != nil || !reflect.DeepEqual(page.Groups, want) || page.Total != 4 {
		t.Fatalf("groups=%+v total=%d err=%v", page.Groups, page.Total, err)
	}
	codes := []string{}
	for _, entry := range page.Items {
		codes = append(codes, entry.Code)
	}
	if !reflect.DeepEqual(codes, []string{"fixture.urgent-a", "fixture.urgent-b", "fixture.important", "fixture.general"}) {
		t.Fatalf("unexpected order: %v", codes)
	}
}

func TestMessageTypeCatalogDistinguishesUnavailableAndEmpty(t *testing.T) {
	catalog, err := NewMessageTypeCatalog(nil)
	if !errors.Is(err, ErrCatalogUnavailable) || catalog != nil {
		t.Fatalf("missing registrations: catalog=%v err=%v", catalog, err)
	}
	for _, unavailable := range []*MessageTypeCatalog{nil, {}} {
		if _, err := unavailable.List(MessageTypeQuery{Page: 1, PageSize: 10}); !errors.Is(err, ErrCatalogUnavailable) {
			t.Fatalf("uninitialized catalog became successful empty: %v", err)
		}
	}
	page, err := newTypeCatalog(t, []MessageType{}).List(MessageTypeQuery{Page: 1, PageSize: 10})
	if err != nil || page.Total != 0 || page.Items == nil || len(page.Items) != 0 || len(page.Groups) != 3 {
		t.Fatalf("explicit empty snapshot: %+v err=%v", page, err)
	}
}

func TestMessageTypeCatalogRejectsInvalidAndDuplicateRegistrationsAtomically(t *testing.T) {
	valid := fixtureTypes()[0]
	cases := []struct {
		name  string
		entry MessageType
		want  error
	}{
		{"empty-code", MessageType{Name: "name", Level: LevelGeneral}, ErrCatalogEntryInvalid},
		{"uppercase-alias", MessageType{Code: "Fixture.type", Name: "name", Level: LevelGeneral}, ErrCatalogEntryInvalid},
		{"leading-space", MessageType{Code: " fixture.type", Name: "name", Level: LevelGeneral}, ErrCatalogEntryInvalid},
		{"code-control", MessageType{Code: "fixture.type\n", Name: "name", Level: LevelGeneral}, ErrCatalogEntryInvalid},
		{"code-nonascii", MessageType{Code: "类型", Name: "name", Level: LevelGeneral}, ErrCatalogEntryInvalid},
		{"code-too-long", MessageType{Code: strings.Repeat("a", 65), Name: "name", Level: LevelGeneral}, ErrCatalogEntryInvalid},
		{"empty-name", MessageType{Code: "fixture.x", Level: LevelGeneral}, ErrCatalogEntryInvalid},
		{"blank-name", MessageType{Code: "fixture.x", Name: "  ", Level: LevelGeneral}, ErrCatalogEntryInvalid},
		{"name-control", MessageType{Code: "fixture.x", Name: "x\ny", Level: LevelGeneral}, ErrCatalogEntryInvalid},
		{"name-utf8", MessageType{Code: "fixture.x", Name: string([]byte{0xff}), Level: LevelGeneral}, ErrCatalogEntryInvalid},
		{"name-too-long", MessageType{Code: "fixture.x", Name: strings.Repeat("名", 257), Level: LevelGeneral}, ErrCatalogEntryInvalid},
		{"unknown-level", MessageType{Code: "fixture.x", Name: "name", Level: "critical"}, ErrCatalogEntryInvalid},
		{"duplicate-same-level", valid, ErrCatalogCodeDuplicate},
		{"duplicate-other-level", MessageType{Code: valid.Code, Name: "different", Level: LevelUrgent}, ErrCatalogCodeDuplicate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			catalog, err := NewMessageTypeCatalog([]MessageType{valid, tc.entry})
			if !errors.Is(err, tc.want) || catalog != nil {
				t.Fatalf("partial/invalid registry returned: %v err=%v want=%v", catalog, err, tc.want)
			}
		})
	}
	newTypeCatalog(t, []MessageType{{Code: strings.Repeat("a", 64), Name: strings.Repeat("名", 256), Level: LevelGeneral}})
}

func TestMessageTypeFiltersUseANDAndCountBeforePagination(t *testing.T) {
	catalog := newTypeCatalog(t, fixtureTypes())
	cases := []struct {
		name  string
		query MessageTypeQuery
		total int
		codes []string
	}{
		{"name", MessageTypeQuery{NameContains: "设备", Page: 1, PageSize: 1}, 3, []string{"fixture.urgent-a"}},
		{"code", MessageTypeQuery{CodeContains: "urgent", Page: 2, PageSize: 1}, 2, []string{"fixture.urgent-b"}},
		{"level", MessageTypeQuery{Level: LevelGeneral, Page: 1, PageSize: 2}, 1, []string{"fixture.general"}},
		{"combined", MessageTypeQuery{CodeContains: "urgent", NameContains: "提醒", Level: LevelUrgent, Page: 1, PageSize: 10}, 1, []string{"fixture.urgent-a"}},
		{"incompatible", MessageTypeQuery{CodeContains: "urgent", Level: LevelGeneral, Page: 1, PageSize: 10}, 0, []string{}},
		{"literal-percent", MessageTypeQuery{NameContains: "%", Page: 1, PageSize: 10}, 1, []string{"fixture.urgent-a"}},
		{"literal-underscore", MessageTypeQuery{NameContains: "_", Page: 1, PageSize: 10}, 1, []string{"fixture.urgent-a"}},
		{"case-sensitive", MessageTypeQuery{CodeContains: "URGENT", Page: 1, PageSize: 10}, 0, []string{}},
		{"zero-matches", MessageTypeQuery{NameContains: "未注册", Page: 1, PageSize: 10}, 0, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := catalog.List(tc.query)
			if err != nil || result.Total != tc.total {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			codes, groupTotal := []string{}, 0
			for _, item := range result.Items {
				codes = append(codes, item.Code)
			}
			for _, group := range result.Groups {
				groupTotal += group.Total
			}
			if !reflect.DeepEqual(codes, tc.codes) || groupTotal != result.Total || result.Items == nil {
				t.Fatalf("page/filter/count mismatch: %+v", result)
			}
		})
	}
}

func TestMessageTypePaginationRejectsInvalidAndHandlesHugePages(t *testing.T) {
	catalog := newTypeCatalog(t, fixtureTypes())
	queries := []MessageTypeQuery{
		{}, {Page: -1, PageSize: 1}, {Page: 1, PageSize: -1},
		{Page: 1, PageSize: MaxCatalogPageSize + 1}, {Page: 1, PageSize: 1, Level: "critical"},
		{Page: 1, PageSize: 1, CodeContains: strings.Repeat("a", 65)},
		{Page: 1, PageSize: 1, NameContains: strings.Repeat("名", 257)},
		{Page: 1, PageSize: 1, NameContains: "x\ny"},
		{Page: 1, PageSize: 1, CodeContains: string([]byte{0xff})},
	}
	for _, query := range queries {
		if _, err := catalog.List(query); !errors.Is(err, ErrCatalogQueryInvalid) {
			t.Fatalf("invalid query accepted: %+v err=%v", query, err)
		}
	}
	for _, page := range []int{3, 1000, int(^uint(0) >> 1)} {
		result, err := catalog.List(MessageTypeQuery{Page: page, PageSize: 2})
		if err != nil || len(result.Items) != 0 || result.Total != 4 || result.Page != page {
			t.Fatalf("out-of-range query: %+v err=%v", result, err)
		}
	}
}

func TestMessageTypePaginationTraversesAllResultsOnce(t *testing.T) {
	entries := []MessageType{}
	for i := 0; i < 205; i++ {
		entries = append(entries, MessageType{Code: fmt.Sprintf("fixture.%03d", i), Name: "目录项", Level: messageLevels()[i%3]})
	}
	catalog := newTypeCatalog(t, entries)
	seen := map[string]bool{}
	for page := 1; page <= 4; page++ {
		result, err := catalog.List(MessageTypeQuery{Page: page, PageSize: 100})
		if err != nil || result.Total != 205 {
			t.Fatalf("page=%d result=%+v err=%v", page, result, err)
		}
		for _, entry := range result.Items {
			if seen[entry.Code] {
				t.Fatalf("duplicate across pages: %s", entry.Code)
			}
			seen[entry.Code] = true
		}
	}
	if len(seen) != 205 {
		t.Fatalf("pagination omitted entries: %d", len(seen))
	}
}

func TestMessageTypeCatalogOwnsSnapshotsAndResults(t *testing.T) {
	entries := fixtureTypes()
	before := append([]MessageType{}, entries...)
	catalog := newTypeCatalog(t, entries)
	if !reflect.DeepEqual(entries, before) {
		t.Fatal("constructor sorted the caller's slice")
	}
	entries[0].Name = "changed by caller"
	query := MessageTypeQuery{Page: 1, PageSize: 100}
	first, err := catalog.List(query)
	if err != nil {
		t.Fatal(err)
	}
	first.Items[0].Name = "changed result"
	first.Groups[0].Total = 999
	first.Items = append(first.Items, MessageType{Code: "injected"})
	second, err := catalog.List(query)
	want := newTypeCatalog(t, before)
	expected, expectedErr := want.List(query)
	if err != nil || expectedErr != nil || !reflect.DeepEqual(second, expected) {
		t.Fatalf("snapshot alias: %+v err=%v", second, err)
	}
	reversed := append([]MessageType{}, before...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	reordered, err := newTypeCatalog(t, reversed).List(query)
	if err != nil || !reflect.DeepEqual(second, reordered) {
		t.Fatal("registration order changes results")
	}
}

func TestMessageTypeCatalogConcurrentReadersRemainIsolated(t *testing.T) {
	catalog := newTypeCatalog(t, fixtureTypes())
	var readers sync.WaitGroup
	for i := 0; i < 16; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for j := 0; j < 25; j++ {
				page, err := catalog.List(MessageTypeQuery{Page: 1, PageSize: 1})
				if err != nil || page.Total != 4 || len(page.Items) != 1 || page.Groups[0].Total != 2 {
					t.Errorf("concurrent read mismatch: %+v err=%v", page, err)
					return
				}
				page.Items[0].Name = "private copy"
				page.Groups[0].Total = 99
			}
		}()
	}
	readers.Wait()
}

func FuzzMessageTypeCatalogPagination(f *testing.F) {
	f.Add(1, 1, "设备", "")
	f.Add(3, 2, "", "urgent")
	f.Add(int(^uint(0)>>1), 100, "%", "")
	catalog, err := NewMessageTypeCatalog(fixtureTypes())
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, page, size int, name, code string) {
		query := MessageTypeQuery{Page: page, PageSize: size, NameContains: name, CodeContains: code}
		result, err := catalog.List(query)
		if query.Validate() != nil {
			if !errors.Is(err, ErrCatalogQueryInvalid) {
				t.Fatalf("invalid query returned %v", err)
			}
			return
		}
		if err != nil || len(result.Groups) != 3 || result.Items == nil {
			t.Fatalf("valid query failed: %+v %v", result, err)
		}
		count := 0
		for _, group := range result.Groups {
			count += group.Total
		}
		if count != result.Total || result.Total > 4 || len(result.Items) > size || len(result.Items) > result.Total {
			t.Fatalf("inconsistent pagination: %+v", result)
		}
	})
}
