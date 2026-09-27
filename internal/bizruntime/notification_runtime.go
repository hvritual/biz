package bizruntime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	devicepersistence "github.com/hvritual/biz/internal/deviceops/infrastructure/persistence"
	notificationapp "github.com/hvritual/biz/internal/notification/application"
	notificationdomain "github.com/hvritual/biz/internal/notification/domain"
	notificationdelivery "github.com/hvritual/biz/internal/notification/infrastructure/delivery"
	notificationpersistence "github.com/hvritual/biz/internal/notification/infrastructure/persistence"
	notificationports "github.com/hvritual/biz/internal/notification/ports"
	accessmodule "github.com/hvritual/biz/modules/access"
	"github.com/hvritual/biz/modules/deviceops"
	"yunka.io/framework/core"
	"yunka.io/framework/platform"
)

const notificationProviderCallbackPath = "/callbacks/notification/provider"

type NotificationRuntimeOptions struct {
	ProviderEndpoint     string
	ProviderBearerToken  string
	ProviderIdempotent   bool
	Channels             []string
	CallbackHMACSecret   []byte
	PollInterval         time.Duration
	RoutingLeaseDuration time.Duration
}

func (options NotificationRuntimeOptions) Enabled() bool {
	return strings.TrimSpace(options.ProviderEndpoint) != ""
}

func (options NotificationRuntimeOptions) normalized() NotificationRuntimeOptions {
	if options.PollInterval == 0 {
		options.PollInterval = 100 * time.Millisecond
	}
	if options.RoutingLeaseDuration == 0 {
		options.RoutingLeaseDuration = time.Minute
	}
	options.Channels = append([]string(nil), options.Channels...)
	sort.Strings(options.Channels)
	return options
}

func (options NotificationRuntimeOptions) Validate() error {
	options = options.normalized()
	if options.PollInterval < 10*time.Millisecond || options.PollInterval > time.Minute {
		return errors.New("notification runtime: invalid poll interval")
	}
	if options.RoutingLeaseDuration < 5*time.Second || options.RoutingLeaseDuration > 5*time.Minute {
		return errors.New("notification runtime: invalid routing lease duration")
	}
	if !options.Enabled() {
		if strings.TrimSpace(options.ProviderBearerToken) != "" || len(options.CallbackHMACSecret) != 0 ||
			len(options.Channels) != 0 || options.ProviderIdempotent {
			return errors.New("notification runtime: provider configuration is incomplete")
		}
		return nil
	}
	if strings.TrimSpace(options.ProviderBearerToken) == "" {
		return errors.New("notification runtime: provider bearer token is required")
	}
	if len(options.CallbackHMACSecret) < 32 {
		return errors.New("notification runtime: callback HMAC secret must be at least 32 bytes")
	}
	if len(options.Channels) < 1 || len(options.Channels) > 2 {
		return errors.New("notification runtime: at least one explicit external channel is required")
	}
	seen := map[string]bool{}
	for _, channel := range options.Channels {
		if (channel != "email" && channel != "sms") || seen[channel] {
			return errors.New("notification runtime: channels must be unique email/sms values")
		}
		seen[channel] = true
	}
	return nil
}

type notificationCatalogSnapshot struct {
	types    *notificationdomain.MessageTypeCatalog
	channels *notificationdomain.ChannelRegistry
}

func buildNotificationCatalogSnapshot(options NotificationRuntimeOptions) (notificationCatalogSnapshot, error) {
	types, err := notificationdomain.NewMessageTypeCatalog([]notificationdomain.MessageType{
		{Code: "device.fault", Name: "设备故障", Level: notificationdomain.LevelUrgent},
		{Code: "device.offline", Name: "设备通信离线", Level: notificationdomain.LevelImportant},
		{Code: "service.maintenance_due", Name: "维护到期提醒", Level: notificationdomain.LevelImportant},
		{Code: "device.connection_recovered", Name: "设备通信恢复", Level: notificationdomain.LevelGeneral},
		{Code: "system.announcement", Name: "企业系统通知", Level: notificationdomain.LevelGeneral},
	})
	if err != nil {
		return notificationCatalogSnapshot{}, err
	}
	enabled := map[string]bool{}
	if options.Enabled() {
		for _, channel := range options.normalized().Channels {
			enabled[channel] = true
		}
	}
	channel := func(code, name, unavailable string) notificationdomain.Channel {
		if enabled[code] {
			return notificationdomain.Channel{Code: code, Name: name, Availability: notificationdomain.ChannelConfigurable}
		}
		return notificationdomain.Channel{Code: code, Name: name, Availability: notificationdomain.ChannelNotConfigurable, UnavailableReason: unavailable}
	}
	channels, err := notificationdomain.NewChannelRegistry([]notificationdomain.Channel{
		{Code: "in_app", Name: "站内消息", Availability: notificationdomain.ChannelConfigurable},
		channel("sms", "短信", "短信投递适配器尚未配置"),
		channel("email", "邮件", "邮件投递适配器尚未配置"),
	})
	if err != nil {
		return notificationCatalogSnapshot{}, err
	}
	return notificationCatalogSnapshot{types: types, channels: channels}, nil
}

