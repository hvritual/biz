package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func configurationRegistry(t *testing.T) *ChannelRegistry {
	t.Helper()
	r, e := NewChannelRegistry([]Channel{{Code: "in_app", Name: "站内", Availability: ChannelConfigurable}, {Code: "email", Name: "邮件", Availability: ChannelConfigurable}, {Code: "sms", Name: "短信", Availability: ChannelNotConfigurable, UnavailableReason: "渠道未接通"}})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func configurationValues() ConfigurationValues {
	return ConfigurationValues{Channels: []string{"in_app", "email"}, PrimaryUserID: "primary", SecondaryUserID: "secondary", AdditionalUserIDs: []string{"member-b", "member-a"}, Notes: "业务提醒\n检查设备"}
}
func TestConfigurationCanonicalOwnsValues(t *testing.T) {
	v := configurationValues()
	c, err := v.Canonical(configurationRegistry(t))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.Channels, []string{"email", "in_app"}) || !reflect.DeepEqual(c.AdditionalUserIDs, []string{"member-a", "member-b"}) {
		t.Fatalf("canonical %+v", c)
	}
	v.Channels[0] = "sms"
	v.AdditionalUserIDs[0] = "altered"
	if c.Channels[1] != "in_app" || c.AdditionalUserIDs[1] != "member-b" {
		t.Fatal("input alias")
	}
	got := c.RecipientIDs()
	got[0] = "changed"
	if c.RecipientIDs()[0] == "changed" {
		t.Fatal("output alias")
	}
}
func TestConfigurationRejectsInvalidRecipientsAndChannels(t *testing.T) {
	r := configurationRegistry(t)
	cases := []struct {
		name   string
		mutate func(*ConfigurationValues)
		want   error
	}{
		{"missing-primary", func(v *ConfigurationValues) { v.PrimaryUserID = "" }, ErrConfigurationInvalid},
		{"same-primary-secondary", func(v *ConfigurationValues) { v.SecondaryUserID = v.PrimaryUserID }, ErrConfigurationInvalid},
		{"extra-is-primary", func(v *ConfigurationValues) { v.AdditionalUserIDs = []string{v.PrimaryUserID} }, ErrConfigurationInvalid},
		{"extra-is-secondary", func(v *ConfigurationValues) { v.AdditionalUserIDs = []string{v.SecondaryUserID} }, ErrConfigurationInvalid},
		{"duplicate-extra", func(v *ConfigurationValues) { v.AdditionalUserIDs = []string{"same", "same"} }, ErrConfigurationInvalid},
		{"invalid-extra", func(v *ConfigurationValues) { v.AdditionalUserIDs = []string{"bad id"} }, ErrConfigurationInvalid},
		{"too-many-extra", func(v *ConfigurationValues) { v.AdditionalUserIDs = make([]string, 101) }, ErrConfigurationInvalid},
		{"empty-channels", func(v *ConfigurationValues) { v.Channels = nil }, ErrConfigurationInvalid},
		{"disabled", func(v *ConfigurationValues) { v.Channels = []string{"in_app", "sms"} }, ErrChannelUnavailable},
		{"unknown", func(v *ConfigurationValues) { v.Channels = []string{"unknown"} }, ErrChannelUnknown},
		{"duplicate-channel", func(v *ConfigurationValues) { v.Channels = []string{"email", "email"} }, ErrChannelSelectionDuplicate},
		{"notes-too-long", func(v *ConfigurationValues) { v.Notes = strings.Repeat("名", 1001) }, ErrConfigurationInvalid},
		{"notes-invalid-utf8", func(v *ConfigurationValues) { v.Notes = string([]byte{0xff}) }, ErrConfigurationInvalid},
		{"notes-control", func(v *ConfigurationValues) { v.Notes = "\x00" }, ErrConfigurationInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := configurationValues()
			tc.mutate(&v)
			out, err := v.Canonical(r)
			if !errors.Is(err, tc.want) || out.PrimaryUserID != "" {
				t.Fatalf("partial value=%+v err=%v want=%v", out, err, tc.want)
			}
		})
	}
	v := configurationValues()
	v.SecondaryUserID = ""
	v.AdditionalUserIDs = nil
	if _, err := v.Canonical(r); err != nil {
		t.Fatal("optional recipients rejected", err)
	}
	if _, err := v.Canonical(nil); !errors.Is(err, ErrCatalogUnavailable) {
		t.Fatal("missing catalog succeeded", err)
	}
}
func TestConfigurationLevelsAreDistinctAndOrdered(t *testing.T) {
	levels := []MessageLevel{LevelGeneral, LevelUrgent, LevelImportant}
	out, err := CanonicalConfigurationLevels(levels)
	if err != nil || !reflect.DeepEqual(out, messageLevels()) {
		t.Fatal(out, err)
	}
	out[0] = LevelGeneral
	if levels[0] != LevelGeneral || levels[1] != LevelUrgent {
		t.Fatal("input mutated")
	}
	for _, bad := range [][]MessageLevel{nil, {}, {LevelGeneral, LevelGeneral}, {"unknown"}, {LevelGeneral, LevelImportant, LevelUrgent, LevelGeneral}} {
		if _, err := CanonicalConfigurationLevels(bad); !errors.Is(err, ErrConfigurationInvalid) {
			t.Fatalf("accepted %v", bad)
		}
	}
}
func TestConfigurationRequestDigestBindsRawRequest(t *testing.T) {
	v := configurationValues()
	a, err := DigestConfigurationRequest(v)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := DigestConfigurationRequest(v)
	if a != b || len(a) != 64 {
		t.Fatal("unstable digest")
	}
	v.AdditionalUserIDs[0], v.AdditionalUserIDs[1] = v.AdditionalUserIDs[1], v.AdditionalUserIDs[0]
	c, _ := DigestConfigurationRequest(v)
	if c == a {
		t.Fatal("changed request accepted")
	}
	if _, err := DigestConfigurationRequest(make(chan int)); err == nil {
		t.Fatal("unserializable request accepted")
	}
}
func TestConfigurationFilterRequiresExplicitBounds(t *testing.T) {
	valid := ConfigurationFilter{GroupIDs: []string{"site-a"}, Page: 1, PageSize: 100}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, f := range []ConfigurationFilter{{}, {Page: -1, PageSize: 10}, {Page: 1000001, PageSize: 10}, {Page: 1, PageSize: 101}, {Page: 1, PageSize: 10, Level: "critical"}, {Page: 1, PageSize: 10, RecipientID: "bad id"}, {Page: 1, PageSize: 10, GroupIDs: []string{" "}}} {
		if f.Validate() == nil {
			t.Fatal("accepted invalid filter", f)
		}
	}
}
