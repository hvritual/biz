YUNKA_ROOT ?= $(abspath ../yunka.io)
YUNKA_APP := $(YUNKA_ROOT)/app
PROTOC ?= protoc

.PHONY: init generate check test verify pressure run

init:
	@cd $(YUNKA_APP) && go run ./cmd init --root $(CURDIR) --db-prefix biz

generate:
	@cd $(YUNKA_APP) && go run ./cmd generate --root $(CURDIR) --protoc $(PROTOC)
	@go mod tidy

check:
	@cd $(YUNKA_APP) && go run ./cmd check --root $(CURDIR) --protoc $(PROTOC)

test:
	@go test ./...

verify: check test
	@go vet ./...
	@go build ./...

pressure: verify
	@: "$${YUNKA_TEST_MYSQL_DSN:?YUNKA_TEST_MYSQL_DSN is required for biz pressure tests}"
	@go test -count=1 -tags=integration ./integration

run:
	@go run ./cmd/biz
