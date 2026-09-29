// Package paymentchannel provides provider-neutral payment instructions for
// commercial orders. Network clients, credentials and webhook verification are
// injected by the deployment composition rather than this domain boundary.
package paymentchannel

import (
	"errors"
	"net/url"
	"strings"

	"github.com/hvritual/biz/internal/commercial/domain/payment"
)

const (
	QRCode   = "QR_CODE"
	Redirect = "REDIRECT"
)

var (
	ErrInvalidConfig = errors.New("payment channel: invalid configuration")
	ErrOrderProvider = errors.New("payment channel: order provider mismatch")
)

// Config contains public deployment endpoints only. Merchant certificates,
// private keys and API secrets stay in a runtime-specific client implementation.
type Config struct {
	NotifyURL string
	ReturnURL string
}

type Instruction struct {
	Provider    string
	Mode        string
	OrderID     string
	AmountMinor uint64
	Currency    string
	NotifyURL   string
	ReturnURL   string
}

type Channel interface {
	Provider() string
	CreateInstruction(payment.Order) (Instruction, error)
}

type adapter struct {
	provider string
	mode     string
	config   Config
}

func validURL(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func NewWechatNative(config Config) (Channel, error) {
	if !validURL(config.NotifyURL) || config.ReturnURL != "" {
		return nil, ErrInvalidConfig
	}
	return adapter{provider: payment.WeChatNative, mode: QRCode, config: config}, nil
}

func NewAlipayPage(config Config) (Channel, error) {
	if !validURL(config.NotifyURL) || !validURL(config.ReturnURL) {
		return nil, ErrInvalidConfig
	}
	return adapter{provider: payment.AlipayPage, mode: Redirect, config: config}, nil
}

func (a adapter) Provider() string { return a.provider }

func (a adapter) CreateInstruction(order payment.Order) (Instruction, error) {
	if err := order.Validate(); err != nil {
		return Instruction{}, err
	}
	if order.Provider != a.provider || strings.TrimSpace(order.ID) == "" {
		return Instruction{}, ErrOrderProvider
	}
	return Instruction{Provider: a.provider, Mode: a.mode, OrderID: order.ID, AmountMinor: order.AmountMinor, Currency: order.Currency, NotifyURL: a.config.NotifyURL, ReturnURL: a.config.ReturnURL}, nil
}
