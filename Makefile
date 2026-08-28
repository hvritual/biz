YUNKA_ROOT ?= $(abspath ../yunka.io)
YUNKA_APP := $(YUNKA_ROOT)/app
PROTO_DIR := $(CURDIR)/contracts/proto
THIRD_PARTY := $(CURDIR)/contracts/third_party
PROTO_FILE := deviceops/v1/deviceops.proto
PROTOC ?= protoc
PROTOC_GEN_GO ?= $(YUNKA_ROOT)/.yunka/bin/protoc-gen-go
PROTOC_GEN_GO_GRPC ?= $(YUNKA_ROOT)/.yunka/bin/protoc-gen-go-grpc
PROTOC_INCLUDE ?=

.PHONY: generate check test verify pressure run

generate:
	@cd $(YUNKA_APP) && go run ./cmd init --root $(CURDIR) --db-prefix biz
	@cd $(YUNKA_APP) && go run ./cmd domain generate --path $(CURDIR)/internal/deviceops
	@$(PROTOC) -I $(PROTO_DIR) -I $(THIRD_PARTY) -I $(YUNKA_ROOT)/contracts/proto $(if $(PROTOC_INCLUDE),-I $(PROTOC_INCLUDE),) \
		--plugin=protoc-gen-go=$(PROTOC_GEN_GO) --plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) \
		--go_out=$(CURDIR) --go_opt=module=github.com/hvritual/biz \
		--go-grpc_out=$(CURDIR) --go-grpc_opt=module=github.com/hvritual/biz,require_unimplemented_servers=false \
		$(PROTO_DIR)/$(PROTO_FILE)
	@cd $(YUNKA_APP) && go run ./cmd contract generate --proto-dir $(PROTO_DIR) --proto-path $(THIRD_PARTY) --proto-path $(YUNKA_ROOT)/contracts/proto --file $(PROTO_FILE) --out $(CURDIR)/contracts/generated --title "biz API" --version "1.0.0" --application-out $(CURDIR)/internal --application-import github.com/hvritual/biz/internal
	@go mod tidy

check:
	@cd $(YUNKA_APP) && go run ./cmd domain check --root $(CURDIR)/internal
	@cd $(YUNKA_APP) && go run ./cmd module check --root $(CURDIR)/modules
	@cd $(YUNKA_APP) && go run ./cmd contract check --proto-dir $(PROTO_DIR) --proto-path $(THIRD_PARTY) --proto-path $(YUNKA_ROOT)/contracts/proto --file $(PROTO_FILE) --out $(CURDIR)/contracts/generated --title "biz API" --version "1.0.0" --application-out $(CURDIR)/internal --application-import github.com/hvritual/biz/internal

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
