# tpm-crypto-service Makefile
# 一站式入口:proto / build / test / lint / bench / run

GO           ?= go
GOFLAGS      ?= -trimpath
BIN_DIR      ?= bin
BIN_NAME     ?= tpm-crypto-service
PKG          := ./...
VERSION      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD_TAGS   ?=
LDFLAGS      ?= -s -w -X main.Version=$(VERSION)

PROTO_FILES  := $(shell find api/proto -name '*.proto')
PROTOC       ?= protoc
PROTOC_GEN_GO := $(shell which protoc-gen-go 2>/dev/null)
PROTOC_GEN_GRPC := $(shell which protoc-gen-go-grpc 2>/dev/null)
PROTOC_GEN_GW := $(shell which protoc-gen-grpc-gateway 2>/dev/null)

.PHONY: all proto build build-openssl test test-cover bench bench-baseline lint run clean help

all: lint test build

help:
	@echo "Targets:"
	@echo "  proto           生成 gRPC + Gateway stub"
	@echo "  build           编译二进制 ($(BIN_NAME))"
	@echo "  build-openssl   使用 -tags=cgo_openssl 编译(链接系统 OpenSSL)"
	@echo "  test            跑单元测试"
	@echo "  test-cover      跑单元测试并生成覆盖率报告"
	@echo "  bench           跑算子基准"
	@echo "  bench-baseline  在 GOMAXPROCS=1 下重跑并刷新 bench/baseline.txt"
	@echo "  lint            go vet + staticcheck + gofmt"
	@echo "  run             启动服务(--config=config/config.example.yaml)"
	@echo "  clean           清理 build/bench 产物"

proto:
	@if [ -z "$(PROTOC_GEN_GO)" ] || [ -z "$(PROTOC_GEN_GRPC)" ] || [ -z "$(PROTOC_GEN_GW)" ]; then \
		echo "missing protoc plugins; install with:"; \
		echo "  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; \
		echo "  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; \
		echo "  go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest"; \
		exit 1; \
	fi
	@mkdir -p api/gen/go
	$(PROTOC) -I api/proto \
		--go_out=api/gen/go --go_opt=paths=source_relative \
		--go-grpc_out=api/gen/go --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=api/gen/go --grpc-gateway_opt=paths=source_relative \
		--grpc-gateway_opt=generate_unbound_methods=true \
		$(PROTO_FILES)
	@echo "proto generated under api/gen/go"

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -tags="$(BUILD_TAGS)" -ldflags='$(LDFLAGS)' -o $(BIN_DIR)/$(BIN_NAME) ./cmd/server

build-openssl:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -tags="cgo_openssl" -ldflags='$(LDFLAGS)' -o $(BIN_DIR)/$(BIN_NAME)-openssl ./cmd/server

test:
	$(GO) test -race -count=1 $(PKG)

test-cover:
	$(GO) test -race -count=1 -coverprofile=coverage.txt -covermode=atomic $(PKG)
	$(GO) tool cover -func=coverage.txt | tail -n 1

bench:
	@mkdir -p bench/results
	$(GO) test -run='^$' -bench=. -benchmem -benchtime=2s $(PKG) | tee bench/results/$(shell date +%Y%m%d)-$(VERSION).txt

bench-baseline:
	@mkdir -p bench
	GOMAXPROCS=1 $(GO) test -run='^$' -bench=. -benchmem -benchtime=3s $(PKG) > bench/baseline.txt
	@echo "baseline written to bench/baseline.txt"

lint:
	@command -v gofmt >/dev/null && $(GO) fmt $(PKG)
	$(GO) vet $(PKG)
	@command -v staticcheck >/dev/null && staticcheck $(PKG) || echo "staticcheck not installed; skip"

run: build
	./$(BIN_DIR)/$(BIN_NAME) --config=config/config.example.yaml

clean:
	rm -rf $(BIN_DIR) coverage.txt bench/results
