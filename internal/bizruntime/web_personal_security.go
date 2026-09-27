package bizruntime

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	accessdomain "github.com/hvritual/biz/internal/access/domain"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	accessports "github.com/hvritual/biz/internal/access/ports"
)

type tenantSelfSecurityContext struct {
	authentication accesspersistence.WebSessionAuthentication
	service        *accesspersistence.TenantSelfSecurityService
}

func (auth *runtimeWebAuth) tenantSelfSecurityContext(writer http.ResponseWriter, request *http.Request) (tenantSelfSecurityContext, bool) {
	authentication, _, err := auth.authenticateSession(request)
	if err != nil || authentication.Session.ActorKind != accesspersistence.WebActorUser ||
		authentication.Session.UserID == "" || authentication.Session.ActiveTenantID == "" {
		http.Error(writer, "Unauthorized", http.StatusUnauthorized)
		return tenantSelfSecurityContext{}, false
	}
	if err := validateExpectedWebSession(request.Header.Get("X-Biz-Session-Context"), authentication.Session); err != nil {
		writeJSON(writer, http.StatusConflict, map[string]any{"error": "SESSION_CONTEXT_CHANGED"})
		return tenantSelfSecurityContext{}, false
	}
	store := auth.currentStore()
	if store == nil || !store.ValidateWebSessionCSRF(authentication, request.Header.Get("X-CSRF-Token")) {
		http.Error(writer, "Forbidden", http.StatusForbidden)
		return tenantSelfSecurityContext{}, false
	}
	service := auth.currentSelfSecurity()
	if service == nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]any{"error": "SELF_SECURITY_UNAVAILABLE"})
		return tenantSelfSecurityContext{}, false
	}
	return tenantSelfSecurityContext{authentication: authentication, service: service}, true
}

func (auth *runtimeWebAuth) handlePersonalContactChangeRequest(writer http.ResponseWriter, request *http.Request) {
	current, ok := auth.tenantSelfSecurityContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > 256 {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "IDEMPOTENCY_KEY_REQUIRED"})
		return
	}
	var input struct {
		Channel         string `json:"channel"`
		Destination     string `json:"destination"`
		CurrentPassword string `json:"current_password"`
	}
	if !decodeSelfSecurityJSON(writer, request, &input) {
		return
	}
	channel, err := parseSelfSecurityChannel(input.Channel)
	if err != nil {
		writeTenantSelfSecurityError(writer, err)
		return
	}
	session := current.authentication.Session
	requestHash := accesspersistence.TokenHash(strings.Join([]string{
		"tenant-self-contact-request", session.UserID, session.ActiveTenantID, idempotencyKey,
	}, "\x00"))
	flowID := "contact-change-" + requestHash[:32]
	receipt, err := current.service.RequestContactChange(
		request.Context(),
		session.UserID,
		session.ActiveTenantID,
		input.CurrentPassword,
		channel,
		input.Destination,
		flowID,
		"tenant-self/contact-change-request/"+requestHash,
	)
	if err != nil {
		writeTenantSelfSecurityError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"challenge_id":         receipt.ChallengeID,
		"flow_id":              receipt.FlowID,
		"masked_destination":   receipt.MaskedDestination,
		"expires_at":           receipt.ExpiresAt,
		"delivery_state":       receipt.DeliveryState,
		"version":              receipt.MemberVersion,
		"resend_after_seconds": int64(receipt.ResendAfter.Seconds()),
	})
}

func (auth *runtimeWebAuth) handlePersonalContactChangeComplete(writer http.ResponseWriter, request *http.Request) {
	current, ok := auth.tenantSelfSecurityContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > 256 {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "IDEMPOTENCY_KEY_REQUIRED"})
		return
	}
	var input struct {
		Channel     string `json:"channel"`
		Destination string `json:"destination"`
		ChallengeID string `json:"challenge_id"`
		FlowID      string `json:"flow_id"`
		OTPCode     string `json:"otp_code"`
		Version     uint64 `json:"version"`
	}
	if !decodeSelfSecurityJSON(writer, request, &input) {
		return
	}
	channel, err := parseSelfSecurityChannel(input.Channel)
	if err != nil {
		writeTenantSelfSecurityError(writer, err)
		return
	}
	session := current.authentication.Session
	receipt, err := current.service.CompleteContactChange(
		request.Context(),
		session.UserID,
		session.ActiveTenantID,
		channel,
		input.ChallengeID,
		input.FlowID,
		input.OTPCode,
		input.Destination,
		input.Version,
		accesspersistence.TokenHash(idempotencyKey),
	)
	if err != nil {
		writeTenantSelfSecurityError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"tenant_id":             receipt.TenantID,
		"user_id":               receipt.UserID,
		"email":                 receipt.MaskedEmail,
		"phone":                 receipt.MaskedPhone,
		"version":               receipt.Version,
		"notification_event_id": receipt.NotificationEventID,
		"notification_state":    receipt.NotificationState,
	})
}

func (auth *runtimeWebAuth) handleTenantDeletionRequest(writer http.ResponseWriter, request *http.Request) {
	current, ok := auth.tenantSelfSecurityContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > 256 {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "IDEMPOTENCY_KEY_REQUIRED"})
		return
	}
	var input struct {
		Channel string `json:"channel"`
	}
	if !decodeSelfSecurityJSON(writer, request, &input) {
		return
	}
	channel, err := parseSelfSecurityChannel(input.Channel)
	if err != nil {
		writeTenantSelfSecurityError(writer, err)
		return
	}
	session := current.authentication.Session
	requestHash := accesspersistence.TokenHash(strings.Join([]string{
		"tenant-self-delete-request", session.UserID, session.ActiveTenantID, idempotencyKey,
	}, "\x00"))
	flowID := "tenant-deletion-" + requestHash[:32]
	receipt, err := current.service.RequestTenantDeletion(
		request.Context(),
		session.UserID,
		session.ActiveTenantID,
		channel,
		flowID,
		"tenant-self/deletion-request/"+requestHash,
	)
	if err != nil {
		writeTenantSelfSecurityError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"challenge_id":         receipt.ChallengeID,
		"flow_id":              receipt.FlowID,
		"masked_destination":   receipt.MaskedDestination,
		"expires_at":           receipt.ExpiresAt,
		"delivery_state":       receipt.DeliveryState,
		"version":              receipt.MemberVersion,
		"resend_after_seconds": int64(receipt.ResendAfter.Seconds()),
	})
}

