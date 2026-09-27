package domain

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// Channel names here are test-owned and do not approve any production channel.
func fixtureChannels() []Channel {
	return []Channel{
		{Code: "fixture-b", Name: "渠道乙", Availability: ChannelConfigurable},
		{Code: "fixture-off", Name: "渠道停用", Availability: ChannelNotConfigurable, UnavailableReason: "测试目录停用"},
		{Code: "fixture-a", Name: "渠道甲", Availability: ChannelConfigurable},
	}
}

func newChannelRegistry(t *testing.T, channels []Channel) *ChannelRegistry {
	t.Helper()
	registry, err := NewChannelRegistry(channels)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func TestChannelRegistryHasNoImplicitDefaults(t *testing.T) {
	registry, err := NewChannelRegistry(nil)
	if !errors.Is(err, ErrCatalogUnavailable) || registry != nil {
		t.Fatalf("missing snapshot: %v %v", registry, err)
	}
	for _, unavailable := range []*ChannelRegistry{nil, {}} {
		if _, err := unavailable.List(); !errors.Is(err, ErrCatalogUnavailable) {
			t.Fatal("unavailable list succeeded")
		}
		if _, err := unavailable.ResolveSelection(nil); !errors.Is(err, ErrCatalogUnavailable) {
			t.Fatal("unavailable selection succeeded")
		}
	}
	empty := newChannelRegistry(t, []Channel{})
	listed, err := empty.List()
	if err != nil || listed == nil || len(listed) != 0 {
		t.Fatalf("explicit empty: %v %v", listed, err)
	}
	for _, code := range []string{"sms", "email", "in_app"} {
		if _, err := empty.ResolveSelection([]string{code}); !errors.Is(err, ErrChannelUnknown) {
			t.Fatalf("invented default %s: %v", code, err)
		}
	}
}

func TestChannelRegistryRejectsInvalidRegistrationsAtomically(t *testing.T) {
	valid := fixtureChannels()[0]
	cases := []struct {
		name    string
		channel Channel
		want    error
	}{
		{"no-code", Channel{Name: "name", Availability: ChannelConfigurable}, ErrCatalogEntryInvalid},
		{"uppercase-alias", Channel{Code: "FIXTURE", Name: "name", Availability: ChannelConfigurable}, ErrCatalogEntryInvalid},
		{"unicode-code", Channel{Code: "渠道", Name: "name", Availability: ChannelConfigurable}, ErrCatalogEntryInvalid},
		{"empty-name", Channel{Code: "fixture-new", Availability: ChannelConfigurable}, ErrCatalogEntryInvalid},
		{"invalid-utf8", Channel{Code: "fixture-new", Name: string([]byte{0xff}), Availability: ChannelConfigurable}, ErrCatalogEntryInvalid},
		{"missing-availability", Channel{Code: "fixture-new", Name: "name"}, ErrCatalogEntryInvalid},
		{"unknown-availability", Channel{Code: "fixture-new", Name: "name", Availability: "sent"}, ErrCatalogEntryInvalid},
		{"unavailable-without-reason", Channel{Code: "fixture-new", Name: "name", Availability: ChannelNotConfigurable}, ErrCatalogEntryInvalid},
		{"unavailable-blank-reason", Channel{Code: "fixture-new", Name: "name", Availability: ChannelNotConfigurable, UnavailableReason: "  "}, ErrCatalogEntryInvalid},
		{"unavailable-control-reason", Channel{Code: "fixture-new", Name: "name", Availability: ChannelNotConfigurable, UnavailableReason: "x\ny"}, ErrCatalogEntryInvalid},
		{"unavailable-long-reason", Channel{Code: "fixture-new", Name: "name", Availability: ChannelNotConfigurable, UnavailableReason: strings.Repeat("名", 257)}, ErrCatalogEntryInvalid},
		{"available-with-unavailable-reason", Channel{Code: "fixture-new", Name: "name", Availability: ChannelConfigurable, UnavailableReason: "disabled"}, ErrCatalogEntryInvalid},
		{"duplicate", valid, ErrCatalogCodeDuplicate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := NewChannelRegistry([]Channel{valid, tc.channel})
			if !errors.Is(err, tc.want) || registry != nil {
				t.Fatalf("partial registry=%v err=%v want=%v", registry, err, tc.want)
			}
		})
	}
}

