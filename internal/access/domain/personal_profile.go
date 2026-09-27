package domain

import "time"

// PersonalProfile is a self-only projection over the existing Account and
// tenant Membership authorities. It is not a second user/profile aggregate.
type PersonalProfile struct {
	Member           Membership
	TenantName       string
	AccountCreatedAt time.Time
}
