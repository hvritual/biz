package capabilitymap

import (
	"bytes"
	"encoding/json"
	"slices"
)

// ValidateEvolution compares the candidate with a fetched main release, not
// with a baseline selected from the candidate itself. Tombstones are append-only.
func ValidateEvolution(previous, next Document) error {
	if previous.SchemaVersion != 1 || next.SchemaVersion != 1 {
		return problem("SCHEMA", "baseline", "unsupported compiled catalog")
	}
	oldVersion, err := version(previous.MappingVersion)
	if err != nil {
		return err
	}
	newVersion, err := version(next.MappingVersion)
	if err != nil {
		return err
	}
	if newVersion < oldVersion {
		return problem("VERSION", "mapping_version", "release version must not decrease")
	}
	owners := map[string]string{}
	for _, capability := range next.Capabilities {
		owners[capability.Code] = capability.ModuleCode
	}
	for _, retired := range previous.RetiredCapabilityCodes {
		if _, active := owners[retired]; active || !slices.Contains(next.RetiredCapabilityCodes, retired) {
			return problem("RETIRED_CAPABILITY", retired, "published tombstones cannot disappear or reactivate")
		}
	}
	for _, capability := range previous.Capabilities {
		if owner, exists := owners[capability.Code]; exists {
			if owner != capability.ModuleCode {
				return problem("CAPABILITY_OWNER", capability.Code, "published capability cannot be repurposed under another module")
			}
		} else if !slices.Contains(next.RetiredCapabilityCodes, capability.Code) {
			return problem("RETIRED_CAPABILITY", capability.Code, "removed capability requires a permanent tombstone")
		}
	}
	// Provenance hashes may change when unrelated contract fields change, but
	// semantic mappings, consumers and graph changes require a new revision.
	previous.InputsSHA256, next.InputsSHA256 = nil, nil
	previous.MappingVersion, next.MappingVersion = "", ""
	a, err := json.Marshal(previous)
	if err != nil {
		return err
	}
	b, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if !bytes.Equal(a, b) && newVersion <= oldVersion {
		return problem("VERSION", "mapping_version", "changed published mapping requires a larger version")
	}
	return nil
}
