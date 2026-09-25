package bizruntime

import (
	"errors"
	"net/http"
	"strings"

	"github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
)

// The expected session is a mandatory comparison precondition on this new
// endpoint. It is never a source of tenant/user identity. The older optional
// precondition helper is deliberately not changed for unrelated routes.
func (auth *runtimeWebAuth) notificationPreferenceContext(writer http.ResponseWriter, request *http.Request) (domain.NotificationPreferenceOwner, accessports.SelfNotificationPreferences, bool) {
	writer.Header().Set("Cache-Control", "no-store")
	authentication, rawSession, err := auth.authenticateSession(request)
	if err != nil || authentication.Session.ActorKind != accesspersistence.WebActorUser ||
		authentication.Session.UserID == "" || authentication.Session.ActiveTenantID == "" {
		writeJSON(writer, http.StatusUnauthorized, map[string]any{"error": "UNAUTHORIZED"})
		return domain.NotificationPreferenceOwner{}, nil, false
	}
	expected := request.Header.Values("X-Biz-Session-Context")
	if len(expected) != 1 || strings.TrimSpace(expected[0]) == "" || validateExpectedWebSession(expected[0], authentication.Session) != nil {
		writeJSON(writer, http.StatusConflict, map[string]any{"error": "SESSION_CONTEXT_CHANGED"})
		return domain.NotificationPreferenceOwner{}, nil, false
	}
	store := auth.currentStore()
	if store == nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]any{"error": "NOTIFICATION_PREFERENCES_UNAVAILABLE"})
		return domain.NotificationPreferenceOwner{}, nil, false
	}
	if request.Method != http.MethodGet {
		csrf := request.Header.Values("X-CSRF-Token")
		if len(csrf) != 1 || !store.ValidateWebSessionCSRF(authentication, csrf[0]) {
			writeJSON(writer, http.StatusForbidden, map[string]any{"error": "FORBIDDEN"})
			return domain.NotificationPreferenceOwner{}, nil, false
		}
	}
	if request.URL.RawQuery != "" {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "PREFERENCE_QUERY_NOT_SUPPORTED"})
		return domain.NotificationPreferenceOwner{}, nil, false
	}
	return domain.NotificationPreferenceOwner{
		TenantID: authentication.Session.ActiveTenantID, UserID: authentication.Session.UserID,
	}, store.NotificationPreferencesForWebSession(rawSession, authentication.Session), true
}

func (auth *runtimeWebAuth) handleReadNotificationPreferences(writer http.ResponseWriter, request *http.Request) {
	owner, service, ok := auth.notificationPreferenceContext(writer, request)
	if !ok {
		return
	}
	preferences, err := service.ReadNotificationPreferences(request.Context(), owner)
	if err != nil {
		writeNotificationPreferenceError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, preferences)
}

func (auth *runtimeWebAuth) handleChangeNotificationPreference(writer http.ResponseWriter, request *http.Request) {
	owner, service, ok := auth.notificationPreferenceContext(writer, request)
	if !ok {
		return
	}
	keys := request.Header.Values("Idempotency-Key")
	if len(keys) != 1 {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "IDEMPOTENCY_KEY_REQUIRED"})
		return
	}
	change, err := decodeNotificationPreferenceChange(writer, request, keys[0])
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "INVALID_NOTIFICATION_PREFERENCE"})
		return
	}
	receipt, err := service.ChangeNotificationPreference(request.Context(), owner, change)
	if err != nil {
		writeNotificationPreferenceError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, receipt)
}

func writeNotificationPreferenceError(writer http.ResponseWriter, err error) {
	status, code := http.StatusServiceUnavailable, "NOTIFICATION_PREFERENCES_UNAVAILABLE"
	switch {
	case errors.Is(err, accesspersistence.ErrWebSessionInvalid):
		status, code = http.StatusUnauthorized, "UNAUTHORIZED"
	case errors.Is(err, accesspersistence.ErrWebSessionChanged):
		status, code = http.StatusConflict, "SESSION_CONTEXT_CHANGED"
	case errors.Is(err, domain.ErrNotificationPreferenceConflict):
		status, code = http.StatusConflict, "PREFERENCE_VERSION_CONFLICT"
	case errors.Is(err, domain.ErrNotificationPreferenceIdempotencyConflict):
		status, code = http.StatusConflict, "IDEMPOTENCY_CONFLICT"
	case errors.Is(err, domain.ErrNotificationPreferenceForbidden):
		status, code = http.StatusForbidden, "FORBIDDEN"
	}
	// Do not leak database errors, session values, raw idempotency keys or IDs.
	writeJSON(writer, status, map[string]any{"error": code})
}
