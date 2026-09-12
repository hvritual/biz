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
waits for each service in order, and refuses to kill a stale PID whose command
does not match its expected local binary. Logs and PID files remain under
ignored `.local/`. Stop the processes with `scripts/local-apps.sh stop` before
database qualification.

The frontend must have CE-13's real API mode and proxy/session configuration
integrated before its `VITE_DATA_MODE=api` process is useful. CE-16 owns the
runtime worker configuration; this launcher deliberately supplies no paid-plan
or grace-period defaults.
