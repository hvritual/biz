from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def replace(path: str, old: str, new: str) -> None:
    target = ROOT / path
    text = target.read_text()
    if old not in text:
        raise SystemExit(f"anchor not found: {path}: {old[:100]!r}")
    target.write_text(text.replace(old, new, 1))


# Preserve the authentication provenance established by the server-side session.
replace(
    "internal/access/infrastructure/persistence/web_session.go",
    '''const (\n\tWebActorUser     = "user"\n\tWebActorPlatform = "platform"\n\t// Current Biz operation contracts classify server-issued credentials as\n\t// api-key authentication. A browser never receives an API key: after OIDC\n\t// verification the BFF replaces provider tokens with a server-side session,\n\t// and only that trusted server session is projected into this existing\n\t// contract authentication class. Session origin remains explicit in the Web\n\t// session context and cannot be supplied by an arbitrary header.\n\tAuthMethodWeb = identity.AuthMethodAPIKey\n)''',
    '''const (\n\tWebActorUser     = "user"\n\tWebActorPlatform = "platform"\n\t// A browser never supplies this value. The BFF sets it only after validating\n\t// the opaque server-side session and resolving the authoritative actor.\n\tAuthMethodWeb = "web-session"\n)''',
)

# Database faults remain availability errors; only an active throttle is deliberately
# indistinguishable from invalid credentials.
replace(
    "internal/access/infrastructure/persistence/first_party_idp_security.go",
    '''\tif err := store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {\n\t\tvar row firstPartyLoginThrottleRecord\n\t\terr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("identity_hash = ?", identityHash).First(&row).Error\n\t\tif err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {\n\t\t\treturn err\n\t\t}\n\t\tif err == nil && row.BlockedUntil != nil && row.BlockedUntil.After(now) {\n\t\t\treturn ErrInvalidUserCredentials\n\t\t}\n\t\treturn nil\n\t}); err != nil {\n\t\tconsumeDummyPasswordWork(password)\n\t\t_ = store.recordFirstPartyLoginAudit(ctx, now, "throttled", "", identityHash, sourceHash)\n\t\treturn LocalUserIdentity{}, ErrInvalidUserCredentials\n\t}''',
    '''\terr := store.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {\n\t\tvar row firstPartyLoginThrottleRecord\n\t\terr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("identity_hash = ?", identityHash).First(&row).Error\n\t\tif err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {\n\t\t\treturn err\n\t\t}\n\t\tif err == nil && row.BlockedUntil != nil && row.BlockedUntil.After(now) {\n\t\t\treturn ErrInvalidUserCredentials\n\t\t}\n\t\treturn nil\n\t})\n\tif err != nil {\n\t\tif !errors.Is(err, ErrInvalidUserCredentials) {\n\t\t\treturn LocalUserIdentity{}, err\n\t\t}\n\t\tconsumeDummyPasswordWork(password)\n\t\t_ = store.recordFirstPartyLoginAudit(ctx, now, "throttled", "", identityHash, sourceHash)\n\t\treturn LocalUserIdentity{}, ErrInvalidUserCredentials\n\t}''',
)

