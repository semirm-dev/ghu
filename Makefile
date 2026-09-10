BINARY  := ghu
MODULE  := github.com/semirm-dev/ghu
BUILD   := .build
BIN     := bin
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64
VERSION := $(shell cat VERSION)
LDFLAGS := -ldflags "-X $(MODULE)/internal/cli.Version=$(VERSION)"
COVERFILE := coverprofile

.PHONY: help build install test test-cover lint order release tidy clean

help: ## Show available targets
	@echo "Usage: make <target>"
	@echo ""
	@echo "  build       Build the ghu binary into $(BUILD)/$(BINARY)."
	@echo "  install     Install ghu into GOBIN."
	@echo "  test        Run the test suite with race detection."
	@echo "  test-cover  Run tests and open the coverage report."
	@echo "  lint        gofmt, goimports, declaration order and go vet."
	@echo "  order       Reorder declarations to the house order."
	@echo "  release     Cross-compile binaries for all platforms into $(BIN)/."
	@echo "  tidy        go mod tidy."
	@echo "  clean       Remove build output."

build: ## Build the ghu binary
	@mkdir -p $(BUILD)
	go build $(LDFLAGS) -o $(BUILD)/$(BINARY) ./cmd/ghu

install: ## Install ghu into GOBIN
	go install $(LDFLAGS) ./cmd/ghu

test: ## Run tests with race detection
	@test -n "$$(find . -name '*_test.go' -not -path './.git/*' -print -quit)" \
		|| (echo "no tests in the tree -- they were removed while the package layout is in flux" && exit 1)
	go test ./... -race -count=1

test-cover: ## Run tests and open the coverage report
	go test ./... -coverprofile=$(COVERFILE)
	go tool cover -html=$(COVERFILE) && go tool cover -func $(COVERFILE) && unlink $(COVERFILE)

lint: ## Check formatting, import grouping, declaration order and run vet
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || (echo "gofmt found issues" && exit 1)
	@test -z "$$(go tool goimports -local $(MODULE) -l . | tee /dev/stderr)" || (echo "goimports found issues" && exit 1)
	@python3 scripts/order.py --check $$(find internal cmd -name '*.go') \
		|| (echo "run 'make order' to fix" && exit 1)
	go vet ./...

release: ## Cross-compile release binaries into bin/
	@rm -rf $(BIN) && mkdir -p $(BIN)
	@set -e; for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out=$(BIN)/$(BINARY)-$$os-$$arch$$ext; \
		echo "  $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -buildvcs=false -ldflags "-s -w -X $(MODULE)/internal/cli.Version=$(VERSION)" \
			-o $$out ./cmd/ghu; \
	done
	@cd $(BIN) && sha256sum * > SHA256SUMS && echo "  $(BIN)/SHA256SUMS"

order: ## Reorder declarations to the house order
	@python3 scripts/order.py $$(find internal cmd -name '*.go')
	@gofmt -w .

tidy: ## Tidy the module graph
	go mod tidy

clean: ## Remove build output
	rm -rf $(BUILD) $(BIN)
