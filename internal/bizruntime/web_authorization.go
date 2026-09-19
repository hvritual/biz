package bizruntime

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	accessauthorization "github.com/hvritual/biz/internal/access/authorization"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/commercial/domain/entitlement"
	"yunka.io/gateway/authz"
)

type currentAuthorizationEntitlementReader interface {
	ReadSnapshot(context.Context, string, []string) (entitlement.Result, error)
}

type authorizationModuleView struct {
	Code    string   `json:"code"`
	Allowed bool     `json:"allowed"`
	Reason  string   `json:"reason"`
	Actions []string `json:"actions"`
}

type currentAuthorizationView struct {
	Authenticated     bool                                     `json:"authenticated"`
	ActorKind         string                                   `json:"actor_kind"`
	UserID            string                                   `json:"user_id,omitempty"`
	PlatformSubject   string                                   `json:"platform_subject,omitempty"`
	TenantID          string                                   `json:"tenant_id,omitempty"`
	TenantName        string                                   `json:"tenant_name,omitempty"`
	Timezone          string                                   `json:"timezone,omitempty"`
	Roles             []string                                 `json:"roles"`
	Grants            []accesspersistence.CurrentGrantFact     `json:"grants"`
	DataPolicies      []accesspersistence.CurrentDataPolicy    `json:"data_policies"`
	SiteIDs           []string                                 `json:"site_ids"`
	PermissionVersion string                                   `json:"permission_version,omitempty"`
	Modules           []authorizationModuleView                `json:"modules"`
	Actions           []accessauthorization.Action             `json:"actions"`
	ButtonCodes       []string                                 `json:"button_codes"`
	Entitlement       *authorizationEntitlementVersionSummary  `json:"entitlement,omitempty"`
}

type authorizationEntitlementVersionSummary struct {
	Version         uint64 `json:"version"`
	SourceVersion   uint64 `json:"source_version"`
	CatalogRevision uint64 `json:"catalog_revision"`
}

func (auth *runtimeWebAuth) setAuthorizationEntitlements(reader currentAuthorizationEntitlementReader) {
	if auth == nil {
		return
	}
	auth.mu.Lock()
	auth.entitlements = reader
	auth.mu.Unlock()
}

func (auth *runtimeWebAuth) currentAuthorizationDependencies() (*accesspersistence.Store, currentAuthorizationEntitlementReader) {
	if auth == nil {
		return nil, nil
	}
	auth.mu.RLock()
	defer auth.mu.RUnlock()
	return auth.store, auth.entitlements
}

func (auth *runtimeWebAuth) handleActionCatalog(writer http.ResponseWriter, request *http.Request) {
	authentication, _, err := auth.authenticateSession(request)
	if err != nil {
		http.Error(writer, "Unauthorized", http.StatusUnauthorized)
		return
	}
	_ = authentication
	actions := accessauthorization.Catalog()
	tenantActions := make([]accessauthorization.Action, 0, len(actions))
	for _, action := range actions {
		if action.TenantRequired {
			tenantActions = append(tenantActions, action)
		}
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"schema_version": "v1",
		"actions":        tenantActions,
		"permissions":    accessauthorization.TenantRolePermissions(),
	})
}

