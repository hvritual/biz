package authorization

import (
	"sort"

	"yunka.io/gateway/authz"
)

type HTTPBinding struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Action struct {
	Code            string                `json:"code"`
	Domain          string                `json:"domain"`
	Application     string                `json:"application"`
	UseCase         string                `json:"use_case"`
	TenantRequired  bool                  `json:"tenant_required"`
	Authentication  []string              `json:"authentication"`
	Permissions     []authz.PermissionKey `json:"permissions"`
	PermissionMode  string                `json:"permission_mode"`
	Classification  string                `json:"classification"`
	ModuleCode      string                `json:"module_code,omitempty"`
	CapabilityCodes []string              `json:"capability_codes,omitempty"`
	RPC             string                `json:"rpc,omitempty"`
	HTTP            []HTTPBinding         `json:"http,omitempty"`
}

type PermissionDefinition struct {
	Permission authz.PermissionKey `json:"permission"`
	Groups     []string            `json:"groups"`
	Actions    []string            `json:"actions"`
}

func Catalog() []Action {
	out := make([]Action, len(generatedActions))
	for index, action := range generatedActions {
		out[index] = cloneAction(action)
	}
	return out
}

func TenantRolePermissions() []PermissionDefinition {
	type aggregate struct {
		groups  map[string]struct{}
		actions map[string]struct{}
	}
	values := map[authz.PermissionKey]*aggregate{}
	for _, action := range generatedActions {
		if !action.TenantRequired {
			continue
		}
		group := action.Domain + "/" + action.Application
		for _, permission := range action.Permissions {
			if permission == "" {
				continue
			}
			item := values[permission]
			if item == nil {
				item = &aggregate{groups: map[string]struct{}{}, actions: map[string]struct{}{}}
				values[permission] = item
			}
			item.groups[group] = struct{}{}
			item.actions[action.Code] = struct{}{}
		}
	}
	keys := make([]string, 0, len(values))
	for permission := range values {
		keys = append(keys, string(permission))
	}
	sort.Strings(keys)
	out := make([]PermissionDefinition, 0, len(keys))
	for _, raw := range keys {
		permission := authz.PermissionKey(raw)
		item := values[permission]
		groups := mapKeys(item.groups)
		actions := mapKeys(item.actions)
		out = append(out, PermissionDefinition{Permission: permission, Groups: groups, Actions: actions})
	}
	return out
}

func AuthorizedActions(grants []authz.Grant) []Action {
	allowed := map[authz.PermissionKey]struct{}{}
	for _, grant := range grants {
		if grant.Permission != "" {
			allowed[grant.Permission] = struct{}{}
		}
	}
	out := []Action{}
	for _, action := range generatedActions {
		if !action.TenantRequired || !actionAllowed(action, allowed) {
			continue
		}
		out = append(out, cloneAction(action))
	}
	return out
}

func actionAllowed(action Action, allowed map[authz.PermissionKey]struct{}) bool {
	if len(action.Permissions) == 0 {
		return true
	}
	if action.PermissionMode == "any" {
		for _, permission := range action.Permissions {
			if _, ok := allowed[permission]; ok {
				return true
			}
		}
		return false
	}
	for _, permission := range action.Permissions {
		if _, ok := allowed[permission]; !ok {
			return false
		}
	}
	return true
}

func cloneAction(action Action) Action {
	action.Authentication = append([]string(nil), action.Authentication...)
	action.Permissions = append([]authz.PermissionKey(nil), action.Permissions...)
	action.CapabilityCodes = append([]string(nil), action.CapabilityCodes...)
	action.HTTP = append([]HTTPBinding(nil), action.HTTP...)
	return action
}

func mapKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
