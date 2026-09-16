package domain

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidTenantBranding = errors.New("access: invalid tenant branding selection")
var brandingHex = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// TenantBranding stores intent, not CSS or an alternative UI renderer.
// Version shares the tenant row's CAS so profile and branding edits cannot overwrite each other.
type TenantBranding struct {
	TenantID  string
	Preset    string
	Primary   string
	Version   uint64
	UpdatedAt time.Time
}

func NormalizeTenantBranding(preset, primary string) (string, string, error) {
	preset = strings.ToLower(strings.TrimSpace(preset))
	primary = strings.ToLower(strings.TrimSpace(primary))
	switch preset {
	case "blue", "emerald", "violet", "amber":
		if primary != "" {
			return "", "", ErrInvalidTenantBranding
		}
		return preset, "", nil
	case "custom":
		if !brandingHex.MatchString(primary) || brandingWhiteContrast(primary) < 4.5 {
			return "", "", ErrInvalidTenantBranding
		}
		return preset, primary, nil
	default:
		return "", "", ErrInvalidTenantBranding
	}
}

// The primary token is also used for text on white. Reject low contrast custom values;
// accepting arbitrary CSS or only checking the button foreground would be insufficient.
func brandingWhiteContrast(hex string) float64 {
	rgb, err := strconv.ParseUint(hex[1:], 16, 24)
	if err != nil {
		return 0
	}
	linear := func(value uint64) float64 {
		channel := float64(value) / 255
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	luminance := 0.2126*linear((rgb>>16)&255) + 0.7152*linear((rgb>>8)&255) + 0.0722*linear(rgb&255)
	return 1.05 / (luminance + 0.05)
}