func (auth *runtimeWebAuth) handleTenantDeletionComplete(writer http.ResponseWriter, request *http.Request) {
	current, ok := auth.tenantSelfSecurityContext(writer, request)
	if !ok {
		return
	}
	idempotencyKey := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if idempotencyKey == "" || len(idempotencyKey) > 256 {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "IDEMPOTENCY_KEY_REQUIRED"})
		return
	}
	var input struct {
		Channel             string `json:"channel"`
		ChallengeID         string `json:"challenge_id"`
		FlowID              string `json:"flow_id"`
		OTPCode             string `json:"otp_code"`
		Version             uint64 `json:"version"`
		ConfirmTenantID     string `json:"confirm_tenant_id"`
		ConfirmIrreversible bool   `json:"confirm_irreversible"`
	}
	if !decodeSelfSecurityJSON(writer, request, &input) {
		return
	}
	session := current.authentication.Session
	if !input.ConfirmIrreversible || strings.TrimSpace(input.ConfirmTenantID) != session.ActiveTenantID {
		writeJSON(writer, http.StatusUnprocessableEntity, map[string]any{"error": "DELETION_CONFIRMATION_REQUIRED"})
		return
	}
	channel, err := parseSelfSecurityChannel(input.Channel)
	if err != nil {
		writeTenantSelfSecurityError(writer, err)
		return
	}
	receipt, err := current.service.CompleteTenantDeletion(
		request.Context(),
		session.UserID,
		session.ActiveTenantID,
		channel,
		input.ChallengeID,
		input.FlowID,
		input.OTPCode,
		input.Version,
		accesspersistence.TokenHash(idempotencyKey),
	)
	if err != nil {
		writeTenantSelfSecurityError(writer, err)
		return
	}
	auth.clearCookie(writer, auth.sessionCookieName())
	writeJSON(writer, http.StatusOK, map[string]any{
		"tenant_id":                 receipt.TenantID,
		"user_id":                   receipt.UserID,
		"version":                   receipt.Version,
		"deleted_at":                receipt.DeletedAt,
		"notification_event_id":     receipt.NotificationEventID,
		"notification_state":        receipt.NotificationState,
		"reauthentication_required": true,
	})
}

func decodeSelfSecurityJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "INVALID_REQUEST"})
		return false
	}
	return true
}

func parseSelfSecurityChannel(value string) (accessdomain.SecurityNotificationChannel, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "email":
		return accessdomain.SecurityNotificationEmail, nil
	case "sms", "phone":
		return accessdomain.SecurityNotificationSMS, nil
	default:
		return "", accessdomain.ErrVerificationInvalid
	}
}

func writeTenantSelfSecurityError(writer http.ResponseWriter, err error) {
	status := http.StatusServiceUnavailable
	code := "SELF_SECURITY_UNAVAILABLE"
	switch {
	case errors.Is(err, accesspersistence.ErrCurrentPasswordInvalid):
		status, code = http.StatusBadRequest, "CURRENT_PASSWORD_INVALID"
	case errors.Is(err, accesspersistence.ErrTenantSelfContactUnchanged):
		status, code = http.StatusUnprocessableEntity, "CONTACT_UNCHANGED"
	case errors.Is(err, accesspersistence.ErrTenantSelfContactUnavailable):
		status, code = http.StatusUnprocessableEntity, "CONTACT_UNAVAILABLE"
	case errors.Is(err, accesspersistence.ErrInvalidContact), errors.Is(err, accessdomain.ErrVerificationInvalid):
		status, code = http.StatusBadRequest, "VERIFICATION_INVALID"
	case errors.Is(err, accessdomain.ErrVerificationRateLimited):
		status, code = http.StatusTooManyRequests, "VERIFICATION_RATE_LIMITED"
	case errors.Is(err, accessdomain.ErrVerificationExpired):
		status, code = http.StatusGone, "VERIFICATION_EXPIRED"
	case errors.Is(err, accessdomain.ErrVerificationConsumed):
		status, code = http.StatusConflict, "VERIFICATION_CONSUMED"
	case errors.Is(err, accessdomain.ErrVerificationConflict):
		status, code = http.StatusConflict, "VERIFICATION_CONFLICT"
	case errors.Is(err, accessports.ErrTenantMemberContactConflict):
		status, code = http.StatusConflict, "CONTACT_CONFLICT"
	case errors.Is(err, accessports.ErrTenantMemberConflict):
		status, code = http.StatusConflict, "MEMBERSHIP_CONFLICT"
	case errors.Is(err, accessports.ErrLastTenantOwner):
		status, code = http.StatusConflict, "LAST_OWNER_PROTECTED"
	case errors.Is(err, accessports.ErrTenantMemberNotFound):
		status, code = http.StatusNotFound, "MEMBERSHIP_NOT_FOUND"
	}
	if retry := verificationRetryAfter(err); retry > 0 {
		writer.Header().Set("Retry-After", strconv.FormatInt(max(1, int64(retry.Seconds())), 10))
	}
	writeJSON(writer, status, map[string]any{"error": code})
}
