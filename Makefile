BINARY  := ghu
MODULE  := github.com/semirm-dev/ghu
BUILD   := .build
BIN     := bin
# GOOS/GOARCH. The published names say macos rather than darwin -- see the
# release target.
PLATFORMS := darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64
COVERFILE := coverprofile
# Set by the release workflow from the tag. A local `make release` leaves it
# empty, and those binaries report "dev", because they are not a release.
VERSION ?=
STAMP := $(if $(VERSION),-X $(MODULE)/internal/cli.version=$(VERSION),)

.PHONY: help build install test test-cover lint order release tag tidy clean

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
	@echo "  tag         Tag a release: make tag VERSION=0.3.0."
	@echo "  tidy        go mod tidy."
	@echo "  clean       Remove build output."

build: ## Build the ghu binary
	@mkdir -p $(BUILD)
	go build -o $(BUILD)/$(BINARY) ./cmd/ghu

install: ## Install ghu into GOBIN
	go install ./cmd/ghu

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
		case $$os in darwin) label=macos ;; *) label=$$os ;; esac; \
		out=$(BIN)/$(BINARY)-$$label-$$arch$$ext; \
		echo "  $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -buildvcs=false -ldflags "-s -w $(STAMP)" \
			-o $$out ./cmd/ghu; \
	done
	@cd $(BIN) && sha256sum * > SHA256SUMS && echo "  $(BIN)/SHA256SUMS"

# ghu's version comes from the tag, so there is no bump to commit and this only
# tags. It exists so the flow matches sigi's, and so the -X stamp is proven on
# this machine rather than discovered to be broken by a release run.
tag: ## Tag a release (make tag VERSION=0.3.0)
	@[ -n "$(VERSION)" ] || { echo "usage: make tag VERSION=0.3.0"; exit 1; }
	@git diff --quiet && git diff --cached --quiet \
		|| { echo "working tree is dirty; commit or stash first"; exit 1; }
	@mkdir -p $(BUILD)
	@go build -ldflags "$(STAMP)" -o $(BUILD)/$(BINARY)-tagcheck ./cmd/ghu
	@test "$$($(BUILD)/$(BINARY)-tagcheck version)" = "$(VERSION)" \
		|| { echo "stamped binary reports $$($(BUILD)/$(BINARY)-tagcheck version), not $(VERSION)"; exit 1; }
	@rm -f $(BUILD)/$(BINARY)-tagcheck
	@git tag -a v$(VERSION) -m "$(BINARY) $(VERSION)"
	@echo "  tagged v$(VERSION) on $$(git rev-parse --short HEAD)"
	@echo "  push with: git push && git push origin v$(VERSION)"

order: ## Reorder declarations to the house order
	@python3 scripts/order.py $$(find internal cmd -name '*.go')
	@gofmt -w .

tidy: ## Tidy the module graph
	go mod tidy

clean: ## Remove build output
	rm -rf $(BUILD) $(BIN)