# IdP configuration supports an exact RP post-logout URI and an overlap key ring.
replace(
    "internal/bizruntime/config.go",
    '''type FirstPartyIdPConfig struct {\n\tPublicURL     string\n\tClientID      string\n\tRedirectURL   string\n\tSigningKeyPEM string\n\tSigningKeyID  string\n\tLoginTTL      time.Duration\n\tCodeTTL       time.Duration\n\tTokenTTL      time.Duration\n\tCookieSecure  bool\n}''',
    '''type FirstPartyIdPVerificationKey struct {\n\tPEM   string\n\tKeyID string\n}\n\ntype FirstPartyIdPConfig struct {\n\tPublicURL             string\n\tClientID              string\n\tRedirectURL           string\n\tPostLogoutRedirectURL string\n\tSigningKeyPEM         string\n\tSigningKeyID          string\n\tPreviousSigningKeys   []FirstPartyIdPVerificationKey\n\tLoginTTL              time.Duration\n\tCodeTTL               time.Duration\n\tTokenTTL              time.Duration\n\tCookieSecure          bool\n}''',
)
replace(
    "internal/bizruntime/config.go",
    '''\tif strings.TrimSpace(config.SigningKeyPEM) == "" {\n\t\treturn errors.New("biz runtime: first-party IdP RSA signing key is required")\n\t}\n\tif config.LoginTTL <= 0 || config.CodeTTL <= 0 || config.TokenTTL <= 0 {''',
    '''\tif strings.TrimSpace(config.SigningKeyPEM) == "" {\n\t\treturn errors.New("biz runtime: first-party IdP RSA signing key is required")\n\t}\n\tif config.PostLogoutRedirectURL != "" {\n\t\tif err := requireHTTPSOrLoopback(config.PostLogoutRedirectURL); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tparsed, _ := url.Parse(config.PostLogoutRedirectURL)\n\t\tif parsed.Fragment != "" {\n\t\t\treturn errors.New("biz runtime: first-party IdP post logout redirect must not contain a fragment")\n\t\t}\n\t}\n\tif len(config.PreviousSigningKeys) > 4 {\n\t\treturn errors.New("biz runtime: at most four previous IdP signing keys are supported")\n\t}\n\tfor _, previous := range config.PreviousSigningKeys {\n\t\tif strings.TrimSpace(previous.PEM) == "" {\n\t\t\treturn errors.New("biz runtime: previous IdP signing key PEM must not be empty")\n\t\t}\n\t}\n\tif config.LoginTTL <= 0 || config.CodeTTL <= 0 || config.TokenTTL <= 0 {''',
)

