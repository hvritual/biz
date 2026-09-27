package qualification

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hvritual/biz/internal/notification/domain"
	"github.com/hvritual/biz/internal/notification/infrastructure/delivery"
)

const CallbackPath = "/callbacks/notification/provider"

type ProviderConfig struct {
	ProviderID          string
	ProviderEndpoint    string
	ProviderBearerToken string
	ProviderIdempotent  bool
	Channel             string
	Destination         string

	CallbackListenAddress string
	CallbackPublicURL     string
	CallbackHMACSecret    []byte

	RequestTimeout time.Duration
	Timeout        time.Duration
}

func (config ProviderConfig) normalized() ProviderConfig {
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 15 * time.Second
	}
	if config.Timeout == 0 {
		config.Timeout = 2 * time.Minute
	}
	return config
}

func (config ProviderConfig) Validate() error {
	config = config.normalized()
	if !safeID(config.ProviderID, 64) {
		return errors.New("notification qualification: provider id is required")
	}
	if err := requireExternalHTTPS(config.ProviderEndpoint, false); err != nil {
		return errors.New("notification qualification: provider endpoint must be external HTTPS")
	}
	if strings.TrimSpace(config.ProviderBearerToken) == "" ||
		strings.ContainsAny(config.ProviderBearerToken, "\r\n") {
		return errors.New("notification qualification: provider bearer token is required")
	}
	if config.Channel != "email" && config.Channel != "sms" {
		return errors.New("notification qualification: channel must be email or sms")
	}
	if strings.TrimSpace(config.Destination) == "" ||
		len(config.Destination) > 512 ||
		strings.ContainsAny(config.Destination, "\r\n") {
		return errors.New("notification qualification: destination is required")
	}
	if strings.TrimSpace(config.CallbackListenAddress) == "" {
		return errors.New("notification qualification: callback listen address is required")
	}
	if err := requireExternalHTTPS(config.CallbackPublicURL, true); err != nil {
		return errors.New("notification qualification: callback public URL must be external HTTPS with the canonical callback path")
	}
	if len(config.CallbackHMACSecret) < 32 {
		return errors.New("notification qualification: callback HMAC secret must be at least 32 bytes")
	}
	if config.RequestTimeout < time.Second || config.RequestTimeout > time.Minute {
		return errors.New("notification qualification: invalid provider request timeout")
	}
	if config.Timeout < 5*time.Second || config.Timeout > 10*time.Minute ||
		config.RequestTimeout >= config.Timeout {
		return errors.New("notification qualification: invalid qualification timeout")
	}
	return nil
}

func requireExternalHTTPS(raw string, callback bool) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" ||
		parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return errors.New("invalid external https url")
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	if strings.EqualFold(host, "localhost") || (ip != nil && (ip.IsLoopback() || ip.IsPrivate())) {
		return errors.New("loopback/private endpoint is not external qualification")
	}
	if callback && parsed.Path != CallbackPath {
		return errors.New("callback path mismatch")
	}
	return nil
}

func safeID(value string, max int) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > max {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') &&
			(char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') &&
			char != '-' && char != '_' && char != '.' {
			return false
		}
	}
	return true
}

type Receipt struct {
	SchemaVersion       int
	State               string
	ProviderID          string
	ProviderHost        string
	ProviderIdempotent  bool
	Channel             string
	DestinationHash     string
	TaskID              string
	TraceID             string
	ProviderReceiptHash string
	ProviderStatus      string
	TerminalStatus      string
	FailureCode         string
	CallbackHost        string
	CallbackObservedAt  time.Time
	ElapsedMillis       int64
	ObservedAt          time.Time
}

type callbackObservation struct {
	status      domain.ExternalProviderCallbackStatus
	failureCode string
	observedAt  time.Time
}

type callbackStore struct {
	taskID string

	receiptMu sync.RWMutex
	receiptID string
	ready     chan struct{}
	readyOnce sync.Once

	resultMu sync.Mutex
	result   *callbackObservation
	done     chan struct{}
	doneOnce sync.Once
}

func newCallbackStore(taskID string) *callbackStore {
	return &callbackStore{
		taskID: taskID,
		ready:  make(chan struct{}),
		done:   make(chan struct{}),
	}
}

func (store *callbackStore) arm(receiptID string) {
	store.readyOnce.Do(func() {
		store.receiptMu.Lock()
		store.receiptID = receiptID
		store.receiptMu.Unlock()
		close(store.ready)
	})
}

func (store *callbackStore) ApplyExternalProviderCallback(
	ctx context.Context,
	callback domain.ExternalProviderCallback,
) (domain.ExternalTaskReceipt, error) {
	select {
	case <-store.ready:
	case <-ctx.Done():
		return domain.ExternalTaskReceipt{}, ctx.Err()
	}
	store.receiptMu.RLock()
	receiptID := store.receiptID
	store.receiptMu.RUnlock()
	if callback.TaskID != store.taskID || callback.ReceiptID != receiptID {
		return domain.ExternalTaskReceipt{}, domain.ErrExternalDeliveryLease
	}

	store.resultMu.Lock()
	defer store.resultMu.Unlock()
	if store.result != nil {
		if store.result.status != callback.Status || store.result.failureCode != callback.FailureCode {
			return domain.ExternalTaskReceipt{}, domain.ErrExternalDeliveryLease
		}
		return callbackReceipt(callback), nil
	}
	store.result = &callbackObservation{
		status:      callback.Status,
		failureCode: callback.FailureCode,
		observedAt:  time.Now().UTC(),
	}
	store.doneOnce.Do(func() { close(store.done) })
	return callbackReceipt(callback), nil
}

