YUNKA_ROOT ?= $(abspath ../yunka.io)
YUNKA_APP := $(YUNKA_ROOT)/app
PROTO_DIR := $(CURDIR)/contracts/proto
THIRD_PARTY := $(CURDIR)/contracts/third_party
PROTO_FILE := deviceops/v1/deviceops.proto
PROTOC ?= protoc
PROTOC_GEN_GO ?= $(YUNKA_ROOT)/.yunka/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC ?= $(YUNKA_ROOT)/.yunka/bin/protoc-gen-go-grpc
PROTOC_INCLUDE ?=

.PHONY: init generate-proto generate check test verify pressure run

init:
	@cd $(YUNKA_APP) && go run ./cmd init --root $(CURDIR) --db-prefix biz

generate-proto:
	@$(PROTOC) -I $(PROTO_DIR) -I $(THIRD_PARTY) $(if $(PROTOC_INCLUDE),-I $(PROTOC_INCLUDE),) \
		--plugin=protoc-gen-go=$(PROTOC_GEN_GO) --plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) \
		--go_out=$(CURDIR) --go_opt=module=github.com/hvritual/biz \
		--go-grpc_out=$(CURDIR) --go-grpc_opt=module=github.com/hvritual/biz,require_unimplemented_servers=false \
		$(PROTO_DIR)/$(PROTO_FILE)

generate: generate-proto
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