# Runtime key ring, audited/throttled login, and exact post-logout registration.
replace(
    "internal/bizruntime/first_party_idp.go",
    '''type runtimeFirstPartyIdP struct {\n\tconfig FirstPartyIdPConfig\n\tkey    *rsa.PrivateKey\n\tkid    string\n\tmu     sync.RWMutex\n\tstore  *accesspersistence.Store\n}''',
    '''type firstPartyVerificationKey struct {\n\tkid string\n\tkey *rsa.PublicKey\n}\n\ntype runtimeFirstPartyIdP struct {\n\tconfig           FirstPartyIdPConfig\n\tkey              *rsa.PrivateKey\n\tkid              string\n\tverificationKeys []firstPartyVerificationKey\n\tmu               sync.RWMutex\n\tstore            *accesspersistence.Store\n}''',
)
replace(
    "internal/bizruntime/first_party_idp.go",
    '''func newRuntimeFirstPartyIdP(config FirstPartyIdPConfig) (*runtimeFirstPartyIdP, error) {\n\tresult := &runtimeFirstPartyIdP{config: config}\n\tif !config.Enabled() {\n\t\treturn result, nil\n\t}\n\tkey, err := parseRSAPrivateKey([]byte(config.SigningKeyPEM))\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tif key.N.BitLen() < 2048 {\n\t\treturn nil, errors.New("biz runtime: first-party IdP RSA signing key must be at least 2048 bits")\n\t}\n\tkid := strings.TrimSpace(config.SigningKeyID)\n\tif kid == "" {\n\t\tencoded, err := x509.MarshalPKIXPublicKey(&key.PublicKey)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tdigest := sha256.Sum256(encoded)\n\t\tkid = base64.RawURLEncoding.EncodeToString(digest[:12])\n\t}\n\tresult.key = key\n\tresult.kid = kid\n\treturn result, nil\n}''',
    '''func newRuntimeFirstPartyIdP(config FirstPartyIdPConfig) (*runtimeFirstPartyIdP, error) {\n\tresult := &runtimeFirstPartyIdP{config: config}\n\tif !config.Enabled() {\n\t\treturn result, nil\n\t}\n\tkey, err := parseRSAPrivateKey([]byte(config.SigningKeyPEM))\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tif key.N.BitLen() < 2048 {\n\t\treturn nil, errors.New("biz runtime: first-party IdP RSA signing key must be at least 2048 bits")\n\t}\n\tkid, err := rsaKeyID(&key.PublicKey, config.SigningKeyID)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tverificationKeys := []firstPartyVerificationKey{{kid: kid, key: &key.PublicKey}}\n\tseen := map[string]struct{}{kid: struct{}{}}\n\tfor _, previous := range config.PreviousSigningKeys {\n\t\tpublic, err := parseRSAPublicKey([]byte(previous.PEM))\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tif public.N.BitLen() < 2048 {\n\t\t\treturn nil, errors.New("biz runtime: previous first-party IdP RSA key must be at least 2048 bits")\n\t\t}\n\t\tpreviousKid, err := rsaKeyID(public, previous.KeyID)\n\t\tif err != nil {\n\t\t\treturn nil, err\n\t\t}\n\t\tif _, duplicate := seen[previousKid]; duplicate {\n\t\t\treturn nil, errors.New("biz runtime: duplicate first-party IdP signing key id")\n\t\t}\n\t\tseen[previousKid] = struct{}{}\n\t\tverificationKeys = append(verificationKeys, firstPartyVerificationKey{kid: previousKid, key: public})\n\t}\n\tresult.key = key\n\tresult.kid = kid\n\tresult.verificationKeys = verificationKeys\n\treturn result, nil\n}''',
)
replace(
    "internal/bizruntime/first_party_idp.go",
    '''func (idp *runtimeFirstPartyIdP) handleJWKS(writer http.ResponseWriter, _ *http.Request) {\n\tpublic := &idp.key.PublicKey\n\twriteJSON(writer, http.StatusOK, map[string]any{\n\t\t"keys": []map[string]any{{\n\t\t\t"kty": "RSA",\n\t\t\t"use": "sig",\n\t\t\t"alg": "RS256",\n\t\t\t"kid": idp.kid,\n\t\t\t"n":   base64.RawURLEncoding.EncodeToString(public.N.Bytes()),\n\t\t\t"e":   base64.RawURLEncoding.EncodeToString(rsaExponentBytes(public.E)),\n\t\t}},\n\t})\n}''',
    '''func (idp *runtimeFirstPartyIdP) handleJWKS(writer http.ResponseWriter, _ *http.Request) {\n\tkeys := make([]map[string]any, 0, len(idp.verificationKeys))\n\tfor _, verification := range idp.verificationKeys {\n\t\tkeys = append(keys, map[string]any{\n\t\t\t"kty": "RSA",\n\t\t\t"use": "sig",\n\t\t\t"alg": "RS256",\n\t\t\t"kid": verification.kid,\n\t\t\t"n":   base64.RawURLEncoding.EncodeToString(verification.key.N.Bytes()),\n\t\t\t"e":   base64.RawURLEncoding.EncodeToString(rsaExponentBytes(verification.key.E)),\n\t\t})\n\t}\n\twriteJSON(writer, http.StatusOK, map[string]any{"keys": keys})\n}''',
)
replace(
    "internal/bizruntime/first_party_idp.go",
    '''\tidentity, err := store.AuthenticateUserPassword(request.Context(), email, password)\n\tif err != nil {\n\t\tidp.renderLogin(writer, http.StatusUnauthorized, requestID, csrf, email, "邮箱或密码错误")\n\t\treturn\n\t}''',
    '''\tidentity, err := store.AuthenticateFirstPartyLogin(request.Context(), email, password, request.RemoteAddr, accesspersistence.DefaultFirstPartyLoginPolicy())\n\tif err != nil {\n\t\tif errors.Is(err, accesspersistence.ErrInvalidUserCredentials) {\n\t\t\tidp.renderLogin(writer, http.StatusUnauthorized, requestID, csrf, email, "邮箱或密码错误")\n\t\t\treturn\n\t\t}\n\t\thttp.Error(writer, "identity provider unavailable", http.StatusServiceUnavailable)\n\t\treturn\n\t}''',
)
replace(
    "internal/bizruntime/first_party_idp.go",
    '''func (idp *runtimeFirstPartyIdP) handleProviderLogout(writer http.ResponseWriter, request *http.Request) {\n\ttarget := strings.TrimSpace(request.URL.Query().Get("post_logout_redirect_uri"))\n\tif target == "" {\n\t\ttarget = strings.TrimRight(idp.config.PublicURL, "/") + "/"\n\t}\n\tbase, baseErr := url.Parse(idp.config.PublicURL)\n\tredirect, redirectErr := url.Parse(target)\n\tif baseErr != nil || redirectErr != nil || redirect.Scheme != base.Scheme || redirect.Host != base.Host {\n\t\thttp.Error(writer, "invalid post logout redirect", http.StatusBadRequest)\n\t\treturn\n\t}\n\thttp.Redirect(writer, request, redirect.String(), http.StatusFound)\n}''',
    '''func (idp *runtimeFirstPartyIdP) handleProviderLogout(writer http.ResponseWriter, request *http.Request) {\n\texpected := strings.TrimSpace(idp.config.PostLogoutRedirectURL)\n\tif expected == "" {\n\t\texpected = strings.TrimRight(idp.config.PublicURL, "/") + "/"\n\t}\n\ttarget := strings.TrimSpace(request.URL.Query().Get("post_logout_redirect_uri"))\n\tif target == "" {\n\t\ttarget = expected\n\t}\n\tif target != expected {\n\t\thttp.Error(writer, "invalid post logout redirect", http.StatusBadRequest)\n\t\treturn\n\t}\n\tidp.clearAuthCookie(writer)\n\thttp.Redirect(writer, request, target, http.StatusFound)\n}''',
)
replace(
    "internal/bizruntime/first_party_idp.go",
    '''func rsaExponentBytes(exponent int) []byte {''',
    '''func parseRSAPublicKey(data []byte) (*rsa.PublicKey, error) {\n\tblock, _ := pem.Decode(data)\n\tif block == nil {\n\t\treturn nil, errors.New("biz runtime: invalid previous first-party IdP signing key PEM")\n\t}\n\tif value, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {\n\t\tif key, ok := value.(*rsa.PublicKey); ok {\n\t\t\treturn key, nil\n\t\t}\n\t}\n\tif key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {\n\t\treturn key, nil\n\t}\n\tif key, err := parseRSAPrivateKey(data); err == nil {\n\t\treturn &key.PublicKey, nil\n\t}\n\treturn nil, errors.New("biz runtime: previous first-party IdP signing key must be RSA")\n}\n\nfunc rsaKeyID(key *rsa.PublicKey, configured string) (string, error) {\n\tif configured = strings.TrimSpace(configured); configured != "" {\n\t\treturn configured, nil\n\t}\n\tencoded, err := x509.MarshalPKIXPublicKey(key)\n\tif err != nil {\n\t\treturn "", err\n\t}\n\tdigest := sha256.Sum256(encoded)\n\treturn base64.RawURLEncoding.EncodeToString(digest[:12]), nil\n}\n\nfunc rsaExponentBytes(exponent int) []byte {''',
)

