package domain

import "errors"

var ErrNotificationDeliveryContactUnavailable = errors.New("access: notification delivery contact unavailable")

type OptionalNotificationDeliveryAdmission struct {
	NotificationPreferenceOwner
	Channel           NotificationPreferenceChannel
	Allowed           bool
	Destination       string
	PreferenceVersion uint64
}