type NotificationTick struct {
	RoutedEventID     string
	ExternalTaskID    string
	ExternalTaskState string
}

type notificationRuntime struct {
	options          NotificationRuntimeOptions
	catalogs         notificationCatalogSnapshot
	protection       *accesspersistence.ContactProtection
	externalProvider notificationports.ExternalNotificationProvider
	routerWorkerID   string
	externalWorkerID string

	runMu sync.Mutex
	mu    sync.RWMutex

	router         *notificationapp.BusinessEventRouter
	inbox          *notificationapp.InboxService
	externalWorker *notificationapp.ExternalDeliveryWorker
	callback       http.Handler
	cancel         context.CancelFunc
	done           chan struct{}
	started        bool
	lastError      error
}

func newNotificationRuntime(
	options NotificationRuntimeOptions,
	catalogs notificationCatalogSnapshot,
	protection *accesspersistence.ContactProtection,
) (*notificationRuntime, error) {
	options = options.normalized()
	if err := options.Validate(); err != nil {
		return nil, err
	}
	var provider notificationports.ExternalNotificationProvider
	if options.Enabled() {
		if protection == nil {
			return nil, accesspersistence.ErrSensitiveDataKeyUnavailable
		}
		httpProvider, err := notificationdelivery.NewHTTPProvider(notificationdelivery.HTTPProviderConfig{
			Endpoint: options.ProviderEndpoint, BearerToken: options.ProviderBearerToken, Idempotent: options.ProviderIdempotent,
		}, nil)
		if err != nil {
			return nil, err
		}
		provider = httpProvider
	}
	var nonce [12]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	suffix := hex.EncodeToString(nonce[:])
	return &notificationRuntime{
		options: options, catalogs: catalogs, protection: protection, externalProvider: provider,
		routerWorkerID: "notification-router-" + suffix, externalWorkerID: "notification-external-" + suffix,
	}, nil
}

func (runtime *notificationRuntime) providerEnabled() bool {
	return runtime != nil && runtime.externalProvider != nil
}

func (runtime *notificationRuntime) bind(
	ctx context.Context,
	provider *platform.Provider,
	autoMigrate bool,
) error {
	if runtime == nil || provider == nil {
		return errors.New("notification runtime: binding unavailable")
	}
	accessContext, err := provider.ForModule(accessmodule.GeneratedDescriptor())
	if err != nil {
		return err
	}
	accessDB, err := accessContext.Databases().GORM("primary")
	if err != nil {
		return err
	}
	deviceContext, err := provider.ForModule(deviceops.GeneratedDescriptor())
	if err != nil {
		return err
	}
	deviceDB, err := deviceContext.Databases().GORM("primary")
	if err != nil {
		return err
	}
	if autoMigrate {
		if err := notificationpersistence.MigrateRouting(ctx, accessDB); err != nil {
			return err
		}
	}
	var accessStore *accesspersistence.Store
	if runtime.providerEnabled() {
		accessStore, err = accesspersistence.NewWithContactProtection(accessDB, runtime.protection)
	} else {
		accessStore, err = accesspersistence.New(accessDB)
	}
	if err != nil {
		return err
	}
	recipients, err := accesspersistence.NewNotificationRouteRecipientDirectory(accessDB)
	if err != nil {
		return err
	}
	groups, err := devicepersistence.NewNotificationRouteGroupDirectory(deviceDB)
	if err != nil {
		return err
	}
	routing, err := notificationpersistence.NewRoutingRepository(accessDB)
	if err != nil {
		return err
	}
	router, err := notificationapp.NewBusinessEventRouter(notificationports.RoutingDependencies{
		Queue: routing, Configurations: routing, Recipients: recipients, Groups: groups, Preferences: accessStore,
	}, runtime.catalogs.types, runtime.catalogs.channels, runtime.options.RoutingLeaseDuration)
	if err != nil {
		return err
	}
	inbox, err := notificationapp.NewInboxService(routing)
	if err != nil {
		return err
	}
	var externalWorker *notificationapp.ExternalDeliveryWorker
	var callback http.Handler
	if runtime.providerEnabled() {
		externalWorker, err = notificationapp.NewExternalDeliveryWorker(
			notificationports.ExternalDeliveryDependencies{Tasks: routing, Admission: accessStore, Provider: runtime.externalProvider},
			notificationdomain.EnterpriseExternalDeliveryPolicy(),
			runtime.externalWorkerID,
		)
		if err != nil {
			return err
		}
		callback, err = notificationdelivery.NewHTTPProviderCallbackHandler(
			routing,
			notificationdelivery.HTTPProviderCallbackConfig{Secret: runtime.options.CallbackHMACSecret},
		)
		if err != nil {
			return err
		}
	}

	runtime.mu.Lock()
	runtime.router = router
	runtime.inbox = inbox
	runtime.externalWorker = externalWorker
	runtime.callback = callback
	runtime.mu.Unlock()
	return nil
}

