package domain

import "errors"

var (
	ErrConfigurationScopeDenied      = errors.New("notification: current scope denied")
	ErrConfigurationRecipientInvalid = errors.New("notification: recipient is outside the active tenant membership")
)
