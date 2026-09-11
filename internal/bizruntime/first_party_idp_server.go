package bizruntime

import (
	"errors"
	"net/http"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

// NewFirstPartyIdPHandler builds the standalone OIDC provider HTTP surface.
// Running it as a separate process keeps credential handling and OIDC signing
// keys out of the Biz resource server while reusing the same member authority
// database.
func NewFirstPartyIdPHandler(config FirstPartyIdPConfig, store *accesspersistence.Store) (http.Handler, error) {
	if !config.Enabled() {
		return nil, errors.New("biz runtime: first-party IdP config is required")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("biz runtime: first-party IdP store is required")
	}
	idp, err := newRuntimeFirstPartyIdP(config)
	if err != nil {
		return nil, err
	}
	idp.setStore(store)
	mux := http.NewServeMux()
	idp.register(mux)
	mux.HandleFunc("GET /healthz", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{"status":"ok"}`))
	})
	return mux, nil
}