# Standalone IdP runtime loads rollover keys and migrates security state.
replace(
    "cmd/biz-idp/main.go",
    '''\tconfig := bizruntime.FirstPartyIdPConfig{\n\t\tPublicURL:     publicURL,\n\t\tClientID:      envOr("YUNKA_BIZ_IDP_CLIENT_ID", "biz-web"),\n\t\tRedirectURL:   redirectURL,\n\t\tSigningKeyPEM: string(signingKey),\n\t\tSigningKeyID:  strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_SIGNING_KEY_ID")),\n\t\tLoginTTL:      envDuration("YUNKA_BIZ_IDP_LOGIN_TTL", 5*time.Minute),\n\t\tCodeTTL:       envDuration("YUNKA_BIZ_IDP_CODE_TTL", 90*time.Second),\n\t\tTokenTTL:      envDuration("YUNKA_BIZ_IDP_TOKEN_TTL", 5*time.Minute),\n\t\tCookieSecure:  envBool("YUNKA_BIZ_IDP_COOKIE_SECURE", true),\n\t}''',
    '''\tpreviousSigningKeys, err := readPreviousSigningKeys()\n\tif err != nil {\n\t\treturn err\n\t}\n\tconfig := bizruntime.FirstPartyIdPConfig{\n\t\tPublicURL:             publicURL,\n\t\tClientID:              envOr("YUNKA_BIZ_IDP_CLIENT_ID", "biz-web"),\n\t\tRedirectURL:           redirectURL,\n\t\tPostLogoutRedirectURL: strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_POST_LOGOUT_REDIRECT_URL")),\n\t\tSigningKeyPEM:         string(signingKey),\n\t\tSigningKeyID:          strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_SIGNING_KEY_ID")),\n\t\tPreviousSigningKeys:   previousSigningKeys,\n\t\tLoginTTL:              envDuration("YUNKA_BIZ_IDP_LOGIN_TTL", 5*time.Minute),\n\t\tCodeTTL:               envDuration("YUNKA_BIZ_IDP_CODE_TTL", 90*time.Second),\n\t\tTokenTTL:              envDuration("YUNKA_BIZ_IDP_TOKEN_TTL", 5*time.Minute),\n\t\tCookieSecure:          envBool("YUNKA_BIZ_IDP_COOKIE_SECURE", true),\n\t}''',
)
replace(
    "cmd/biz-idp/main.go",
    '''\tif envBool("YUNKA_BIZ_IDP_AUTO_MIGRATE", false) {\n\t\tif err := store.EnsureFirstPartyIDPSchema(context.Background()); err != nil {\n\t\t\treturn fmt.Errorf("migrate IdP schema: %w", err)\n\t\t}\n\t}''',
    '''\tif envBool("YUNKA_BIZ_IDP_AUTO_MIGRATE", false) {\n\t\tif err := store.EnsureFirstPartyIDPSchema(context.Background()); err != nil {\n\t\t\treturn fmt.Errorf("migrate IdP schema: %w", err)\n\t\t}\n\t\tif err := store.EnsureFirstPartyIDPSecuritySchema(context.Background()); err != nil {\n\t\t\treturn fmt.Errorf("migrate IdP security schema: %w", err)\n\t\t}\n\t}''',
)
replace(
    "cmd/biz-idp/main.go",
    '''func envOr(name, fallback string) string {''',
    '''func readPreviousSigningKeys() ([]bizruntime.FirstPartyIdPVerificationKey, error) {\n\trawFiles := strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_PREVIOUS_SIGNING_KEY_FILES"))\n\tif rawFiles == "" {\n\t\treturn nil, nil\n\t}\n\tfiles := strings.Split(rawFiles, ",")\n\tids := strings.Split(strings.TrimSpace(os.Getenv("YUNKA_BIZ_IDP_PREVIOUS_SIGNING_KEY_IDS")), ",")\n\tkeys := make([]bizruntime.FirstPartyIdPVerificationKey, 0, len(files))\n\tfor index, rawFile := range files {\n\t\tfile := strings.TrimSpace(rawFile)\n\t\tif file == "" {\n\t\t\treturn nil, errors.New("YUNKA_BIZ_IDP_PREVIOUS_SIGNING_KEY_FILES contains an empty path")\n\t\t}\n\t\tpemBytes, err := os.ReadFile(file)\n\t\tif err != nil {\n\t\t\treturn nil, fmt.Errorf("read previous IdP signing key %s: %w", file, err)\n\t\t}\n\t\tkeyID := ""\n\t\tif index < len(ids) {\n\t\t\tkeyID = strings.TrimSpace(ids[index])\n\t\t}\n\t\tkeys = append(keys, bizruntime.FirstPartyIdPVerificationKey{PEM: string(pemBytes), KeyID: keyID})\n\t}\n\treturn keys, nil\n}\n\nfunc envOr(name, fallback string) string {''',
)