func (runtime *notificationRuntime) inboxService() *notificationapp.InboxService {
	if runtime == nil {
		return nil
	}
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	return runtime.inbox
}

func (runtime *notificationRuntime) component() core.RuntimeComponent {
	return core.RuntimeComponent{
		Name:         "notification-runtime",
		StartFunc:    runtime.start,
		HealthFunc:   runtime.health,
		ShutdownFunc: runtime.shutdown,
	}
}

func (runtime *notificationRuntime) callbackHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		runtime.mu.RLock()
		callback := runtime.callback
		runtime.mu.RUnlock()
		if callback == nil {
			http.Error(writer, "notification callback unavailable", http.StatusServiceUnavailable)
			return
		}
		callback.ServeHTTP(writer, request)
	})
}

func (runtime *notificationRuntime) start(context.Context) error {
	if runtime == nil {
		return errors.New("notification runtime: unavailable")
	}
	runtime.mu.Lock()
	if runtime.router == nil || runtime.inbox == nil ||
		(runtime.providerEnabled() && (runtime.externalWorker == nil || runtime.callback == nil)) {
		runtime.mu.Unlock()
		return errors.New("notification runtime: binding incomplete")
	}
	if runtime.started {
		runtime.mu.Unlock()
		return nil
	}
	runtime.started = true
	runtime.done = make(chan struct{})
	loop, cancel := context.WithCancel(context.Background())
	runtime.cancel = cancel
	runtime.mu.Unlock()

	go runtime.loop(loop)
	return nil
}

func (runtime *notificationRuntime) loop(ctx context.Context) {
	defer func() {
		runtime.mu.RLock()
		done := runtime.done
		runtime.mu.RUnlock()
		if done != nil {
			close(done)
		}
	}()
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		tick, err := runtime.tick(ctx)
		runtime.mu.Lock()
		runtime.lastError = err
		runtime.mu.Unlock()
		if err != nil && ctx.Err() == nil {
			slog.Warn("notification runtime tick failed")
		}
		if err == nil && (tick.RoutedEventID != "" || tick.ExternalTaskID != "") {
			timer.Reset(time.Millisecond)
			continue
		}
		timer.Reset(runtime.options.PollInterval)
	}
}

func (runtime *notificationRuntime) tick(ctx context.Context) (NotificationTick, error) {
	runtime.runMu.Lock()
	defer runtime.runMu.Unlock()

	runtime.mu.RLock()
	router, externalWorker := runtime.router, runtime.externalWorker
	runtime.mu.RUnlock()
	if router == nil {
		return NotificationTick{}, errors.New("notification runtime: binding incomplete")
	}
	routeResult, routeErr := router.RouteOnce(ctx, runtime.routerWorkerID)
	if externalWorker == nil {
		return NotificationTick{RoutedEventID: routeResult.EventID}, routeErr
	}
	deliveryResult, deliveryErr := externalWorker.RunOnce(ctx)
	return NotificationTick{
		RoutedEventID:     routeResult.EventID,
		ExternalTaskID:    deliveryResult.TaskID,
		ExternalTaskState: deliveryResult.State,
	}, errors.Join(routeErr, deliveryErr)
}

func (runtime *notificationRuntime) health(context.Context) error {
	if runtime == nil {
		return errors.New("notification runtime: unavailable")
	}
	runtime.mu.RLock()
	defer runtime.mu.RUnlock()
	if !runtime.started {
		return errors.New("notification runtime: not started")
	}
	return runtime.lastError
}

func (runtime *notificationRuntime) shutdown(ctx context.Context) error {
	if runtime == nil {
		return nil
	}
	runtime.mu.RLock()
	cancel, done := runtime.cancel, runtime.done
	runtime.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (started *Started) RunNotificationOnce(ctx context.Context) (NotificationTick, error) {
	if started == nil || started.notificationRunner == nil {
		return NotificationTick{}, errors.New("notification runtime unavailable")
	}
	return started.notificationRunner.tick(ctx)
}
