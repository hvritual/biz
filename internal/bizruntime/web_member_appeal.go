package bizruntime

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
)

func (auth *runtimeWebAuth) handleMemberAppealEligibility(writer http.ResponseWriter, request *http.Request) {
	authentication, _, err := auth.authenticateSession(request)
	if err != nil || authentication.Session.ActorKind != accesspersistence.WebActorUser || strings.TrimSpace(authentication.Session.UserID) == "" {
		http.Error(writer, "Unauthorized", http.StatusUnauthorized)
		return
	}
	service := auth.currentMemberAppeals()
	if service == nil {
		http.Error(writer, "member appeal unavailable", http.StatusServiceUnavailable)
		return
	}
	items, err := service.ListEligible(request.Context(), authentication.Session.UserID)
	if err != nil {
		http.Error(writer, "member appeal unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"eligible": items})
}

func (auth *runtimeWebAuth) handleMemberAppealSubmit(writer http.ResponseWriter, request *http.Request) {
	authentication, _, err := auth.authenticateSession(request)
	if err != nil || authentication.Session.ActorKind != accesspersistence.WebActorUser || strings.TrimSpace(authentication.Session.UserID) == "" {
		http.Error(writer, "Unauthorized", http.StatusUnauthorized)
		return
	}
	store := auth.currentStore()
	if store == nil || !store.ValidateWebSessionCSRF(authentication, request.Header.Get("X-CSRF-Token")) {
		http.Error(writer, "Forbidden", http.StatusForbidden)
		return
	}
	service := auth.currentMemberAppeals()
	if service == nil {
		http.Error(writer, "member appeal unavailable", http.StatusServiceUnavailable)
		return
	}
	var input struct {
		TenantID string `json:"tenant_id"`
		Reason   string `json:"reason"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || strings.TrimSpace(input.TenantID) == "" || strings.TrimSpace(input.Reason) == "" {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "INVALID_APPEAL_REQUEST"})
		return
	}
	receipt, err := service.Submit(request.Context(), authentication.Session.UserID, input.TenantID, input.Reason)
	if err == nil {
		writeJSON(writer, http.StatusAccepted, map[string]any{
			"accepted":               true,
			"appeal_id":              receipt.AppealID,
			"tenant_id":              receipt.TenantID,
			"membership_status":      receipt.Status,
			"state":                  receipt.State,
			"submitted_at":           receipt.SubmittedAt.UTC().Format("2006-01-02T15:04:05.999999Z07:00"),
			"notification_event_ids": receipt.NotificationEventIDs,
		})
		return
	}
	var rateLimited accesspersistence.MemberAppealRateLimitError
	switch {
	case errors.As(err, &rateLimited):
		retry := int(rateLimited.RetryAfter.Seconds())
		if retry < 1 {
			retry = 1
		}
		writer.Header().Set("Retry-After", strconv.Itoa(retry))
		writeJSON(writer, http.StatusTooManyRequests, map[string]any{"error": "APPEAL_RATE_LIMITED", "retry_after_seconds": retry})
	case errors.Is(err, accesspersistence.ErrMemberAppealNotEligible):
		writeJSON(writer, http.StatusConflict, map[string]any{"error": "APPEAL_NOT_ELIGIBLE"})
	default:
		http.Error(writer, "member appeal unavailable", http.StatusServiceUnavailable)
	}
}
