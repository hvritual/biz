package domain

import "time"

// MemberBusinessScope is an authoritative read model over biz_member_sites.
// Version is the owning tenant Membership resource version so scope changes are
// serialized with profile/lifecycle changes instead of creating a second CAS authority.
type MemberBusinessScope struct {
	TenantID  string
	UserID    string
	SiteIDs   []string
	Version   uint64
	UpdatedAt time.Time
}
