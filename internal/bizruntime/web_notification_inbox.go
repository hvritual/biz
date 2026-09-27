package bizruntime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	notificationdomain "github.com/hvritual/biz/internal/notification/domain"
)

type webNotificationInbox interface {
	ReadUnread(context.Context, notificationdomain.InboxOwner) (notificationdomain.InboxSnapshot, error)
	MarkAllRead(context.Context, notificationdomain.InboxOwner, string) (notificationdomain.MarkAllReadReceipt, error)
}

func (auth *runtimeWebAuth) setNotificationInbox(service webNotificationInbox) {
	if auth == nil {
		return
	}
	auth.mu.Lock()
	auth.notificationInbox = service
	auth.mu.Unlock()
}

func (auth *runtimeWebAuth) currentNotificationInbox() webNotificationInbox {
	if auth == nil {
		return nil
	}
	auth.mu.RLock()
	defer auth.mu.RUnlock()
	return auth.notificationInbox
}

func (auth *runtimeWebAuth) notificationInboxContext(writer http.ResponseWriter, request *http.Request) (notificationdomain.InboxOwner, webNotificationInbox, bool) {
	writer.Header().Set("Cache-Control", "no-store")
	authentication, _, err := auth.authenticateSession(request)
	if err != nil || authentication.Session.ActorKind != accesspersistence.WebActorUser ||
		authentication.Session.UserID == "" || authentication.Session.ActiveTenantID == "" {
		writeJSON(writer, http.StatusUnauthorized, map[string]any{"error": "UNAUTHORIZED"})
		return notificationdomain.InboxOwner{}, nil, false
	}
	expected := request.Header.Values("X-Biz-Session-Context")
	if len(expected) != 1 || strings.TrimSpace(expected[0]) == "" || validateExpectedWebSession(expected[0], authentication.Session) != nil {
		writeJSON(writer, http.StatusConflict, map[string]any{"error": "SESSION_CONTEXT_CHANGED"})
		return notificationdomain.InboxOwner{}, nil, false
	}
	if request.URL.RawQuery != "" {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "INBOX_QUERY_NOT_SUPPORTED"})
		return notificationdomain.InboxOwner{}, nil, false
	}
	if request.Method != http.MethodGet {
		store := auth.currentStore()
		csrf := request.Header.Values("X-CSRF-Token")
		if store == nil || len(csrf) != 1 || !store.ValidateWebSessionCSRF(authentication, csrf[0]) {
			writeJSON(writer, http.StatusForbidden, map[string]any{"error": "FORBIDDEN"})
			return notificationdomain.InboxOwner{}, nil, false
		}
	}
	service := auth.currentNotificationInbox()
	if service == nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]any{"error": "NOTIFICATION_INBOX_UNAVAILABLE"})
		return notificationdomain.InboxOwner{}, nil, false
	}
	return notificationdomain.InboxOwner{TenantID: authentication.Session.ActiveTenantID, UserID: authentication.Session.UserID}, service, true
}

func (auth *runtimeWebAuth) handleReadNotificationInbox(writer http.ResponseWriter, request *http.Request) {
	owner, service, ok := auth.notificationInboxContext(writer, request)
	if !ok {
		return
	}
	snapshot, err := service.ReadUnread(request.Context(), owner)
	if err != nil {
		writeNotificationInboxError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, snapshot)
}

func (auth *runtimeWebAuth) handleMarkAllNotificationInboxRead(writer http.ResponseWriter, request *http.Request) {
	owner, service, ok := auth.notificationInboxContext(writer, request)
	if !ok {
		return
	}
	keys := request.Header.Values("Idempotency-Key")
	if len(keys) != 1 || strings.TrimSpace(keys[0]) == "" {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "IDEMPOTENCY_KEY_REQUIRED"})
		return
	}
	if request.ContentLength > 256 {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "INVALID_NOTIFICATION_INBOX_REQUEST"})
		return
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 256))
	var body map[string]json.RawMessage
	if err := decoder.Decode(&body); err != nil || body == nil || len(body) != 0 {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "INVALID_NOTIFICATION_INBOX_REQUEST"})
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeJSON(writer, http.StatusBadRequest, map[string]any{"error": "INVALID_NOTIFICATION_INBOX_REQUEST"})
		return
	}
	receipt, err := service.MarkAllRead(request.Context(), owner, keys[0])
	if err != nil {
		writeNotificationInboxError(writer, err)
		return
	}
	writeJSON(writer, http.StatusOK, receipt)
}

func writeNotificationInboxError(writer http.ResponseWriter, err error) {
	status, code := http.StatusServiceUnavailable, "NOTIFICATION_INBOX_UNAVAILABLE"
	if errors.Is(err, notificationdomain.ErrInboxInvalid) {
		status, code = http.StatusBadRequest, "INVALID_NOTIFICATION_INBOX_REQUEST"
	}
	writeJSON(writer, status, map[string]any{"error": code})
}