func TestChannelRegistryListsUnavailableChannelsWithReasons(t *testing.T) {
	channels, err := newChannelRegistry(t, fixtureChannels()).List()
	if err != nil || len(channels) != 3 {
		t.Fatalf("list=%v err=%v", channels, err)
	}
	if channels[0].Code != "fixture-a" || channels[1].Code != "fixture-b" ||
		channels[2].Availability != ChannelNotConfigurable || channels[2].UnavailableReason == "" {
		t.Fatalf("lost catalog order or unavailable explanation: %+v", channels)
	}
}

func TestChannelSelectionIsAtomicAndDoesNotNormalizeOrDropEntries(t *testing.T) {
	registry := newChannelRegistry(t, fixtureChannels())
	cases := []struct {
		name  string
		codes []string
		want  error
	}{
		{"unknown", []string{"fixture-a", "missing"}, ErrChannelUnknown},
		{"disabled", []string{"fixture-a", "fixture-off"}, ErrChannelUnavailable},
		{"duplicate", []string{"fixture-a", "fixture-a"}, ErrChannelSelectionDuplicate},
		{"uppercase", []string{"FIXTURE-A"}, ErrCatalogEntryInvalid},
		{"space", []string{"fixture-a "}, ErrCatalogEntryInvalid},
		{"empty-code", []string{""}, ErrCatalogEntryInvalid},
		{"unicode", []string{"渠道甲"}, ErrCatalogEntryInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selected, err := registry.ResolveSelection(tc.codes)
			if !errors.Is(err, tc.want) || selected != nil {
				t.Fatalf("partial selection=%v err=%v want=%v", selected, err, tc.want)
			}
		})
	}
	selected, err := registry.ResolveSelection([]string{"fixture-b", "fixture-a"})
	if err != nil || len(selected) != 2 || selected[0].Code != "fixture-a" || selected[1].Code != "fixture-b" {
		t.Fatalf("valid selection=%v err=%v", selected, err)
	}
}

func TestEmptyChannelSelectionDoesNotInventConfigurationPolicy(t *testing.T) {
	registry := newChannelRegistry(t, fixtureChannels())
	for _, input := range [][]string{nil, {}} {
		selected, err := registry.ResolveSelection(input)
		if err != nil || selected == nil || len(selected) != 0 {
			t.Fatalf("empty selection=%v err=%v", selected, err)
		}
	}
}

func TestChannelRegistryOwnsInputsAndReturnedValues(t *testing.T) {
	input := fixtureChannels()
	before := append([]Channel{}, input...)
	registry := newChannelRegistry(t, input)
	if !reflect.DeepEqual(input, before) {
		t.Fatal("constructor sorted caller slice")
	}
	input[0].Availability = ChannelNotConfigurable
	input[0].Code = "untrusted"
	listed, err := registry.List()
	if err != nil {
		t.Fatal(err)
	}
	listed[0].Availability = ChannelNotConfigurable
	selected, err := registry.ResolveSelection([]string{"fixture-a", "fixture-b"})
	if err != nil || len(selected) != 2 {
		t.Fatalf("external mutation changed lookup: %v", err)
	}
	selected[0].Name = "changed"
	again, err := registry.List()
	if err != nil || again[0].Name != "渠道甲" || again[0].Availability != ChannelConfigurable {
		t.Fatalf("result alias: %v %v", again, err)
	}
}

func TestReplacementChannelSnapshotDoesNotChangeOldReaders(t *testing.T) {
	registrations := fixtureChannels()
	old := newChannelRegistry(t, registrations)
	registrations[0].Availability = ChannelNotConfigurable
	registrations[0].UnavailableReason = "新目录停用"
	current := newChannelRegistry(t, registrations)
	if _, err := old.ResolveSelection([]string{"fixture-b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := current.ResolveSelection([]string{"fixture-b"}); !errors.Is(err, ErrChannelUnavailable) {
		t.Fatal("new snapshot did not apply availability")
	}
	// Configuration writes and delivery must acquire the current snapshot;
	// snapshot isolation is not permission to keep using an obsolete registry.
}

func TestChannelRegistryConcurrentReadersRemainIsolated(t *testing.T) {
	registry := newChannelRegistry(t, fixtureChannels())
	var readers sync.WaitGroup
	for i := 0; i < 16; i++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for j := 0; j < 25; j++ {
				list, err := registry.List()
				if err != nil || len(list) != 3 {
					t.Errorf("list: %v", err)
					return
				}
				list[0].Name = "reader copy"
				selected, err := registry.ResolveSelection([]string{"fixture-a"})
				if err != nil || len(selected) != 1 || selected[0].Name != "渠道甲" {
					t.Errorf("selection: %v %v", selected, err)
					return
				}
				selected[0].Name = "private"
			}
		}()
	}
	readers.Wait()
}
