package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	accessapp "github.com/hvritual/biz/internal/access/application"
	accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
	"github.com/hvritual/biz/internal/bizruntime"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	dsn := strings.TrimSpace(os.Getenv("YUNKA_BIZ_MYSQL_DSN"))
	if dsn == "" {
		return errors.New("YUNKA_BIZ_MYSQL_DSN is required")
	}
	publicURL := strings.TrimRight(strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_PUBLIC_URL")), "/")
	if publicURL == "" {
		return errors.New("YUNKA_BIZ_IDP_PUBLIC_URL is required")
	}
	redirectURL := strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_REDIRECT_URL"))
	if redirectURL == "" {
		return errors.New("YUNKA_BIZ_IDP_REDIRECT_URL is required")
	}
	signingKeyFile := strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_SIGNING_KEY_FILE"))
	if signingKeyFile == "" {
		return errors.New("YUNKA_BIZ_IDP_SIGNING_KEY_FILE is required")
	}
	signingKey, err := os.ReadFile(signingKeyFile)
	if err != nil {
		return fmt.Errorf("read IdP signing key: %w", err)
	}
	previousSigningKeys, err := readPreviousSigningKeys()
	if err != nil {
		return err
	}
	config := bizruntime.FirstPartyIdPConfig{
		PublicURL:             publicURL,
		ClientID:              envOr("YUNKA_BIZ_IDP_CLIENT_ID", "biz-web"),
		RedirectURL:           redirectURL,
		PostLogoutRedirectURL: strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_POST_LOGOUT_REDIRECT_URL")),
		SigningKeyPEM:         string(signingKey),
		SigningKeyID:          strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_SIGNING_KEY_ID")),
		PreviousSigningKeys:   previousSigningKeys,
		LoginTTL:              envDuration("YUNKA_BIZ_IDP_LOGIN_TTL", 5*time.Minute),
		CodeTTL:               envDuration("YUNKA_BIZ_IDP_CODE_TTL", 90*time.Second),
		TokenTTL:              envDuration("YUNKA_BIZ_IDP_TOKEN_TTL", 5*time.Minute),
		RememberIdentifierTTL: envOptionalDuration("YUNKA_BIZ_IDP_REMEMBER_IDENTIFIER_TTL"),
		CookieSecure:          envBool("YUNKA_BIZ_IDP_COOKIE_SECURE", true),
		PrivacyConsent: bizruntime.FirstPartyPrivacyConsentConfig{
			AgreementVersion: strings.TrimSpace(os.Getenv("YUNKA_BIZ_PRIVACY_AGREEMENT_VERSION")),
			PrivacyPolicyURL: strings.TrimSpace(os.Getenv("YUNKA_BIZ_PRIVACY_POLICY_URL")),
			TermsURL:         strings.TrimSpace(os.Getenv("YUNKA_BIZ_TERMS_URL")),
			ReconsentPolicy:  bizruntime.PrivacyReconsentPolicy(strings.TrimSpace(os.Getenv("YUNKA_BIZ_PRIVACY_RECONSENT_POLICY"))),
		},
	}
	if err := config.Validate(); err != nil {
		return err
	}
	contactProtection, err := bizruntime.BuildContactProtection(
		os.Getenv("YUNKA_BIZ_PII_ACTIVE_KEY_VERSION"),
		os.Getenv("YUNKA_BIZ_PII_KEYS_JSON"),
		os.Getenv("YUNKA_BIZ_PII_LOOKUP_KEY_B64"),
	)
	if err != nil {
		return err
	}
	verificationConfig, err := verificationSecurityConfigFromEnv()
	if err != nil {
		return err
	}
	verificationProtection, err := bizruntime.BuildVerificationProtection(
		os.Getenv("YUNKA_BIZ_VERIFICATION_ACTIVE_KEY_VERSION"),
		os.Getenv("YUNKA_BIZ_VERIFICATION_KEYS_JSON"),
		os.Getenv("YUNKA_BIZ_VERIFICATION_HMAC_KEY_B64"),
	)
	if err != nil {
		return err
	}
	if verificationConfig.Enabled() {
		if verificationProtection == nil {
			return errors.New("verification protection keys are required when native OTP login is configured")
		}
		config.OTPCodeDigits = verificationConfig.CodeDigits
		config.RecoveryAuthorizationTTL = verificationConfig.AuthorizationTTL
	}
	database, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("open IdP database: %w", err)
	}
	var store *accesspersistence.Store
	if contactProtection == nil {
		store, err = accesspersistence.New(database)
	} else {
		store, err = accesspersistence.NewWithContactProtection(database, contactProtection)
	}
	if err != nil {
		return err
	}
	if envBool("YUNKA_BIZ_IDP_AUTO_MIGRATE", false) {
		if err := store.EnsureFirstPartyIDPSchema(context.Background()); err != nil {
			return fmt.Errorf("migrate IdP schema: %w", err)
		}
		if err := store.EnsureFirstPartyIDPSecuritySchema(context.Background()); err != nil {
			return fmt.Errorf("migrate IdP security schema: %w", err)
		}
		if err := store.EnsureMemberActivationSchema(context.Background()); err != nil {
			return fmt.Errorf("migrate member activation schema: %w", err)
		}
		if verificationProtection != nil {
			repository, err := accesspersistence.NewVerificationRepository(database, verificationProtection)
			if err != nil {
				return err
			}
			if err := repository.EnsureSchema(context.Background()); err != nil {
				return fmt.Errorf("migrate verification schema: %w", err)
			}
		}
	}
	var verificationService *accessapp.VerificationService
	sender := qualificationNotificationSender()
	if sender != nil {
		verificationService, err = bizruntime.BuildVerificationService(database, verificationConfig, verificationProtection, sender)
		if err != nil {
			return err
		}
	}
	handler, err := bizruntime.NewFirstPartyIdPHandlerWithSecurity(config, store, verificationService, verificationProtection)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:              envOr("YUNKA_BIZ_IDP_LISTEN", "127.0.0.1:8081"),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		return server.Shutdown(shutdown)
	}
}

