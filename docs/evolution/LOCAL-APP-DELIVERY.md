# Local account and application delivery

This procedure uses the one shared local MySQL instance from
[LOCAL-DATABASE.md](LOCAL-DATABASE.md). It neither creates a container nor
resets any data. Do not run it while the serial integration qualification owns
the database.

After CE-13 and CE-16 have been integrated, prepare the accounts once from the
repository root:

```sh
set -a
. ./.env.local
set +a
PATH=/tmp/biz-branch-audit/toolchain/bin:$PATH GOTOOLCHAIN=go1.25.13 \
  go run ./cmd/biz-local-setup
```

The command temporarily runs the ordinary Biz runtime on ephemeral loopback
ports so all commercial catalog, plan, default-subscription, tenant creation,
and activation calls use generated gRPC clients and the normal executor. It
creates a tenantless platform user and a tenant owner, binds their first-party
IdP identities, and assigns PBKDF2 password credentials. It writes generated
secrets only to `.local/biz-local-credentials.json` with mode `0600`; that
ignored file includes the two human login passwords and separate platform and
least-privilege worker tokens. Do not commit or paste its contents in logs.

The setup persists the generated credential intent before opening the database,
then atomically records the tenant ID on completion. It is therefore safe to
resume after an interruption. It rejects a pre-existing user ID or email that
does not exactly match the expected active local account, instead of
overwriting an unrelated identity.

Start the three processes only after setup succeeds:

```sh
scripts/local-apps.sh start
```

It uses `14183` for the Vue app, `18380` for Biz HTTP, `18381` for the IdP and
`18382` for gRPC. The launcher exports `.env.local`, builds owned binaries,
checks the exact parsed local DSN through `biz-local-setup --validate-config`
before starting anything, waits for each service in order, and refuses to kill
a stale PID unless its exact binary or Vite entry and reserved listening port
match. Stop mode needs neither credentials nor `.env.local`; it only checks
owned PID evidence. Logs and PID files remain under ignored `.local/`. Stop the
processes with `scripts/local-apps.sh stop` before database qualification.

After both CE-13 and CE-16 have reached their independent DONE receipts and
the local processes are ready, run the final browser acceptance using installed
system Chrome. It reads the ignored credential file only inside the Node
process, writes post-login screenshots under ignored `.local/login-acceptance/`,
and prints only role-safe PASS/FAIL output. Browser authentication and API calls
stay on the Vite Web origin: `/auth` uses its proxy and `/v1` is routed through
`/api`, matching the CE-13 harness and preserving the callback/session origin.

```sh
PLAYWRIGHT_SYSTEM_CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  node scripts/accept-local-login.mjs
```

The frontend must have CE-13's real API mode and proxy/session configuration
integrated before its `VITE_DATA_MODE=api` process is useful. CE-16 owns the
runtime worker configuration; this launcher deliberately supplies no paid-plan
or grace-period defaults.
