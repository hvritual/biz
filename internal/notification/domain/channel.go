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

type ChannelAvailability string

const (
	ChannelConfigurable    ChannelAvailability = "configurable"
	ChannelNotConfigurable ChannelAvailability = "unavailable"
)

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

func (registry *ChannelRegistry) Lookup(code string) (Channel, error) {
	if registry == nil || !registry.ready {
		return Channel{}, ErrCatalogUnavailable
	}
	if !validCode(code) {
		return Channel{}, ErrCatalogEntryInvalid
	}
	channel, ok := registry.byCode[code]
	if !ok {
		return Channel{}, fmt.Errorf("%w: %s", ErrChannelUnknown, code)
	}
	return channel, nil
}

func (registry *ChannelRegistry) List() ([]Channel, error) {
	if registry == nil || !registry.ready {
		return nil, ErrCatalogUnavailable
	}
	channels := make([]Channel, len(registry.channels))
	copy(channels, registry.channels)
	return channels, nil
}

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
