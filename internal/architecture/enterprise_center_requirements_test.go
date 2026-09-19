package architecture_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestEnterpriseCenterRequirementMapHasExactCoverage(t *testing.T) {
	root := filepath.Join("..", "..")
	body, err := os.ReadFile(filepath.Join(root, "docs", "enterprise-center", "requirements-map.md"))
	if err != nil {
		t.Fatal(err)
	}

	rangePattern := regexp.MustCompile(`\| (US|FR|Q)-(\d+)\.\.(US|FR|Q)-(\d+) \|`)
	seen := map[string]map[int]int{"US": {}, "FR": {}, "Q": {}}
	for _, match := range rangePattern.FindAllStringSubmatch(string(body), -1) {
		if match[1] != match[3] {
			t.Fatalf("mixed requirement range: %q", match[0])
		}
		start, err := strconv.Atoi(match[2])
		if err != nil {
			t.Fatal(err)
		}
		end, err := strconv.Atoi(match[4])
		if err != nil {
			t.Fatal(err)
		}
		if start <= 0 || end < start {
			t.Fatalf("invalid %s range %d..%d", match[1], start, end)
		}
		for id := start; id <= end; id++ {
			seen[match[1]][id]++
		}
	}

	want := map[string]int{"US": 51, "FR": 183, "Q": 20}
	for prefix, maxID := range want {
		for id := 1; id <= maxID; id++ {
			count := seen[prefix][id]
			if count != 1 {
				t.Errorf("%s requirement %s must be mapped exactly once, got %d", prefix, requirementID(prefix, id), count)
			}
		}
		for id := range seen[prefix] {
			if id > maxID {
				t.Errorf("unexpected %s requirement %s", prefix, requirementID(prefix, id))
			}
		}
	}

	text := string(body)
	for issue := 167; issue <= 192; issue++ {
		if !strings.Contains(text, fmt.Sprintf("#%d", issue)) {
			t.Errorf("delivery issue #%d is missing from requirement map", issue)
		}
	}
	for _, superseded := range []string{"FR-128", "FR-132"} {
		if !strings.Contains(text, superseded) || !strings.Contains(text, "SUPERSEDED") {
			t.Errorf("%s supersede mapping must remain explicit", superseded)
		}
	}
}

func TestEnterpriseCenterAuthorityBaselineRejectsLegacyAuthorizationModel(t *testing.T) {
	root := filepath.Join("..", "..")
	contracts, err := os.ReadFile(filepath.Join(root, "docs", "enterprise-center", "contracts.md"))
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := os.ReadFile(filepath.Join(root, "docs", "enterprise-center", "baseline.md"))
	if err != nil {
		t.Fatal(err)
	}
	joined := string(contracts) + "\n" + string(baseline)
	for _, required := range []string{
		"Access 是 Account、Membership、Profile、Department、Role",
		"Gateway 是统一请求级授权执行点",
		"不创建全局 Authorization Version",
		"不维护 Gateway allow index",
		"PermissionVersion",
	} {
		if !strings.Contains(joined, required) {
			t.Errorf("authority baseline lost required rule %q", required)
		}
	}
}

func requirementID(prefix string, id int) string {
	if prefix == "US" || prefix == "Q" {
		return fmt.Sprintf("%s-%03d", prefix, id)
	}
	return fmt.Sprintf("%s-%d", prefix, id)
}
