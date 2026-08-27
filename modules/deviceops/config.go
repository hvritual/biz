package deviceops

import (
	"errors"
	"strings"
)

type BootstrapConfig struct {
	TenantID   string
	TenantName string
	UserID     string
	Email      string
	Token      string
	SiteID     string
	SiteName   string
}

type Config struct {
	HTTPListenAddress string
	GRPCListenAddress string
	AutoMigrate       bool
	Bootstrap         BootstrapConfig
}

func DefaultConfig() Config {
	return Config{HTTPListenAddress: "127.0.0.1:8080", GRPCListenAddress: "127.0.0.1:9090"}
}
func (config Config) Validate() error {
	if strings.TrimSpace(config.HTTPListenAddress) == "" || strings.TrimSpace(config.GRPCListenAddress) == "" {
		return errors.New("deviceops: HTTP and gRPC listen addresses are required")
	}
	if config.Bootstrap.Token == "" {
		return nil
	}
	if strings.TrimSpace(config.Bootstrap.TenantID) == "" || strings.TrimSpace(config.Bootstrap.TenantName) == "" || strings.TrimSpace(config.Bootstrap.UserID) == "" || strings.TrimSpace(config.Bootstrap.Email) == "" || strings.TrimSpace(config.Bootstrap.SiteID) == "" || strings.TrimSpace(config.Bootstrap.SiteName) == "" {
		return errors.New("deviceops: complete bootstrap identity and site are required")
	}
	return nil
}