func (auth *runtimeWebAuth) handleCurrentAuthorization(writer http.ResponseWriter, request *http.Request) {
	authentication, _, err := auth.authenticateSession(request)
	if err != nil {
		http.Error(writer, "Unauthorized", http.StatusUnauthorized)
		return
	}
	base := currentAuthorizationView{
		Authenticated: true,
		ActorKind: authentication.Session.ActorKind,
		UserID: authentication.Session.UserID,
		PlatformSubject: authentication.Session.PlatformSubject,
		Roles: []string{},
		Grants: []accesspersistence.CurrentGrantFact{},
		DataPolicies: []accesspersistence.CurrentDataPolicy{},
		SiteIDs: []string{},
		Modules: []authorizationModuleView{},
		Actions: []accessauthorization.Action{},
		ButtonCodes: []string{},
	}
	if authentication.Session.ActorKind != accesspersistence.WebActorUser || strings.TrimSpace(authentication.Session.ActiveTenantID) == "" {
		writeJSON(writer, http.StatusOK, base)
		return
	}

	store, entitlements := auth.currentAuthorizationDependencies()
	if store == nil || entitlements == nil {
		http.Error(writer, "authorization unavailable", http.StatusServiceUnavailable)
		return
	}
	snapshot, err := store.CurrentAuthorization(request.Context(), authentication.Principal)
	if err != nil {
		if errors.Is(err, accesspersistence.ErrUnauthorized) {
			http.Error(writer, "Unauthorized", http.StatusUnauthorized)
			return
		}
		http.Error(writer, "authorization unavailable", http.StatusServiceUnavailable)
		return
	}
	entitlementSnapshot, err := entitlements.ReadSnapshot(request.Context(), snapshot.TenantID, nil)
	if err != nil {
		http.Error(writer, "authorization unavailable", http.StatusServiceUnavailable)
		return
	}

	grants := make([]authz.Grant, 0, len(snapshot.Grants))
	for _, grant := range snapshot.Grants {
		grants = append(grants, authz.Grant{Permission: grant.Permission, RoleID: grant.RoleID, Scope: grant.Scope})
	}
	accessActions := accessauthorization.AuthorizedActions(grants)
	capabilityDecisions := map[string]entitlement.Decision{}
	moduleDecisions := map[string]entitlement.Decision{}
	for _, decision := range entitlementSnapshot.Decisions {
		switch decision.Kind {
		case entitlement.Module:
			moduleDecisions[decision.ModuleCode] = decision
		case entitlement.Capability:
			capabilityDecisions[decision.Key] = decision
		}
	}

	effectiveActions := make([]accessauthorization.Action, 0, len(accessActions))
	buttonCodes := []string{}
	moduleActions := map[string][]string{}
	for _, action := range accessActions {
		if !commerciallyAllowsAction(action, capabilityDecisions) {
			continue
		}
		effectiveActions = append(effectiveActions, action)
		buttonCodes = append(buttonCodes, action.Code)
		if action.ModuleCode != "" {
			moduleActions[action.ModuleCode] = append(moduleActions[action.ModuleCode], action.Code)
		}
	}
	sort.Strings(buttonCodes)

	modules := make([]authorizationModuleView, 0, len(moduleDecisions))
	for code, decision := range moduleDecisions {
		actions := append([]string(nil), moduleActions[code]...)
		sort.Strings(actions)
		modules = append(modules, authorizationModuleView{
			Code: code, Allowed: decision.Allowed, Reason: decision.Reason, Actions: actions,
		})
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Code < modules[j].Code })

	base.TenantID = snapshot.TenantID
	base.TenantName = snapshot.TenantName
	base.Timezone = snapshot.Timezone
	base.Roles = snapshot.Roles
	base.Grants = snapshot.Grants
	base.DataPolicies = snapshot.DataPolicies
	base.SiteIDs = snapshot.SiteIDs
	base.PermissionVersion = snapshot.PermissionVersion
	base.Modules = modules
	base.Actions = effectiveActions
	base.ButtonCodes = buttonCodes
	base.Entitlement = &authorizationEntitlementVersionSummary{
		Version: entitlementSnapshot.EntitlementVersion,
		SourceVersion: entitlementSnapshot.SourceVersion,
		CatalogRevision: entitlementSnapshot.CatalogRevision,
	}
	writeJSON(writer, http.StatusOK, base)
}

func commerciallyAllowsAction(action accessauthorization.Action, decisions map[string]entitlement.Decision) bool {
	if action.Classification != "tenant_business" {
		return true
	}
	if action.ModuleCode == "" || len(action.CapabilityCodes) == 0 {
		return false
	}
	for _, capability := range action.CapabilityCodes {
		decision, ok := decisions[capability]
		if !ok || !decision.Allowed {
			return false
		}
	}
	return true
}
