// Package capabilitymap compiles commercial metadata for build-time coverage.
// It is not an authorizer and must not be substituted for IAM or CE-05 guards.
package capabilitymap

type Classification string

const (
	TenantBusiness     Classification = "tenant_business"
	PlatformManagement Classification = "platform_management"
	Recovery           Classification = "recovery"
	FoundationExempt   Classification = "foundation_exempt"
)

type Declaration struct {
	SchemaVersion          int       `json:"schema_version"`
	MappingVersion         string    `json:"mapping_version"`
	RetiredCapabilityCodes []string  `json:"retired_capability_codes"`
	Operations             []Mapping `json:"operations"`
}

type Mapping struct {
	OperationID     string         `json:"operation_id"`
	Classification  Classification `json:"classification"`
	ModuleCode      string         `json:"module_code,omitempty"`
	CapabilityCodes []string       `json:"capability_codes"`
	ExemptionReason string         `json:"exemption_reason,omitempty"`
	Children        []Child        `json:"children"`
}

type Child struct {
	OperationID string `json:"operation_id"`
	Mode        string `json:"mode"` // always, or conditional on the real execution path
	Condition   string `json:"condition,omitempty"`
	Source      string `json:"source"` // repository-relative Go path#function
}

// Only schema-versioned fields needed by this compiler are projected from
// Yunka artifacts. No permissions or transport routes are reinterpreted.
type Plans struct {
	SchemaVersion int         `json:"schemaVersion"`
	Operations    []Operation `json:"operations"`
}

type Operation struct {
	ID          string `json:"operationId"`
	Domain      string `json:"domain"`
	Application string `json:"application"`
	Security    struct {
		TenantRequired bool `json:"tenantRequired"`
	} `json:"security"`
	Composition struct {
		Requires []string `json:"requiresOperations"`
	} `json:"composition"`
	Bindings struct {
		RPC  string        `json:"rpc"`
		HTTP []HTTPBinding `json:"http"`
	} `json:"bindings"`
}

type HTTPBinding struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Body   string `json:"body,omitempty"`
}

type Manifest struct {
	SchemaVersion int `json:"schemaVersion"`
	Services      []struct {
		FullName    string `json:"fullName"`
		Domain      string `json:"domain"`
		Application struct {
			Name       string              `json:"name"`
			Operations []ManifestOperation `json:"operations"`
		} `json:"application"`
		Methods []struct {
			Name      string            `json:"name"`
			Operation ManifestOperation `json:"operation"`
			HTTP      []HTTPBinding     `json:"http"`
		} `json:"methods"`
	} `json:"services"`
}

type ManifestOperation struct {
	ID             string   `json:"id"`
	TenantRequired bool     `json:"tenantRequired"`
	Requires       []string `json:"requiresOperations"`
}

// LookupModule resolves the actual CE-02 code Registry, not a second catalog.
type LookupModule func(moduleCode string) (capabilityCodes []string, exists bool)

type Capability struct {
	Code       string `json:"capability_code"`
	ModuleCode string `json:"module_code"`
}

type CompiledOperation struct {
	Mapping
	ConsumerType                  string   `json:"consumer_type"`
	TenantRequired                bool     `json:"tenant_required"`
	RPC                           string   `json:"rpc,omitempty"`
	RequiredCapabilityCodes       []string `json:"required_capability_codes"`
	AlwaysRequiredCapabilityCodes []string `json:"always_required_capability_codes"`
}

type Document struct {
	SchemaVersion          int                 `json:"schema_version"`
	MappingVersion         string              `json:"mapping_version"`
	InputsSHA256           map[string]string   `json:"inputs_sha256"`
	RetiredCapabilityCodes []string            `json:"retired_capability_codes"`
	Capabilities           []Capability        `json:"capabilities"`
	Operations             []CompiledOperation `json:"operations"`
}
