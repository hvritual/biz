package delivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
)

func externalHTTPProviderRequest() domain.ExternalProviderRequest {
	return domain.ExternalProviderRequest{
		TaskID: "task-http-185", EventID: "event-http-185", TenantID: "tenant-http-185", UserID: "user-http-185",
		Channel: "email", Destination: "member@example.invalid", TypeCode: "device.fault", Level: domain.LevelUrgent,
		TraceID: "trace-http-185", ReferenceKind: "device", ReferenceID: "machine-http-185",
	}
}

func TestEnterprise185HTTPProviderAuthenticatesAndUsesStableIdempotency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost ||
			request.Header.Get("Authorization") != "Bearer provider-token" ||
			request.Header.Get("Idempotency-Key") != "task-http-185" ||
			request.Header.Get("X-Yunka-Notification-Trace") != "trace-http-185" {
			t.Fatal("provider request headers mismatch")
		}
		var payload map[string]string
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["destination"] != "member@example.invalid" || payload["channel"] != "email" ||
			payload["task_id"] != "task-http-185" || payload["event_id"] != "event-http-185" {
			t.Fatalf("provider payload=%v", payload)
		}
		if _, ok := payload["tenant_id"]; ok {
			t.Fatal("provider payload leaked tenant identity")
		}
		if _, ok := payload["user_id"]; ok {
			t.Fatal("provider payload leaked user identity")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte("{\"receipt_id\":\"provider-receipt-185\",\"status\":\"accepted\"}"))
	}))
	defer server.Close()

	provider, err := NewHTTPProvider(HTTPProviderConfig{Endpoint: server.URL, BearerToken: "provider-token", Idempotent: true}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.SendExternalNotification(context.Background(), externalHTTPProviderRequest())
	if err != nil || result.ReceiptID != "provider-receipt-185" || result.Status != domain.ExternalProviderAccepted ||
		!provider.ExternalNotificationIdempotent() {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestEnterprise185HTTPProviderRejectsUnsafeEndpointAndRedirect(t *testing.T) {
	if _, err := NewHTTPProvider(HTTPProviderConfig{Endpoint: "http://example.com/send", BearerToken: "token"}, nil); err == nil {
		t.Fatal("plain HTTP non-loopback provider was accepted")
	}
	redirected := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected = true
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	provider, err := NewHTTPProvider(HTTPProviderConfig{Endpoint: server.URL, BearerToken: "provider-token", Idempotent: true}, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.SendExternalNotification(context.Background(), externalHTTPProviderRequest())
	failure, ok := err.(ports.ExternalNotificationProviderFailure)
	if err == nil || !ok || failure.Retryable() || !failure.OutcomeKnown() ||
		failure.FailureCode() != "PROVIDER_PROTOCOL_ERROR" || redirected {
		t.Fatalf("failure=%v redirected=%v", err, redirected)
	}
}

func TestEnterprise185HTTPProviderFailureSemantics(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		code      string
		retryable bool
		known     bool
	}{
		{"rate-limit", http.StatusTooManyRequests, "PROVIDER_RATE_LIMITED", true, true},
		{"server-unknown", http.StatusInternalServerError, "DELIVERY_OUTCOME_UNKNOWN", true, false},
		{"auth", http.StatusUnauthorized, "PROVIDER_AUTH_REJECTED", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(tc.status)
			}))
			defer server.Close()
			provider, err := NewHTTPProvider(HTTPProviderConfig{Endpoint: server.URL, BearerToken: "provider-token", Idempotent: true}, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.SendExternalNotification(context.Background(), externalHTTPProviderRequest())
			failure, ok := err.(ports.ExternalNotificationProviderFailure)
			if err == nil || !ok ||
				failure.FailureCode() != tc.code || failure.Retryable() != tc.retryable || failure.OutcomeKnown() != tc.known {
				t.Fatalf("failure=%v", err)
			}
		})
	}
}