# Security-preserving credential administration.
(ROOT / "cmd/biz-idp-credential/main.go").write_text(r'''package main

import (
    "bufio"
    "context"
    "errors"
    "flag"
    "fmt"
    "os"
    "strings"

    accesspersistence "github.com/hvritual/biz/internal/access/infrastructure/persistence"
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
    userID := flag.String("user-id", "", "existing Biz user id")
    action := flag.String("action", "rotate", "credential action: rotate or disable")
    flag.Parse()
    user := strings.TrimSpace(*userID)
    if user == "" {
        return errors.New("-user-id is required")
    }
    dsn := strings.TrimSpace(os.Getenv("YUNKA_BIZ_MYSQL_DSN"))
    if dsn == "" {
        return errors.New("YUNKA_BIZ_MYSQL_DSN is required")
    }
    database, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
    if err != nil {
        return fmt.Errorf("open credential database: %w", err)
    }
    store, err := accesspersistence.New(database)
    if err != nil {
        return err
    }
    switch strings.ToLower(strings.TrimSpace(*action)) {
    case "disable":
        if err := store.DisableUserPassword(context.Background(), user); err != nil {
            return err
        }
        fmt.Printf("credential disabled and web sessions revoked for user %s\n", user)
        return nil
    case "rotate":
        password, err := bufio.NewReader(os.Stdin).ReadString('\n')
        if err != nil && len(password) == 0 {
            return fmt.Errorf("read password from stdin: %w", err)
        }
        password = strings.TrimRight(password, "\r\n")
        if err := store.RotateUserPassword(context.Background(), user, password); err != nil {
            return err
        }
        fmt.Printf("credential rotated and web sessions revoked for user %s\n", user)
        return nil
    default:
        return errors.New("-action must be rotate or disable")
    }
}
''')

