package delivery

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/ports"
)

const (
	externalProviderMaxResponseBytes = int64(64 << 10)
	externalCallbackMaxBodyBytes     = int64(64 << 10)

	externalCallbackTimestampHeader = "X-Yunka-Notification-Timestamp"
	externalCallbackSignatureHeader = "X-Yunka-Notification-Signature"
)

type HTTPProviderConfig struct {
	Endpoint    string
	BearerToken string
	Idempotent  bool
}

type HTTPProvider struct {
	endpoint    string
	bearerToken string
	idempotent  bool
	client      *http.Client
}

var _ ports.ExternalNotificationProvider = (*HTTPProvider)(nil)
var _ ports.ExternalNotificationProviderRetrySafety = (*HTTPProvider)(nil)

func NewHTTPProvider(config HTTPProviderConfig, client *http.Client) (*HTTPProvider, error) {
	endpoint, err := validateHTTPProviderEndpoint(config.Endpoint)
	if err != nil {
		return nil, err
	}
	token := strings.TrimSpace(config.BearerToken)
	if token == "" || strings.ContainsAny(token, "\\r\\n") {
		return nil, domain.ErrExternalDeliveryInvalid
	}
	if client == nil {
		client = http.DefaultClient
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &HTTPProvider{
		endpoint:    endpoint,
		bearerToken: token,
		idempotent:  config.Idempotent,
		client:      &copyClient,
	}, nil
}

func (provider *HTTPProvider) ExternalNotificationIdempotent() bool {
	return provider != nil && provider.idempotent
}

func (provider *HTTPProvider) SendExternalNotification(
	ctx context.Context,
	request domain.ExternalProviderRequest,
) (domain.ExternalProviderResult, error) {
	if provider == nil || provider.client == nil || request.Validate() != nil {
		return domain.ExternalProviderResult{}, domain.ErrExternalDeliveryInvalid
	}
	payload := map[string]string{
		"task_id":        request.TaskID,
		"event_id":       request.EventID,
		"channel":        request.Channel,
		"destination":    request.Destination,
		"type_code":      request.TypeCode,
		"level":          string(request.Level),
		"trace_id":       request.TraceID,
		"reference_kind": request.ReferenceKind,
		"reference_id":   request.ReferenceID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.ExternalProviderResult{}, domain.ErrExternalDeliveryInvalid
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.endpoint, bytes.NewReader(body))
	if err != nil {
		return domain.ExternalProviderResult{}, domain.ErrExternalDeliveryInvalid
	}
	httpRequest.Header.Set("Authorization", "Bearer "+provider.bearerToken)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Idempotency-Key", request.TaskID)
	httpRequest.Header.Set("X-Yunka-Notification-Trace", request.TraceID)

	response, err := provider.client.Do(httpRequest)
	if err != nil {
		return domain.ExternalProviderResult{}, httpProviderFailure{
			code: "PROVIDER_TRANSPORT", retryable: true, outcomeKnown: false,
		}
	}
	defer response.Body.Close()
	responseBody, err := readExternalProviderBody(response.Body)
	if err != nil {
		return domain.ExternalProviderResult{}, httpProviderFailure{
			code: "PROVIDER_INVALID_RESPONSE", retryable: false, outcomeKnown: false,
		}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return domain.ExternalProviderResult{}, classifyHTTPProviderStatus(response.StatusCode)
	}

	var resultPayload map[string]string
	if err := json.Unmarshal(responseBody, &resultPayload); err != nil ||
		len(resultPayload) != 2 || resultPayload["receipt_id"] == "" || resultPayload["status"] == "" {
		return domain.ExternalProviderResult{}, httpProviderFailure{
			code: "PROVIDER_INVALID_RESPONSE", retryable: false, outcomeKnown: false,
		}
	}
	result := domain.ExternalProviderResult{
		ReceiptID: resultPayload["receipt_id"],
		Status:    domain.ExternalProviderStatus(resultPayload["status"]),
	}
	if err := result.Validate(); err != nil {
		return domain.ExternalProviderResult{}, httpProviderFailure{
			code: "PROVIDER_INVALID_RESPONSE", retryable: false, outcomeKnown: false,
		}
	}
	return result, nil
}

func validateHTTPProviderEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", domain.ErrExternalDeliveryInvalid
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		host := parsed.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return "", domain.ErrExternalDeliveryInvalid
		}
	default:
		return "", domain.ErrExternalDeliveryInvalid
	}
	return parsed.String(), nil
}

func readExternalProviderBody(reader io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, externalProviderMaxResponseBytes+1))
	if err != nil || int64(len(body)) > externalProviderMaxResponseBytes {
		return nil, domain.ErrExternalDeliveryInvalid
	}
	return body, nil
}

type httpProviderFailure struct {
	code         string
	retryable    bool
	outcomeKnown bool
}

func (failure httpProviderFailure) Error() string {
	return "notification provider request failed: " + failure.code
}
func (failure httpProviderFailure) FailureCode() string { return failure.code }
func (failure httpProviderFailure) Retryable() bool     { return failure.retryable }
func (failure httpProviderFailure) OutcomeKnown() bool  { return failure.outcomeKnown }

var _ ports.ExternalNotificationProviderFailure = httpProviderFailure{}

