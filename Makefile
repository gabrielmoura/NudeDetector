# =============================================================================
# NudeDetector — Build System
# =============================================================================

BINARY_NAME := nude-detector
CMD_PATH := ./cmd/api
BUILD_DIR := bin
GOCACHE ?= /tmp/nude-detector-go-build
GOTMPDIR ?= /tmp/nude-detector-go-tmp
GOFLAGS ?= -p=1

MODEL_DIR := models
MODEL_PATH := $(MODEL_DIR)/model.onnx
MODEL_URL := https://huggingface.co/onnx-community/nsfw-classifier-ONNX/resolve/main/onnx/model.onnx
MODEL_SHA256 := d6e841f61f193d9f435b464748c772b4096edd941c9f12b3a05bcb655fb267eb

ONNXRUNTIME_VERSION := 1.22.0
ONNXRUNTIME_DIR := .cache/onnxruntime/$(ONNXRUNTIME_VERSION)
ONNXRUNTIME_LIBRARY := $(ONNXRUNTIME_DIR)/lib/libonnxruntime.so.$(ONNXRUNTIME_VERSION)

IMAGE ?= nude-detector
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
BUILD_FLAGS := -trimpath -buildvcs=false

.DEFAULT_GOAL := help

.PHONY: help
help: ## Mostra os alvos disponíveis
	@echo "NudeDetector — Build Instructions"
	@echo ""
	@echo "Uso: make <target>"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: model
model: ## Baixa e valida o modelo ONNX versionado
	@mkdir -p $(MODEL_DIR)
	@if [ ! -f $(MODEL_PATH) ]; then \
		curl --fail --location --silent --show-error $(MODEL_URL) --output $(MODEL_PATH).tmp; \
		mv $(MODEL_PATH).tmp $(MODEL_PATH); \
	fi
	@echo "$(MODEL_SHA256)  $(MODEL_PATH)" | sha256sum --check

.PHONY: onnxruntime
onnxruntime: ## Baixa a Runtime ONNX CPU compatível para desenvolvimento local
	@mkdir -p $(ONNXRUNTIME_DIR)
	@if [ ! -f $(ONNXRUNTIME_LIBRARY) ]; then \
		curl --fail --location --silent --show-error \
			"https://github.com/microsoft/onnxruntime/releases/download/v$(ONNXRUNTIME_VERSION)/onnxruntime-linux-x64-$(ONNXRUNTIME_VERSION).tgz" \
			--output /tmp/nude-detector-onnxruntime.tgz; \
		tar -xzf /tmp/nude-detector-onnxruntime.tgz -C $(ONNXRUNTIME_DIR) --strip-components=1; \
		rm -f /tmp/nude-detector-onnxruntime.tgz; \
	fi
	@test -f $(ONNXRUNTIME_LIBRARY)

.PHONY: build
build: ## Compila o binário Linux local
	@mkdir -p $(BUILD_DIR) $(GOCACHE) $(GOTMPDIR)
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) GOFLAGS="$(GOFLAGS)" CGO_ENABLED=1 go build $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)

.PHONY: test
test: ## Executa os testes Go
	@mkdir -p $(GOCACHE) $(GOTMPDIR)
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) GOFLAGS="$(GOFLAGS)" go test ./...

.PHONY: test-race
test-race: ## Executa os testes Go com detector de corrida
	@mkdir -p $(GOCACHE) $(GOTMPDIR)
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) GOFLAGS="$(GOFLAGS)" go test -race ./...

.PHONY: vet
vet: ## Executa análise estática do Go
	@mkdir -p $(GOCACHE) $(GOTMPDIR)
	GOCACHE=$(GOCACHE) GOTMPDIR=$(GOTMPDIR) GOFLAGS="$(GOFLAGS)" go vet ./...

.PHONY: fmt
fmt: ## Formata o código Go
	gofmt -w $$(find cmd internal -name '*.go' -type f)

.PHONY: fmt-check
fmt-check: ## Falha se arquivos Go não estiverem formatados
	@test -z "$$(gofmt -l $$(find cmd internal -name '*.go' -type f))"

.PHONY: check
check: fmt-check vet test ## Executa verificações sem corrida

.PHONY: run
run: model onnxruntime build ## Inicia a API local com modelo e Runtime compatíveis
	ONNXRUNTIME_SHARED_LIBRARY_PATH=$(ONNXRUNTIME_LIBRARY) ./$(BUILD_DIR)/$(BINARY_NAME)

.PHONY: docker
docker: ## Cria a imagem Docker de produção
	docker build \
		--build-arg ONNXRUNTIME_VERSION=$(ONNXRUNTIME_VERSION) \
		--build-arg MODEL_URL=$(MODEL_URL) \
		--build-arg MODEL_SHA256=$(MODEL_SHA256) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		.

.PHONY: clean
clean: ## Remove os artefatos locais de build e Runtime
	rm -rf $(BUILD_DIR) .cache

.PHONY: all
all: check build ## Executa verificações e compila o binário
