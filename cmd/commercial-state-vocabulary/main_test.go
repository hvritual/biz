package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var vocabularyInputs = []string{
	"internal/commercial/domain/subscription/model.go",
	"internal/commercial/domain/plan/model.go",
	"internal/commercial/modulecatalog/model.go",
	"contracts/proto/commercial/v1/module.proto",
	"internal/commercial/domain/entitlement/model.go",
	"internal/commercial/domain/subscriptionchange/model.go",
	"internal/commercial/application/subscriptionchanges/internal/usecase/preview.go",
	"internal/commercial/domain/provisioning/model.go",
}

func sourceFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, path := range vocabularyInputs {
		data, err := os.ReadFile(filepath.Join("../..", path))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "web/src/services/commercial"), 0755); err != nil {
		t.Fatal(err)
	}
	return root
}
func mutateSource(t *testing.T, root, path, before, after string) {
	t.Helper()
	target := filepath.Join(root, path)
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(data, []byte(before)) != 1 {
		t.Fatalf("mutation must match once: %s", before)
	}
	if err := os.WriteFile(target, bytes.Replace(data, []byte(before), []byte(after), 1), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestProjectionMatchesCommittedSources(t *testing.T) {
	first, err := generate("../..")
	if err != nil {
		t.Fatal(err)
	}
	second, err := generate("../..")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("generation is not deterministic")
	}
	existing, err := os.ReadFile("../../web/src/services/commercial/state-vocabulary.generated.ts")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, existing) {
		t.Fatal("committed projection drift; run make commercial-state-generate")
	}
}
func TestNewValidatedSubscriptionStateRequiresRegeneration(t *testing.T) {
	root := sourceFixture(t)
	if err := run(root, true); err != nil {
		t.Fatal(err)
	}
	path := vocabularyInputs[0]
	mutateSource(t, root, path, "case StateTrial, StateActive, StateGrace, StateRestricted, StateEnded:", "case StateTrial, StateActive, StateGrace, StateRestricted, StateEnded, \"PAUSED_FUTURE\":")
	if err := run(root, false); err == nil {
		t.Fatal("new backend state silently accepted stale frontend")
	}
	if err := run(root, true); err != nil {
		t.Fatal(err)
	}
	data, err := generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"PAUSED_FUTURE"`)) {
		t.Fatal("new valid wire state not projected")
	}
}
func TestUnrelatedConstantDoesNotBecomePlanState(t *testing.T) {
	root := sourceFixture(t)
	mutateSource(t, root, vocabularyInputs[1], "const (", "const (\n InternalNote = \"NOT_A_PLAN_STATE\"")
	data, err := generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"NOT_A_PLAN_STATE"`)) {
		t.Fatal("non-state constant exposed as state")
	}
}
func TestUnsupportedGoStateExpressionFailsClosed(t *testing.T) {
	root := sourceFixture(t)
	mutateSource(t, root, vocabularyInputs[0], "case StateTrial, StateActive, StateGrace, StateRestricted, StateEnded:", "case StateTrial, StateActive, StateGrace, StateRestricted, StateEnded, computedState():")
	if _, err := generate(root); err == nil {
		t.Fatal("unsupported validator expression silently omitted")
	}
}
func TestUnsupportedTypedSourceKindFailsClosed(t *testing.T) {
	root := sourceFixture(t)
	mutateSource(t, root, vocabularyInputs[4], `AddonSource    SourceKind = "addon"`, `AddonSource    SourceKind = SourceKind("addon")`)
	if _, err := generate(root); err == nil {
		t.Fatal("unresolved source kind silently omitted")
	}
}
func TestNewProtoMemberWithOptionIsIncluded(t *testing.T) {
	root := sourceFixture(t)
	path := vocabularyInputs[3]
	mutateSource(t, root, path, "enum ModuleSalesStatus {", "enum ModuleSalesStatus { MODULE_SALES_STATUS_FUTURE = 8 [deprecated = true];")
	data, err := generate(root)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"MODULE_SALES_STATUS_FUTURE"`)) {
		t.Fatal("inline enum member was ignored")
	}
}
func TestUnsupportedProtoDeclarationFailsClosed(t *testing.T) {
	root := sourceFixture(t)
	mutateSource(t, root, vocabularyInputs[3], "enum ModuleSalesStatus {", "enum ModuleSalesStatus { option allow_alias = true;")
	if _, err := generate(root); err == nil {
		t.Fatal("unsupported enum syntax silently ignored")
	}
}
func TestDuplicateProtoMemberFailsClosed(t *testing.T) {
	root := sourceFixture(t)
	mutateSource(t, root, vocabularyInputs[3], "enum ModuleSalesStatus {", "enum ModuleSalesStatus { MODULE_SALES_STATUS_SELLABLE = 8;")
	if _, err := generate(root); err == nil {
		t.Fatal("duplicate enum member accepted")
	}
}
func TestMissingInputCannotProducePartialGreen(t *testing.T) {
	root := sourceFixture(t)
	if err := os.Remove(filepath.Join(root, vocabularyInputs[4])); err != nil {
		t.Fatal(err)
	}
	if _, err := generate(root); err == nil {
		t.Fatal("missing source accepted")
	}
}
func TestMalformedSourceCannotProducePartialGreen(t *testing.T) {
	root := sourceFixture(t)
	if err := os.WriteFile(filepath.Join(root, vocabularyInputs[0]), []byte("invalid Go"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := generate(root); err == nil {
		t.Fatal("malformed source accepted")
	}
}
func TestSourceStateIncludesScheduledNotInventedPending(t *testing.T) {
	root := sourceFixture(t)
	c := collector{root: root, hashes: map[string]string{}}
	s, err := c.goSource(vocabularyInputs[4])
	if err != nil {
		t.Fatal(err)
	}
	states, err := s.returnValues("Source", "State")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(states, ",") != "active,expired,revoked,scheduled" {
		t.Fatalf("unexpected states: %v", states)
	}
}
func TestReadonlyCheckDoesNotRewriteTamperedProjection(t *testing.T) {
	root := sourceFixture(t)
	target := filepath.Join(root, "web/src/services/commercial/state-vocabulary.generated.ts")
	if err := os.WriteFile(target, []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := run(root, false); err == nil {
		t.Fatal("tampering accepted")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "tampered" {
		t.Fatal("readonly check rewrote source")
	}
}
