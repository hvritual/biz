package domain

import (
	"errors"
	"fmt"
	"sort"
)

var (
	ErrChannelUnknown            = errors.New("notification: unknown channel")
	ErrChannelUnavailable        = errors.New("notification: channel unavailable for configuration")
	ErrChannelSelectionDuplicate = errors.New("notification: duplicate channel selection")
)

// ChannelAvailability is configuration metadata only. Configurable does not
// mean a provider is healthy, a user opted in, or a notification was delivered.
type ChannelAvailability string

const (
	ChannelConfigurable    ChannelAvailability = "configurable"
	ChannelNotConfigurable ChannelAvailability = "unavailable"
)

// Channel describes an explicit server registration. This package deliberately
// provides no default production channel list or security-message exemption.
type Channel struct {
	Code              string
	Name              string
	Availability      ChannelAvailability
	UnavailableReason string
}

func (channel Channel) validate() error {
	if !validCode(channel.Code) || !validText(channel.Name, 256) {
		return ErrCatalogEntryInvalid
	}
	switch channel.Availability {
	case ChannelConfigurable:
		if channel.UnavailableReason != "" {
			return ErrCatalogEntryInvalid
		}
	case ChannelNotConfigurable:
		if !validText(channel.UnavailableReason, 256) {
			return ErrCatalogEntryInvalid
		}
	default:
		return ErrCatalogEntryInvalid
	}
	return nil
}

// ChannelRegistry is a complete immutable registration snapshot. Trusted
// composition must supply approved registrations; it must not build this
// registry from a browser request or use it as a grant/recipient authority.
type ChannelRegistry struct {
	ready    bool
	channels []Channel
	byCode   map[string]Channel
}

func NewChannelRegistry(registrations []Channel) (*ChannelRegistry, error) {
	if registrations == nil {
		return nil, ErrCatalogUnavailable
	}
	channels := make([]Channel, len(registrations))
	copy(channels, registrations)
	byCode := make(map[string]Channel, len(channels))
	for _, channel := range channels {
		if err := channel.validate(); err != nil {
			return nil, err
		}
		if _, exists := byCode[channel.Code]; exists {
			return nil, fmt.Errorf("%w: %s", ErrCatalogCodeDuplicate, channel.Code)
		}
		byCode[channel.Code] = channel
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].Code < channels[j].Code })
	return &ChannelRegistry{ready: true, channels: channels, byCode: byCode}, nil
}

// List includes unavailable registered channels with their reason. A consumer
// can explain why a channel cannot be selected without inventing a new channel.
func (registry *ChannelRegistry) List() ([]Channel, error) {
	if registry == nil || !registry.ready {
		return nil, ErrCatalogUnavailable
	}
	channels := make([]Channel, len(registry.channels))
	copy(channels, registry.channels)
	return channels, nil
}

// ResolveSelection validates the entire selection against this snapshot before
// returning any result. It does not silently drop unavailable or unknown codes.
// Empty selection is structurally valid here; minimum-recipient/channel rules
// belong to the approved configuration policy, not this metadata registry.
func (registry *ChannelRegistry) ResolveSelection(codes []string) ([]Channel, error) {
	if registry == nil || !registry.ready {
		return nil, ErrCatalogUnavailable
	}
	selected := make([]Channel, 0, len(codes))
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if !validCode(code) {
			return nil, ErrCatalogEntryInvalid
		}
		if _, exists := seen[code]; exists {
			return nil, fmt.Errorf("%w: %s", ErrChannelSelectionDuplicate, code)
		}
		seen[code] = struct{}{}
		channel, exists := registry.byCode[code]
		if !exists {
			return nil, fmt.Errorf("%w: %s", ErrChannelUnknown, code)
		}
		if channel.Availability != ChannelConfigurable {
			return nil, fmt.Errorf("%w: %s", ErrChannelUnavailable, code)
		}
		selected = append(selected, channel)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Code < selected[j].Code })
	return selected, nil
}
