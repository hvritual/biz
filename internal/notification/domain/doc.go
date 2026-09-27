// Package domain defines immutable notification catalog snapshots.
//
// The package owns type and channel metadata, not identity, authorization,
// personal preferences, business-group membership, or delivery. Constructors
// accept explicit registrations; they never invent default types or channels.
// A valid catalog is not permission to configure or send a notification.
package domain
