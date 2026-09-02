YUNKA_ROOT ?= $(abspath ../yunka.io)
YUNKA_APP := $(YUNKA_ROOT)/app
PROTOC ?= protoc

.PHONY: init generate check test verify pressure run workspace-check yunka-source-check release-resolution-check release-certify consumer-certify

init:
	@cd $(YUNKA_APP) && go run ./cmd init --root $(CURDIR) --db-prefix biz

generate:
	@cd $(YUNKA_APP) && go run ./cmd generate --root $(CURDIR) --protoc $(PROTOC)
	@go mod tidy

check:
	@cd $(YUNKA_APP) && go run ./cmd check --root $(CURDIR) --protoc $(PROTOC)

workspace-check:
	@./scripts/consumer-resolution-check.sh

yunka-source-check:
	@YUNKA_ROOT="$(YUNKA_ROOT)" ./scripts/verify-yunka-source.sh

release-resolution-check:
	@./scripts/release-resolution-check.sh

test:
	@go test ./...

verify: workspace-check check test
	@go vet ./...
	@go build ./...

release-certify: release-resolution-check
	@GOWORK=off go test ./...
	@GOWORK=off go vet ./...
	@GOWORK=off go build ./...

consumer-certify: yunka-source-check workspace-check
	@go test ./...
	@go vet ./...
	@go build ./...
	@$(MAKE) release-certify

pressure: verify
	@: "$${YUNKA_TEST_MYSQL_DSN:?YUNKA_TEST_MYSQL_DSN is required for biz pressure tests}"
	@go test -count=1 -tags=integration ./integration

run:
	@go run ./cmd/biz
