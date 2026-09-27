package bizruntime

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The pinned generated REST adapter preserves conflicts but otherwise collapses
// application errors to 400. This is an error-only projection, not another
// transport, grant source, request binder, or Executor. Only the server-side
// application wrapper can record a failure; request text cannot set this state.
type notificationOutcomeKey struct{}
type notificationFailure struct {
	Error     string `json:"error"`
	Operation string `json:"operation"`
	Status    int    `json:"-"`
}
type notificationOutcome struct {
	mu      sync.Mutex
	failure *notificationFailure
}

func notificationErrorStatus(err error) (string, int, bool) {
	s, ok := status.FromError(err)
	if !ok {
		return "", 0, false
	}
	expected := map[string]struct {
		code codes.Code
		http int
	}{
		"NOTIFICATION_INVALID":             {codes.InvalidArgument, http.StatusBadRequest},
		"NOTIFICATION_RECIPIENT_INVALID":   {codes.InvalidArgument, http.StatusBadRequest},
		"NOTIFICATION_FORBIDDEN":           {codes.PermissionDenied, http.StatusForbidden},
		"NOTIFICATION_NOT_FOUND":           {codes.NotFound, http.StatusNotFound},
		"NOTIFICATION_DUPLICATE":           {codes.AlreadyExists, http.StatusConflict},
		"NOTIFICATION_VERSION_CONFLICT":    {codes.Aborted, http.StatusConflict},
		"NOTIFICATION_REPLAY_CONFLICT":     {codes.Aborted, http.StatusConflict},
		"NOTIFICATION_CHANNEL_UNAVAILABLE": {codes.FailedPrecondition, http.StatusPreconditionFailed},
		"NOTIFICATION_UNAVAILABLE":         {codes.Unavailable, http.StatusServiceUnavailable},
	}
	mapping, known := expected[s.Message()]
	return s.Message(), mapping.http, known && s.Code() == mapping.code
}
func rememberNotificationError(ctx context.Context, operation string, err error) error {
	if err == nil || ctx == nil {
		return err
	}
	o, ok := ctx.Value(notificationOutcomeKey{}).(*notificationOutcome)
	if !ok {
		return err
	}
	reason, code, known := notificationErrorStatus(err)
	if !known {
		return err
	}
	// operation is a literal from checkedNotification, not submitted by a caller.
	o.mu.Lock()
	if o.failure == nil {
		o.failure = &notificationFailure{Error: reason, Operation: operation, Status: code}
	}
	o.mu.Unlock()
	return err
}
func notificationHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/tenant/notification/") {
			next.ServeHTTP(w, r)
			return
		}
		o := &notificationOutcome{}
		ctx := context.WithValue(r.Context(), notificationOutcomeKey{}, o)
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(&notificationErrorWriter{ResponseWriter: w, outcome: o}, r.WithContext(ctx))
	})
}

type notificationErrorWriter struct {
	http.ResponseWriter
	outcome  *notificationOutcome
	written  bool
	suppress bool
}

func (w *notificationErrorWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *notificationErrorWriter) WriteHeader(code int) {
	if w.written {
		return
	}
	w.written = true
	w.outcome.mu.Lock()
	f := w.outcome.failure
	w.outcome.mu.Unlock()
	// Never rewrite a successful transaction response, native IAM denial, or
	// parsing error merely by inspecting its text. Commercial error middleware
	// remains outside this writer and retains its existing authoritative result.
	if code >= 400 && f != nil {
		w.suppress = true
		w.Header().Del("Content-Length")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.ResponseWriter.WriteHeader(f.Status)
		_ = json.NewEncoder(w.ResponseWriter).Encode(f)
		return
	}
	w.ResponseWriter.WriteHeader(code)
}
func (w *notificationErrorWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(http.StatusOK)
	}
	if w.suppress {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}