func readPreviousSigningKeys() ([]bizruntime.FirstPartyIdPVerificationKey, error) {
	rawFiles := strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_PREVIOUS_SIGNING_KEY_FILES"))
	if rawFiles == "" {
		return nil, nil
	}
	files := strings.Split(rawFiles, ",")
	ids := strings.Split(strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_PREVIOUS_SIGNING_KEY_IDS")), ",")
	keys := make([]bizruntime.FirstPartyIdPVerificationKey, 0, len(files))
	for index, rawFile := range files {
		file := strings.TrimSpace(rawFile)
		if file == "" {
			return nil, errors.New("YUNKA_BIZ_IDP_PREVIOUS_SIGNING_KEY_FILES contains an empty path")
		}
		pemBytes, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read previous IdP signing key %s: %w", file, err)
		}
		keyID := ""
		if index < len(ids) {
			keyID = strings.TrimSpace(ids[index])
		}
		keys = append(keys, bizruntime.FirstPartyIdPVerificationKey{PEM: string(pemBytes), KeyID: keyID})
	}
	return keys, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envOptionalDuration(name string) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed < 0 {
		return -1
	}
	return parsed
}

func envOptionalInt(name string) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return parsed, nil
}

func verificationSecurityConfigFromEnv() (bizruntime.VerificationSecurityConfig, error) {
	maxSends, err := envOptionalInt("YUNKA_BIZ_VERIFICATION_MAX_SENDS_PER_WINDOW")
	if err != nil {
		return bizruntime.VerificationSecurityConfig{}, err
	}
	maxTries, err := envOptionalInt("YUNKA_BIZ_VERIFICATION_MAX_TRIES")
	if err != nil {
		return bizruntime.VerificationSecurityConfig{}, err
	}
	codeDigits, err := envOptionalInt("YUNKA_BIZ_VERIFICATION_CODE_DIGITS")
	if err != nil {
		return bizruntime.VerificationSecurityConfig{}, err
	}
	config := bizruntime.VerificationSecurityConfig{
		CodeTTL:              envOptionalDuration("YUNKA_BIZ_VERIFICATION_CODE_TTL"),
		AuthorizationTTL:     envOptionalDuration("YUNKA_BIZ_VERIFICATION_AUTHORIZATION_TTL"),
		ResendInterval:       envOptionalDuration("YUNKA_BIZ_VERIFICATION_RESEND_INTERVAL"),
		SendLimitWindow:      envOptionalDuration("YUNKA_BIZ_VERIFICATION_SEND_LIMIT_WINDOW"),
		MaxSendsPerWindow:    maxSends,
		MaxVerificationTries: maxTries,
		CodeDigits:           codeDigits,
		Notification: bizruntime.SecurityNotificationProviderConfig{
			Provider:      strings.TrimSpace(os.Getenv("YUNKA_BIZ_SECURITY_NOTIFICATION_PROVIDER")),
			Endpoint:      strings.TrimSpace(os.Getenv("YUNKA_BIZ_SECURITY_NOTIFICATION_ENDPOINT")),
			SenderID:      strings.TrimSpace(os.Getenv("YUNKA_BIZ_SECURITY_NOTIFICATION_SENDER_ID")),
			CredentialRef: strings.TrimSpace(os.Getenv("YUNKA_BIZ_SECURITY_NOTIFICATION_CREDENTIAL_REF")),
		},
	}
	if err := config.Validate(); err != nil {
		return bizruntime.VerificationSecurityConfig{}, err
	}
	return config, nil
}