# Explicit deployable schema, independent of AutoMigrate.
(ROOT / "internal/access/infrastructure/persistence/migrations/0002_ce12_first_party_identity.sql").write_text(r'''CREATE TABLE IF NOT EXISTS biz_user_password_credentials (
  user_id VARCHAR(64) NOT NULL PRIMARY KEY,
  salt VARCHAR(128) NOT NULL,
  password_hash VARCHAR(128) NOT NULL,
  iterations INT NOT NULL,
  disabled BOOLEAN NOT NULL DEFAULT FALSE,
  password_changed_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_idp_authorization_requests (
  request_hash VARCHAR(64) NOT NULL PRIMARY KEY,
  browser_hash VARCHAR(64) NOT NULL,
  csrf_hash VARCHAR(64) NOT NULL,
  client_id VARCHAR(200) NOT NULL,
  redirect_uri VARCHAR(1024) NOT NULL,
  state VARCHAR(1024) NOT NULL,
  nonce VARCHAR(512) NOT NULL,
  code_challenge VARCHAR(128) NOT NULL,
  scope VARCHAR(1024) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_idp_request_browser (browser_hash),
  KEY idx_idp_request_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_idp_authorization_codes (
  code_hash VARCHAR(64) NOT NULL PRIMARY KEY,
  client_id VARCHAR(200) NOT NULL,
  redirect_uri VARCHAR(1024) NOT NULL,
  user_id VARCHAR(64) NOT NULL,
  nonce VARCHAR(512) NOT NULL,
  code_challenge VARCHAR(128) NOT NULL,
  scope VARCHAR(1024) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_idp_code_user (user_id),
  KEY idx_idp_code_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_idp_login_throttles (
  identity_hash VARCHAR(64) NOT NULL PRIMARY KEY,
  failure_count INT NOT NULL,
  window_started_at DATETIME(6) NOT NULL,
  blocked_until DATETIME(6) NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY idx_idp_throttle_blocked (blocked_until)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_idp_login_audit (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  occurred_at DATETIME(6) NOT NULL,
  outcome VARCHAR(32) NOT NULL,
  user_id VARCHAR(64) NULL,
  email_hash VARCHAR(64) NOT NULL,
  source_hash VARCHAR(64) NOT NULL,
  KEY idx_idp_audit_occurred (occurred_at),
  KEY idx_idp_audit_outcome (outcome),
  KEY idx_idp_audit_user (user_id),
  KEY idx_idp_audit_email (email_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_web_identities (
  issuer VARCHAR(512) NOT NULL,
  subject VARCHAR(255) NOT NULL,
  actor_kind VARCHAR(32) NOT NULL,
  actor_id VARCHAR(200) NOT NULL,
  email VARCHAR(320) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (issuer, subject),
  KEY idx_web_identity_actor_kind (actor_kind),
  KEY idx_web_identity_actor_id (actor_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_web_sessions (
  token_hash VARCHAR(64) NOT NULL PRIMARY KEY,
  issuer VARCHAR(512) NOT NULL,
  subject VARCHAR(255) NOT NULL,
  active_tenant_id VARCHAR(64) NULL,
  csrf_token VARCHAR(128) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  revoked_at DATETIME(6) NULL,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  KEY idx_web_session_identity (issuer, subject),
  KEY idx_web_session_tenant (active_tenant_id),
  KEY idx_web_session_expires (expires_at),
  KEY idx_web_session_revoked (revoked_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS biz_web_login_flows (
  state_hash VARCHAR(64) NOT NULL PRIMARY KEY,
  browser_hash VARCHAR(64) NOT NULL,
  code_verifier VARCHAR(160) NOT NULL,
  nonce_hash VARCHAR(64) NOT NULL,
  return_to VARCHAR(1024) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL,
  KEY idx_web_login_browser (browser_hash),
  KEY idx_web_login_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
''')

