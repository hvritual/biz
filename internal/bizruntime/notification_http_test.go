package bizruntime

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNotificationHTTPProjectsOnlyRecordedServerErrors(t *testing.T) {
	cases := []struct {
		reason string
		grpc   codes.Code
		http   int
	}{
		{"NOTIFICATION_INVALID", codes.InvalidArgument, 400},
		{"NOTIFICATION_RECIPIENT_INVALID", codes.InvalidArgument, 400},
		{"NOTIFICATION_FORBIDDEN", codes.PermissionDenied, 403},
		{"NOTIFICATION_NOT_FOUND", codes.NotFound, 404},
		{"NOTIFICATION_DUPLICATE", codes.AlreadyExists, 409},
		{"NOTIFICATION_VERSION_CONFLICT", codes.Aborted, 409},
		{"NOTIFICATION_REPLAY_CONFLICT", codes.Aborted, 409},
		{"NOTIFICATION_CHANNEL_UNAVAILABLE", codes.FailedPrecondition, 412},
		{"NOTIFICATION_UNAVAILABLE", codes.Unavailable, 503},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			original := status.Error(tc.grpc, tc.reason)
			h := notificationHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if rememberNotificationError(r.Context(), "notification.configuration.create", original) != original {
					t.Fatal("error replaced before audit")
				}
				w.Header().Set("Content-Length", "999")
				http.Error(w, "application request failed", 400)
			}))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/tenant/notification/configurations", nil))
			if rec.Code != tc.http || !strings.Contains(rec.Body.String(), tc.reason) || strings.Contains(rec.Body.String(), "application request failed") || rec.Header().Get("Content-Length") != "" || rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("response %d %s %v", rec.Code, rec.Body.String(), rec.Header())
			}
		})
	}
}
func TestNotificationHTTPLeavesIAMParsingAndOtherSurfacesUntouched(t *testing.T) {
	cases := []struct {
		name, path string
		err        error
		code       int
		record     bool
	}{
		{"native-denial", "/v1/tenant/notification/configurations", errors.New("FORBIDDEN"), 403, true},
		{"spoofed-query", "/v1/tenant/notification/configurations?error=NOTIFICATION_NOT_FOUND", nil, 400, false},
		{"unknown-code", "/v1/tenant/notification/configurations", status.Error(codes.Internal, "SQL secret"), 400, true},
		{"wrong-pair", "/v1/tenant/notification/configurations", status.Error(codes.OK, "NOTIFICATION_NOT_FOUND"), 200, true},
		{"wrong-status", "/v1/tenant/notification/configurations", status.Error(codes.InvalidArgument, "NOTIFICATION_NOT_FOUND"), 400, true},
		{"other-surface", "/v1/tenant/members", status.Error(codes.NotFound, "NOTIFICATION_NOT_FOUND"), 400, true},
		{"success-not-overwritten", "/v1/tenant/notification/configurations", status.Error(codes.NotFound, "NOTIFICATION_NOT_FOUND"), 200, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := notificationHTTP(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.record {
					rememberNotificationError(r.Context(), "notification.configuration.get", tc.err)
				}
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte("unchanged"))
			}))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
			if rec.Code != tc.code || rec.Body.String() != "unchanged" {
				t.Fatalf("unexpected error projection: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}