type callbackStoreFake struct {
	callback domain.ExternalProviderCallback
	calls    int
	err      error
}

func (store *callbackStoreFake) ApplyExternalProviderCallback(_ context.Context, callback domain.ExternalProviderCallback) (domain.ExternalTaskReceipt, error) {
	store.calls++
	store.callback = callback
	return domain.ExternalTaskReceipt{TaskID: callback.TaskID}, store.err
}

func TestEnterprise185HTTPProviderCallbackRequiresHMACAndFreshTimestamp(t *testing.T) {
	secret := []byte(strings.Repeat("H", 32))
	store := &callbackStoreFake{}
	handler, err := NewHTTPProviderCallbackHandler(store, HTTPProviderCallbackConfig{Secret: secret, MaxClockSkew: 5 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("{\"task_id\":\"task-http-185\",\"receipt_id\":\"provider-receipt-185\",\"status\":\"delivered\",\"failure_code\":\"\"}")
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

	request := httptest.NewRequest(http.MethodPost, "/callbacks/notification", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(externalCallbackTimestampHeader, timestamp)
	request.Header.Set(externalCallbackSignatureHeader, signExternalCallback(secret, timestamp, body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || store.calls != 1 ||
		store.callback.Status != domain.ExternalProviderCallbackDelivered {
		t.Fatalf("status=%d calls=%d callback=%+v", response.Code, store.calls, store.callback)
	}

	bad := httptest.NewRequest(http.MethodPost, "/callbacks/notification", bytes.NewReader(body))
	bad.Header.Set("Content-Type", "application/json")
	bad.Header.Set(externalCallbackTimestampHeader, timestamp)
	bad.Header.Set(externalCallbackSignatureHeader, signExternalCallback([]byte(strings.Repeat("X", 32)), timestamp, body))
	badResponse := httptest.NewRecorder()
	handler.ServeHTTP(badResponse, bad)
	if badResponse.Code != http.StatusUnauthorized || store.calls != 1 {
		t.Fatalf("bad signature status=%d calls=%d", badResponse.Code, store.calls)
	}

	staleTimestamp := strconv.FormatInt(time.Now().UTC().Add(-10*time.Minute).Unix(), 10)
	stale := httptest.NewRequest(http.MethodPost, "/callbacks/notification", bytes.NewReader(body))
	stale.Header.Set("Content-Type", "application/json")
	stale.Header.Set(externalCallbackTimestampHeader, staleTimestamp)
	stale.Header.Set(externalCallbackSignatureHeader, signExternalCallback(secret, staleTimestamp, body))
	staleResponse := httptest.NewRecorder()
	handler.ServeHTTP(staleResponse, stale)
	if staleResponse.Code != http.StatusUnauthorized || store.calls != 1 {
		t.Fatalf("stale callback status=%d calls=%d", staleResponse.Code, store.calls)
	}
}

func TestEnterprise185HTTPProviderCallbackMapsStateConflicts(t *testing.T) {
	secret := []byte(strings.Repeat("H", 32))
	store := &callbackStoreFake{err: domain.ErrExternalDeliveryLease}
	handler, err := NewHTTPProviderCallbackHandler(store, HTTPProviderCallbackConfig{Secret: secret})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("{\"task_id\":\"task-http-185\",\"receipt_id\":\"provider-receipt-185\",\"status\":\"failed\",\"failure_code\":\"PROVIDER_BOUNCED\"}")
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)
	request := httptest.NewRequest(http.MethodPost, "/callbacks/notification", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(externalCallbackTimestampHeader, timestamp)
	request.Header.Set(externalCallbackSignatureHeader, signExternalCallback(secret, timestamp, body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || store.calls != 1 {
		t.Fatalf("status=%d calls=%d", response.Code, store.calls)
	}
}

func signExternalCallback(secret []byte, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

var _ ports.ExternalProviderCallbackStore = (*callbackStoreFake)(nil)