# Permanent unit coverage for key overlap and strict provider logout.
(ROOT / "internal/bizruntime/first_party_idp_hardening_test.go").write_text(r'''package bizruntime

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/json"
    "encoding/pem"
    "net/http"
    "net/http/httptest"
    "net/url"
    "testing"
    "time"
)

func TestCE12FirstPartyIdPKeyRolloverPublishesActiveAndPreviousKeys(t *testing.T) {
    active := testIDPPrivateKeyPEM(t)
    previous := testIDPPrivateKeyPEM(t)
    idp, err := newRuntimeFirstPartyIdP(testFirstPartyIdPConfig(active, []FirstPartyIdPVerificationKey{{PEM: previous}}))
    if err != nil {
        t.Fatal(err)
    }
    recorder := httptest.NewRecorder()
    idp.handleJWKS(recorder, httptest.NewRequest(http.MethodGet, "/idp/jwks", nil))
    if recorder.Code != http.StatusOK {
        t.Fatalf("jwks status=%d body=%s", recorder.Code, recorder.Body.String())
    }
    var body struct {
        Keys []map[string]any `json:"keys"`
    }
    if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
        t.Fatal(err)
    }
    if len(body.Keys) != 2 {
        t.Fatalf("jwks keys=%d want 2", len(body.Keys))
    }
    if body.Keys[0]["kid"] == body.Keys[1]["kid"] {
        t.Fatal("active and previous signing keys must have different kid values")
    }
}

func TestCE12FirstPartyIdPLogoutRequiresExactRegisteredRedirect(t *testing.T) {
    config := testFirstPartyIdPConfig(testIDPPrivateKeyPEM(t), nil)
    config.PostLogoutRedirectURL = "http://127.0.0.1:18080/signed-out"
    idp, err := newRuntimeFirstPartyIdP(config)
    if err != nil {
        t.Fatal(err)
    }
    invalid := httptest.NewRecorder()
    idp.handleProviderLogout(invalid, httptest.NewRequest(http.MethodGet, "/idp/logout?post_logout_redirect_uri="+url.QueryEscape("http://127.0.0.1:18081/anything"), nil))
    if invalid.Code != http.StatusBadRequest {
        t.Fatalf("invalid logout redirect status=%d want 400", invalid.Code)
    }
    valid := httptest.NewRecorder()
    idp.handleProviderLogout(valid, httptest.NewRequest(http.MethodGet, "/idp/logout?post_logout_redirect_uri="+url.QueryEscape(config.PostLogoutRedirectURL), nil))
    if valid.Code != http.StatusFound {
        t.Fatalf("valid logout redirect status=%d want 302", valid.Code)
    }
    if got := valid.Header().Get("Location"); got != config.PostLogoutRedirectURL {
        t.Fatalf("logout location=%q want %q", got, config.PostLogoutRedirectURL)
    }
}

func testFirstPartyIdPConfig(active string, previous []FirstPartyIdPVerificationKey) FirstPartyIdPConfig {
    return FirstPartyIdPConfig{
        PublicURL: "http://127.0.0.1:18081",
        ClientID: "biz-web",
        RedirectURL: "http://127.0.0.1:18080/auth/callback",
        SigningKeyPEM: active,
        PreviousSigningKeys: previous,
        LoginTTL: 5 * time.Minute,
        CodeTTL: time.Minute,
        TokenTTL: 5 * time.Minute,
    }
}

func testIDPPrivateKeyPEM(t *testing.T) string {
    t.Helper()
    key, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        t.Fatal(err)
    }
    return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
}
''')

