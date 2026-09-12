#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
yunka_root="${YUNKA_ROOT:-$root/../yunka.io}"
source "$yunka_root/tools/toolchain.env"
fail() { echo "toolchain mismatch: $*" >&2; exit 1; }
actual_go="$(go version)"
[[ "$actual_go" == "go version go${GO_VERSION} "* ]] || fail "expected Go ${GO_VERSION}; got ${actual_go} (use GOTOOLCHAIN=go${GO_VERSION})"
actual_protoc="$("${PROTOC:-protoc}" --version)"
[[ "$actual_protoc" == "libprotoc ${PROTOC_VERSION}" ]] || fail "expected protoc ${PROTOC_VERSION}; got ${actual_protoc}"
actual_go_plugin="$(protoc-gen-go --version)"
[[ "$actual_go_plugin" == "protoc-gen-go ${PROTOC_GEN_GO_VERSION}" ]] || fail "expected protoc-gen-go ${PROTOC_GEN_GO_VERSION}; got ${actual_go_plugin}"
actual_grpc_plugin="$(protoc-gen-go-grpc --version)"
[[ "$actual_grpc_plugin" == "protoc-gen-go-grpc ${PROTOC_GEN_GO_GRPC_VERSION#v}" ]] || fail "expected protoc-gen-go-grpc ${PROTOC_GEN_GO_GRPC_VERSION}; got ${actual_grpc_plugin}"
echo "toolchain check: Go ${GO_VERSION}, protoc ${PROTOC_VERSION}, Go ${PROTOC_GEN_GO_VERSION}, gRPC ${PROTOC_GEN_GO_GRPC_VERSION}"
