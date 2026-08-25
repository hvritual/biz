GO ?= go

.PHONY: fmt test race vet build verify

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

race:
	CGO_ENABLED=1 $(GO) test -race ./...

vet:
	$(GO) vet ./...

build:
	$(GO) build ./...

verify: test race vet build