func classifyHTTPProviderStatus(status int) httpProviderFailure {
	switch {
	case status == http.StatusTooManyRequests:
		return httpProviderFailure{code: "PROVIDER_RATE_LIMITED", retryable: true, outcomeKnown: true}
	case status == http.StatusRequestTimeout || status == http.StatusTooEarly:
		return httpProviderFailure{code: "DELIVERY_OUTCOME_UNKNOWN", retryable: true, outcomeKnown: false}
	case status >= http.StatusInternalServerError:
		return httpProviderFailure{code: "DELIVERY_OUTCOME_UNKNOWN", retryable: true, outcomeKnown: false}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return httpProviderFailure{code: "PROVIDER_AUTH_REJECTED", retryable: false, outcomeKnown: true}
	case status >= http.StatusBadRequest:
		return httpProviderFailure{code: "PROVIDER_REJECTED", retryable: false, outcomeKnown: true}
	default:
		return httpProviderFailure{code: "PROVIDER_PROTOCOL_ERROR", retryable: false, outcomeKnown: true}
	}
}

type HTTPProviderCallbackConfig struct {
	Secret       []byte
	MaxClockSkew time.Duration
}

type HTTPProviderCallbackHandler struct {
	store        ports.ExternalProviderCallbackStore
	secret       []byte
	maxClockSkew time.Duration
}

func NewHTTPProviderCallbackHandler(
	store ports.ExternalProviderCallbackStore,
	config HTTPProviderCallbackConfig,
) (*HTTPProviderCallbackHandler, error) {
	if store == nil || len(config.Secret) < 32 {
		return nil, domain.ErrExternalDeliveryInvalid
	}
	skew := config.MaxClockSkew
	if skew == 0 {
		skew = 5 * time.Minute
	}
	if skew < 30*time.Second || skew > 15*time.Minute {
		return nil, domain.ErrExternalDeliveryInvalid
	}
	secret := append([]byte(nil), config.Secret...)
	return &HTTPProviderCallbackHandler{store: store, secret: secret, maxClockSkew: skew}, nil
}

func (handler *HTTPProviderCallbackHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if handler == nil || handler.store == nil {
		http.Error(writer, "callback unavailable", http.StatusServiceUnavailable)
		return
	}
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if contentType := request.Header.Get("Content-Type"); !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		http.Error(writer, "unsupported media type", http.StatusUnsupportedMediaType)
		return
	}
	if request.ContentLength > externalCallbackMaxBodyBytes {
		http.Error(writer, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, externalCallbackMaxBodyBytes+1))
	if err != nil || int64(len(body)) > externalCallbackMaxBodyBytes {
		http.Error(writer, "request too large", http.StatusRequestEntityTooLarge)
		return
	}
	timestamp := request.Header.Get(externalCallbackTimestampHeader)
	if !handler.validTimestamp(timestamp, time.Now().UTC()) ||
		!handler.validSignature(timestamp, body, request.Header.Get(externalCallbackSignatureHeader)) {
		http.Error(writer, "unauthorized callback", http.StatusUnauthorized)
		return
	}

	callback, err := decodeExternalProviderCallback(body)
	if err != nil {
		http.Error(writer, "invalid callback", http.StatusBadRequest)
		return
	}
	_, err = handler.store.ApplyExternalProviderCallback(request.Context(), callback)
	switch {
	case err == nil:
		writer.WriteHeader(http.StatusNoContent)
	case errors.Is(err, domain.ErrExternalDeliveryInvalid):
		http.Error(writer, "invalid callback", http.StatusBadRequest)
	case errors.Is(err, domain.ErrExternalDeliveryUnavailable):
		http.Error(writer, "callback target unavailable", http.StatusNotFound)
	case errors.Is(err, domain.ErrExternalDeliveryLease):
		http.Error(writer, "callback state conflict", http.StatusConflict)
	default:
		http.Error(writer, "callback failed", http.StatusInternalServerError)
	}
}

func (handler *HTTPProviderCallbackHandler) validTimestamp(raw string, now time.Time) bool {
	seconds, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || seconds <= 0 {
		return false
	}
	delta := now.Sub(time.Unix(seconds, 0).UTC())
	return delta >= -handler.maxClockSkew && delta <= handler.maxClockSkew
}

func (handler *HTTPProviderCallbackHandler) validSignature(timestamp string, body []byte, raw string) bool {
	if !strings.HasPrefix(raw, "v1=") {
		return false
	}
	actual, err := hex.DecodeString(strings.TrimPrefix(raw, "v1="))
	if err != nil || len(actual) != sha256.Size {
		return false
	}
	mac := hmac.New(sha256.New, handler.secret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return hmac.Equal(mac.Sum(nil), actual)
}

func decodeExternalProviderCallback(body []byte) (domain.ExternalProviderCallback, error) {
	var payload map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		return domain.ExternalProviderCallback{}, domain.ErrExternalDeliveryInvalid
	}
	for key := range payload {
		switch key {
		case "task_id", "receipt_id", "status", "failure_code":
		default:
			return domain.ExternalProviderCallback{}, domain.ErrExternalDeliveryInvalid
		}
	}
	callback := domain.ExternalProviderCallback{
		TaskID:      payload["task_id"],
		ReceiptID:   payload["receipt_id"],
		Status:      domain.ExternalProviderCallbackStatus(payload["status"]),
		FailureCode: payload["failure_code"],
	}
	if err := callback.Validate(); err != nil {
		return domain.ExternalProviderCallback{}, err
	}
	return callback, nil
}
