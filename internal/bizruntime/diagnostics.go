package bizruntime

import (
	"net/http"
	"sync"

	"yunka.io/framework/core"
	frameworkdiagnostics "yunka.io/framework/diagnostics"
)

const diagnosticsPath = "/__yunka/diagnostics"

// runtimeDiagnostics is only a lifecycle-safe indirection around the canonical
// framework diagnostics handler. It does not define or transform diagnostics
// data; the payload remains framework/diagnostics.Report sourced from core.App.
type runtimeDiagnostics struct {
	mu      sync.RWMutex
	handler http.Handler
}

func (current *runtimeDiagnostics) set(app *core.App) error {
	collector, err := frameworkdiagnostics.New(app)
	if err != nil {
		return err
	}
	handler, err := frameworkdiagnostics.NewHTTPHandler(collector, frameworkdiagnostics.HTTPOptions{})
	if err != nil {
		return err
	}
	current.mu.Lock()
	current.handler = handler
	current.mu.Unlock()
	return nil
}

func (current *runtimeDiagnostics) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	current.mu.RLock()
	handler := current.handler
	current.mu.RUnlock()
	if handler == nil {
		http.Error(writer, "diagnostics unavailable", http.StatusServiceUnavailable)
		return
	}
	handler.ServeHTTP(writer, request)
}
