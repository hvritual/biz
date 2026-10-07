//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCE10GeneratedSubscriptionRESTPreservesNotFoundStatus(t *testing.T) {
	path := filepath.Join("..", "internal", "commercial", "transport", "rest", "zz_yunka_subscription_management_operation_executor_gen.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, `status.Code(err) == codes.NotFound`) ||
		!strings.Contains(text, `http.Error(writer, "application not found", http.StatusNotFound)`) {
		t.Fatal("generated C9 REST adapter must preserve application NotFound as HTTP 404")
	}
}
