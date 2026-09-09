YUNKA_ROOT ?= $(abspath ../yunka.io)
YUNKA_APP := $(YUNKA_ROOT)/app
PROTOC ?= protoc
COMMERCIAL_BASELINE ?=

.PHONY: init generate check test verify pressure run workspace-check yunka-source-check consumer-certify

init:
	@cd $(YUNKA_APP) && go run ./cmd init --root $(CURDIR) --db-prefix biz

generate:
	@cd $(YUNKA_APP) && go run ./cmd generate --root $(CURDIR) --protoc $(PROTOC)
	@go mod tidy
	@$(MAKE) commercial-generate

check:
	@cd $(YUNKA_APP) && go run ./cmd check --root $(CURDIR) --protoc $(PROTOC)
	@$(MAKE) commercial-check

workspace-check:
	@./scripts/consumer-resolution-check.sh

yunka-source-check:
	@YUNKA_ROOT="$(YUNKA_ROOT)" ./scripts/verify-yunka-source.sh

test:
	@go test ./...

verify: workspace-check check test
	@go vet ./...
	@go build ./...

consumer-certify: yunka-source-check workspace-check
	@go test ./...
	@go vet ./...
	@go build ./...
	@GOWORK=off go test ./...
	@GOWORK=off go vet ./...
	@GOWORK=off go build ./...

pressure: verify
	@: "$${YUNKA_TEST_MYSQL_DSN:?YUNKA_TEST_MYSQL_DSN is required for biz pressure tests}"
	@go test -count=1 -tags=integration ./integration

run:
	@go run ./cmd/biz

# Linux-only exact compiler boundary probes; run after normal dependency setup.
.PHONY: tenant-boundary-check
tenant-boundary-check:
	@./scripts/check-tenant-boundary.sh

# CE-03 metadata only; runtime authorization remains an independent CE-05 task.
.PHONY: commercial-generate commercial-check
commercial-generate:
	@go run ./cmd/commercial-catalog --root $(CURDIR) --write --baseline "$(COMMERCIAL_BASELINE)"

commercial-check:
	@go run ./cmd/commercial-catalog --root $(CURDIR) --baseline "$(COMMERCIAL_BASELINE)"
