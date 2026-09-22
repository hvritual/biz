package domain

import (
	"errors"
	"sort"
	"strings"
	"time"
)

const (
	TenantDataPolicyStatusActive  = "active"
	TenantDataPolicyStatusRevoked = "revoked"
	DataPolicyInvalidMissing      = "missing"
	DataPolicyInvalidRevoked      = "revoked"
	DataPolicyInvalidNotStarted   = "not_started"
	DataPolicyInvalidExpired      = "expired"
)

var ErrInvalidTenantDataPolicy = errors.New("access: invalid tenant data policy")

type DataPolicy struct {
	ID        string
	TenantID  string
	Name      string
	Status    string
	SiteIDs   []string
	Version   uint64
	NotBefore *time.Time
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DataPolicyReference struct {
	PolicyID        string
	PolicyName      string
	PolicyVersion   uint64
	AcceptedVersion uint64
	Effective       bool
	InvalidReason   string
}

func NewDataPolicy(id, tenantID, name string, siteIDs []string, notBefore, expiresAt *time.Time, now time.Time) (DataPolicy, error) {
	policy := DataPolicy{ID: strings.TrimSpace(id), TenantID: strings.TrimSpace(tenantID), Status: TenantDataPolicyStatusActive, Version: 1, CreatedAt: now, UpdatedAt: now}
	if policy.ID == "" || len(policy.ID) > 160 || policy.TenantID == "" {
		return DataPolicy{}, ErrInvalidTenantDataPolicy
	}
	if err := policy.Apply(name, siteIDs, notBefore, expiresAt, now); err != nil {
		return DataPolicy{}, err
	}
	return policy, nil
}

func (policy *DataPolicy) Apply(name string, siteIDs []string, notBefore, expiresAt *time.Time, now time.Time) error {
	if policy == nil || policy.Status == TenantDataPolicyStatusRevoked {
		return ErrInvalidTenantDataPolicy
	}
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 {
		return ErrInvalidTenantDataPolicy
	}
	if notBefore != nil && expiresAt != nil && !expiresAt.After(*notBefore) {
		return ErrInvalidTenantDataPolicy
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(siteIDs))
	for _, value := range siteIDs {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 160 {
			return ErrInvalidTenantDataPolicy
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	policy.Name = name
	policy.SiteIDs = normalized
	policy.NotBefore = clonePolicyTime(notBefore)
	policy.ExpiresAt = clonePolicyTime(expiresAt)
	policy.UpdatedAt = now
	return nil
}

func (policy DataPolicy) EffectiveAt(now time.Time) (bool, string) {
	if policy.Status == TenantDataPolicyStatusRevoked {
		return false, DataPolicyInvalidRevoked
	}
	if policy.Status != TenantDataPolicyStatusActive {
		return false, DataPolicyInvalidMissing
	}
	if policy.NotBefore != nil && now.Before(*policy.NotBefore) {
		return false, DataPolicyInvalidNotStarted
	}
	if policy.ExpiresAt != nil && !now.Before(*policy.ExpiresAt) {
		return false, DataPolicyInvalidExpired
	}
	return true, ""
}

func clonePolicyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := value.UTC()
	return &copy
}
