package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

var ErrInvalidTenantProfile = errors.New("access: invalid tenant profile")

type TenantProfile struct {
	TenantID     string
	Name         string
	ShortName    string
	Industry     string
	CompanySize  string
	Timezone     string
	ContactName  string
	Phone        string
	Email        string
	Address      string
	Description  string
	LogoAssetRef string
	Version      uint64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (profile *TenantProfile) Update(name, shortName, industry, companySize, timezone, contactName, phone, email, address, description, logoAssetRef string, now time.Time) error {
	if profile == nil {
		return ErrInvalidTenantProfile
	}
	name = strings.TrimSpace(name)
	shortName = strings.TrimSpace(shortName)
	industry = strings.TrimSpace(industry)
	companySize = strings.TrimSpace(companySize)
	timezone = strings.TrimSpace(timezone)
	contactName = strings.TrimSpace(contactName)
	phone = strings.TrimSpace(phone)
	email = strings.ToLower(strings.TrimSpace(email))
	address = strings.TrimSpace(address)
	description = strings.TrimSpace(description)
	logoAssetRef = strings.TrimSpace(logoAssetRef)
	if name == "" || shortName == "" || timezone == "" {
		return ErrInvalidTenantProfile
	}
	if len([]rune(name)) > 200 || len([]rune(shortName)) > 80 || len([]rune(industry)) > 120 || len([]rune(companySize)) > 64 || len([]rune(timezone)) > 64 || len([]rune(contactName)) > 100 || len([]rune(phone)) > 40 || len([]rune(email)) > 320 || len([]rune(address)) > 500 || len([]rune(description)) > 1000 || len([]rune(logoAssetRef)) > 512 {
		return ErrInvalidTenantProfile
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return ErrInvalidTenantProfile
	}
	if email != "" {
		parsed, err := mail.ParseAddress(email)
		if err != nil || !strings.EqualFold(parsed.Address, email) {
			return ErrInvalidTenantProfile
		}
	}
	if strings.HasPrefix(strings.ToLower(logoAssetRef), "data:") {
		return ErrInvalidTenantProfile
	}
	profile.Name = name
	profile.ShortName = shortName
	profile.Industry = industry
	profile.CompanySize = companySize
	profile.Timezone = timezone
	profile.ContactName = contactName
	profile.Phone = phone
	profile.Email = email
	profile.Address = address
	profile.Description = description
	profile.LogoAssetRef = logoAssetRef
	profile.UpdatedAt = now
	return nil
}