# The permanent qualification workflows use the compatible Yunka DSL head and
# the actual CE-12 branch.
replace(
    ".github/workflows/ce12-browser-e2e.yml",
    "YUNKA_SHA: 33b98ceba57494abda2299e4f0290a5651dab4bc",
    "YUNKA_SHA: e323ee5833d929b5b1d0494fc29aae0aaa2b6f4a",
)
replace(
    ".github/workflows/ce12-browser-e2e.yml",
    '''            biz/internal/access/infrastructure/persistence/first_party_idp.go \\\n            biz/internal/access/infrastructure/persistence/web_session.go \\\n            biz/internal/bizruntime/config.go \\\n            biz/internal/bizruntime/first_party_idp.go \\\n            biz/internal/bizruntime/first_party_idp_server.go \\\n            biz/internal/bizruntime/oidc_static.go \\\n            biz/integration/ce12_browser_seed_test.go \\\n            biz/cmd/biz-idp/main.go \\\n            biz/cmd/biz-idp-credential/main.go)"''',
    '''            biz/internal/access/infrastructure/persistence/first_party_idp.go \\\n            biz/internal/access/infrastructure/persistence/first_party_idp_security.go \\\n            biz/internal/access/infrastructure/persistence/web_session.go \\\n            biz/internal/bizruntime/config.go \\\n            biz/internal/bizruntime/first_party_idp.go \\\n            biz/internal/bizruntime/first_party_idp_hardening_test.go \\\n            biz/internal/bizruntime/first_party_idp_server.go \\\n            biz/internal/bizruntime/oidc_static.go \\\n            biz/integration/ce12_browser_seed_test.go \\\n            biz/cmd/biz-idp/main.go \\\n            biz/cmd/biz-idp-credential/main.go)"''',
)
replace(
    ".github/workflows/ce12-browser-e2e.yml",
    '''          openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$RUNNER_TEMP/ce12-idp-signing.pem" >/dev/null 2>&1\n          chmod 600 "$RUNNER_TEMP/ce12-idp-signing.pem"''',
    '''          openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$RUNNER_TEMP/ce12-idp-signing.pem" >/dev/null 2>&1\n          openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$RUNNER_TEMP/ce12-idp-previous.pem" >/dev/null 2>&1\n          chmod 600 "$RUNNER_TEMP/ce12-idp-signing.pem" "$RUNNER_TEMP/ce12-idp-previous.pem"''',
)
replace(
    ".github/workflows/ce12-browser-e2e.yml",
    '''          export YUNKA_BIZ_IDP_SIGNING_KEY_FILE="$RUNNER_TEMP/ce12-idp-signing.pem"\n          export YUNKA_BIZ_IDP_COOKIE_SECURE="false"''',
    '''          export YUNKA_BIZ_IDP_SIGNING_KEY_FILE="$RUNNER_TEMP/ce12-idp-signing.pem"\n          export YUNKA_BIZ_IDP_PREVIOUS_SIGNING_KEY_FILES="$RUNNER_TEMP/ce12-idp-previous.pem"\n          export YUNKA_BIZ_IDP_POST_LOGOUT_REDIRECT_URL="http://127.0.0.1:18080/"\n          export YUNKA_BIZ_IDP_COOKIE_SECURE="false"''',
)
replace(
    ".github/workflows/ce12-qualification.yml",
    "branches: [feat/ce-12-oidc-bff-session, main]",
    "branches: [feat/ce-12-first-party-idp-e2e, main]",
)
replace(
    ".github/workflows/ce12-qualification.yml",
    "YUNKA_SHA: 33b98ceba57494abda2299e4f0290a5651dab4bc",
    "YUNKA_SHA: e323ee5833d929b5b1d0494fc29aae0aaa2b6f4a",
)
replace(
    ".github/workflows/ce12-qualification.yml",
    '''          bad="$(gofmt -l biz/internal/access/infrastructure/persistence/platform.go biz/internal/access/infrastructure/persistence/web_session.go biz/internal/bizruntime/config.go biz/internal/bizruntime/oidc_client.go biz/internal/bizruntime/oidc_client_test.go biz/internal/bizruntime/runtime.go biz/internal/bizruntime/web_auth.go biz/integration/ce12_web_session_mysql_test.go biz/cmd/biz/main.go)"''',
    '''          bad="$(gofmt -l biz/internal/access/infrastructure/persistence/platform.go biz/internal/access/infrastructure/persistence/first_party_idp.go biz/internal/access/infrastructure/persistence/first_party_idp_security.go biz/internal/access/infrastructure/persistence/web_session.go biz/internal/bizruntime/config.go biz/internal/bizruntime/first_party_idp.go biz/internal/bizruntime/first_party_idp_hardening_test.go biz/internal/bizruntime/oidc_client.go biz/internal/bizruntime/oidc_client_test.go biz/internal/bizruntime/runtime.go biz/internal/bizruntime/web_auth.go biz/integration/ce12_web_session_mysql_test.go biz/cmd/biz/main.go biz/cmd/biz-idp/main.go biz/cmd/biz-idp-credential/main.go)"''',
)
replace(
    ".github/workflows/ce12-qualification.yml",
    "go -C biz build ./cmd/biz",
    "go -C biz build ./cmd/biz ./cmd/biz-idp ./cmd/biz-idp-credential",
)

print("CE12_PRODUCTION_HARDENING_PATCHED=1")
