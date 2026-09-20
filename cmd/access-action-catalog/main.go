package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

type operationPlans struct {
	SchemaVersion int         `json:"schemaVersion"`
	Operations    []operation `json:"operations"`
}

type operation struct {
	OperationID string `json:"operationId"`
	Domain      string `json:"domain"`
	Application string `json:"application"`
	UseCase     string `json:"useCase"`
	Security    struct {
		TenantRequired bool     `json:"tenantRequired"`
		Authentication []string `json:"authentication"`
		Permissions    []string `json:"permissions"`
		PermissionMode string   `json:"permissionMode"`
	} `json:"security"`
	Bindings struct {
		RPC  string        `json:"rpc"`
		HTTP []httpBinding `json:"http"`
	} `json:"bindings"`
}

type httpBinding struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type capabilityMappings struct {
	SchemaVersion  int             `json:"schema_version"`
	MappingVersion string          `json:"mapping_version"`
	Operations     []capabilityMap `json:"operations"`
}

type capabilityMap struct {
	OperationID     string   `json:"operation_id"`
	Classification  string   `json:"classification"`
	ModuleCode      string   `json:"module_code"`
	CapabilityCodes []string `json:"capability_codes"`
}

func main() {
	var root string
	var write bool
	flag.StringVar(&root, "root", ".", "repository root")
	flag.BoolVar(&write, "write", false, "write generated catalog instead of only checking")
	flag.Parse()
	if err := run(root, write); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(root string, write bool) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	plans, err := loadJSON[operationPlans](filepath.Join(root, "contracts", "generated", "operation-plans.json"))
	if err != nil {
		return err
	}
	mappings, err := loadJSON[capabilityMappings](filepath.Join(root, "contracts", "commercial", "operation-capabilities.v1.json"))
	if err != nil {
		return err
	}
	if plans.SchemaVersion != 2 {
		return fmt.Errorf("action catalog: operation plan schema_version=%d want=2", plans.SchemaVersion)
	}
	if mappings.SchemaVersion != 1 || mappings.MappingVersion == "" {
		return errors.New("action catalog: commercial mapping schema/version is invalid")
	}

	byID := make(map[string]capabilityMap, len(mappings.Operations))
	for _, mapping := range mappings.Operations {
		if mapping.OperationID == "" {
			return errors.New("action catalog: commercial mapping contains empty operation id")
		}
		if _, exists := byID[mapping.OperationID]; exists {
			return fmt.Errorf("action catalog: duplicate commercial mapping %q", mapping.OperationID)
		}
		byID[mapping.OperationID] = mapping
	}
	seen := make(map[string]struct{}, len(plans.Operations))
	for _, op := range plans.Operations {
		if op.OperationID == "" {
			return errors.New("action catalog: operation plan contains empty operation id")
		}
		if _, exists := seen[op.OperationID]; exists {
			return fmt.Errorf("action catalog: duplicate operation plan %q", op.OperationID)
		}
		seen[op.OperationID] = struct{}{}
		if _, exists := byID[op.OperationID]; !exists {
			return fmt.Errorf("action catalog: operation %q has no commercial mapping", op.OperationID)
		}
	}
	if len(seen) != len(byID) {
		extra := make([]string, 0)
		for id := range byID {
			if _, exists := seen[id]; !exists {
				extra = append(extra, id)
			}
		}
		sort.Strings(extra)
		return fmt.Errorf("action catalog: commercial mappings without operation plans: %v", extra)
	}

	generated, err := render(plans.Operations, byID, mappings.MappingVersion)
	if err != nil {
		return err
	}
	target := filepath.Join(root, "internal", "access", "authorization", "catalog_gen.go")
	if write {
		return os.WriteFile(target, generated, 0o644)
	}
	current, err := os.ReadFile(target)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, generated) {
		return errors.New("action catalog: generated catalog is stale; run make authorization-generate")
	}
	return nil
}

func loadJSON[T any](path string) (T, error) {
	var value T
	payload, err := os.ReadFile(path)
	if err != nil {
		return value, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("action catalog: decode %s: %w", path, err)
	}
	return value, nil
}

func render(operations []operation, mappings map[string]capabilityMap, mappingVersion string) ([]byte, error) {
	var out bytes.Buffer
	out.WriteString("// Code generated from contracts/generated/operation-plans.json and contracts/commercial/operation-capabilities.v1.json; DO NOT EDIT.\n\n")
	out.WriteString("package authorization\n\n")
	out.WriteString("import \"yunka.io/gateway/authz\"\n\n")
	fmt.Fprintf(&out, "const CommercialCapabilityMappingVersion = %s\n\n", strconv.Quote(mappingVersion))
	out.WriteString("var generatedActions = []Action{\n")
	for _, op := range operations {
		mapping := mappings[op.OperationID]
		mode := op.Security.PermissionMode
		if mode == "" {
			mode = "all"
		}
		out.WriteString("\t{\n")
		fmt.Fprintf(&out, "\t\tCode: %s, Domain: %s, Application: %s, UseCase: %s,\n",
			strconv.Quote(op.OperationID), strconv.Quote(op.Domain), strconv.Quote(op.Application), strconv.Quote(op.UseCase))
		fmt.Fprintf(&out, "\t\tTenantRequired: %t, Authentication: %s,\n",
			op.Security.TenantRequired, stringSliceLiteral(op.Security.Authentication))
		fmt.Fprintf(&out, "\t\tPermissions: %s, PermissionMode: %s,\n",
			permissionSliceLiteral(op.Security.Permissions), strconv.Quote(mode))
		fmt.Fprintf(&out, "\t\tClassification: %s, ModuleCode: %s, CapabilityCodes: %s,\n",
			strconv.Quote(mapping.Classification), strconv.Quote(mapping.ModuleCode), stringSliceLiteral(mapping.CapabilityCodes))
		fmt.Fprintf(&out, "\t\tRPC: %s, HTTP: %s,\n",
			strconv.Quote(op.Bindings.RPC), httpSliceLiteral(op.Bindings.HTTP))
		out.WriteString("\t},\n")
	}
	out.WriteString("}\n")
	formatted, err := format.Source(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("action catalog: gofmt generated source: %w", err)
	}
	return formatted, nil
}

func stringSliceLiteral(values []string) string {
	var out bytes.Buffer
	out.WriteString("[]string{")
	for index, value := range values {
		if index > 0 {
			out.WriteString(", ")
		}
		out.WriteString(strconv.Quote(value))
	}
	out.WriteString("}")
	return out.String()
}

func permissionSliceLiteral(values []string) string {
	var out bytes.Buffer
	out.WriteString("[]authz.PermissionKey{")
	for index, value := range values {
		if index > 0 {
			out.WriteString(", ")
		}
		out.WriteString("authz.PermissionKey(")
		out.WriteString(strconv.Quote(value))
		out.WriteString(")")
	}
	out.WriteString("}")
	return out.String()
}

func httpSliceLiteral(values []httpBinding) string {
	var out bytes.Buffer
	out.WriteString("[]HTTPBinding{")
	for index, value := range values {
		if index > 0 {
			out.WriteString(", ")
		}
		out.WriteString("{Method: ")
		out.WriteString(strconv.Quote(value.Method))
		out.WriteString(", Path: ")
		out.WriteString(strconv.Quote(value.Path))
		out.WriteString("}")
	}
	out.WriteString("}")
	return out.String()
}