func callbackReceipt(callback domain.ExternalProviderCallback) domain.ExternalTaskReceipt {
	state := domain.ExternalTaskStateDelivered
	if callback.Status == domain.ExternalProviderCallbackFailed {
		state = domain.ExternalTaskStateManualReview
	}
	return domain.ExternalTaskReceipt{
		TaskID:          callback.TaskID,
		State:           state,
		FailureCode:     callback.FailureCode,
		ProviderReceipt: callback.ReceiptID,
	}
}

func (store *callbackStore) observation() (callbackObservation, bool) {
	store.resultMu.Lock()
	defer store.resultMu.Unlock()
	if store.result == nil {
		return callbackObservation{}, false
	}
	return *store.result, true
}

func RunProvider(ctx context.Context, config ProviderConfig) (Receipt, error) {
	config = config.normalized()
	if err := config.Validate(); err != nil {
		return Receipt{}, err
	}
	suffix, err := randomHex(12)
	if err != nil {
		return Receipt{}, err
	}
	taskID := "provider-qualification-" + suffix
	traceID := "provider-qualification-trace-" + suffix
	startedAt := time.Now().UTC()
	receipt := Receipt{
		SchemaVersion:      1,
		State:              "RUNNING",
		ProviderID:         config.ProviderID,
		ProviderHost:       hostOnly(config.ProviderEndpoint),
		ProviderIdempotent: config.ProviderIdempotent,
		Channel:            config.Channel,
		DestinationHash:    sha256Hex(config.Destination),
		TaskID:             taskID,
		TraceID:            traceID,
		CallbackHost:       hostOnly(config.CallbackPublicURL),
		ObservedAt:         startedAt,
	}

	runCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	store := newCallbackStore(taskID)
	defer store.arm("")
	callback, err := delivery.NewHTTPProviderCallbackHandler(
		store,
		delivery.HTTPProviderCallbackConfig{Secret: config.CallbackHMACSecret},
	)
	if err != nil {
		receipt.State = "BLOCKED"
		return finish(receipt, startedAt), err
	}
	listener, err := net.Listen("tcp", config.CallbackListenAddress)
	if err != nil {
		receipt.State = "BLOCKED"
		return finish(receipt, startedAt), errors.New("notification qualification: callback listener unavailable")
	}
	server := &http.Server{
		Handler:           qualificationMux(callback),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		_ = server.Serve(listener)
	}()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	provider, err := delivery.NewHTTPProvider(
		delivery.HTTPProviderConfig{
			Endpoint:    config.ProviderEndpoint,
			BearerToken: config.ProviderBearerToken,
			Idempotent:  config.ProviderIdempotent,
		},
		&http.Client{Timeout: config.RequestTimeout},
	)
	if err != nil {
		receipt.State = "BLOCKED"
		return finish(receipt, startedAt), err
	}
	result, err := provider.SendExternalNotification(runCtx, domain.ExternalProviderRequest{
		TaskID:        taskID,
		EventID:       "provider-qualification-event-" + suffix,
		TenantID:      "provider-qualification",
		UserID:        "provider-qualification",
		Channel:       config.Channel,
		Destination:   config.Destination,
		TypeCode:      "system.announcement",
		Level:         domain.LevelGeneral,
		TraceID:       traceID,
		ReferenceKind: "qualification",
		ReferenceID:   "provider-" + suffix,
	})
	if err != nil {
		store.arm("")
		receipt.State = "FAIL"
		if failure, ok := err.(interface{ FailureCode() string }); ok {
			receipt.FailureCode = failure.FailureCode()
		}
		return finish(receipt, startedAt), errors.New("notification qualification: provider request failed")
	}
	receipt.ProviderStatus = string(result.Status)
	receipt.ProviderReceiptHash = sha256Hex(result.ReceiptID)
	store.arm(result.ReceiptID)

	select {
	case <-store.done:
		observation, ok := store.observation()
		if !ok {
			receipt.State = "FAIL"
			return finish(receipt, startedAt), errors.New("notification qualification: callback result unavailable")
		}
		receipt.CallbackObservedAt = observation.observedAt
		receipt.TerminalStatus = string(observation.status)
		receipt.FailureCode = observation.failureCode
		if observation.status != domain.ExternalProviderCallbackDelivered {
			receipt.State = "FAIL"
			return finish(receipt, startedAt), errors.New("notification qualification: provider reported terminal failure")
		}
		receipt.State = "PASS"
		return finish(receipt, startedAt), nil
	case <-runCtx.Done():
		receipt.State = "FAIL"
		receipt.FailureCode = "CALLBACK_TIMEOUT"
		return finish(receipt, startedAt), errors.New("notification qualification: terminal callback not observed before timeout")
	}
}

func qualificationMux(callback http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("POST "+CallbackPath, callback)
	return mux
}

func randomHex(bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func hostOnly(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func finish(receipt Receipt, startedAt time.Time) Receipt {
	receipt.ElapsedMillis = time.Since(startedAt).Milliseconds()
	return receipt
}
