package bizruntime

import (
	"encoding/json"
	"errors"
	"net/http"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

func (auth *runtimeWebAuth) handlePasswordChange(writer http.ResponseWriter, request *http.Request) {
	authentication, _, err := auth.authenticateSession(request)
	if err != nil || authentication.Session.ActorKind != accesspersistence.WebActorUser || authentication.Session.UserID == "" {
		http.Error(writer, "Unauthorized", http.StatusUnauthorized)
		return
	}
	store := auth.currentStore()
	if store == nil || !store.ValidateWebSessionCSRF(authentication, request.Header.Get("X-CSRF-Token")) {
		http.Error(writer, "Forbidden", http.StatusForbidden)
		return
	}
	var input struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
		ConfirmPassword string `json:"confirm_password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(writer, "invalid password change request", http.StatusBadRequest)
		return
	}
	err = store.ChangeOwnPassword(request.Context(), authentication.Session.UserID, input.CurrentPassword, input.NewPassword, input.ConfirmPassword)
	switch {
	case err == nil:
		auth.clearCookie(writer, auth.sessionCookieName())
		writeJSON(writer, http.StatusOK, map[string]any{"changed": true, "reauthentication_required": true})
	case errors.Is(err, accesspersistence.ErrCurrentPasswordInvalid):
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "CURRENT_PASSWORD_INVALID"})
	case errors.Is(err, accesspersistence.ErrWeakUserPassword):
		writeJSON(writer, http.StatusUnprocessableEntity, map[string]any{"error": "WEAK_PASSWORD"})
	case errors.Is(err, accesspersistence.ErrPasswordMismatch):
		writeJSON(writer, http.StatusUnprocessableEntity, map[string]any{"error": "PASSWORD_MISMATCH"})
	default:
		http.Error(writer, "password change unavailable", http.StatusServiceUnavailable)
	}
}
