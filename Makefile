BINARY    := agentic
BUILD_DIR := dist
# build/install are local dev targets - VERSION defaults to "dev" so the proxy
# image compiles from local source instead of installing a published module
# that may not have the code being developed yet. dist/docker-dist produce
# distributable artifacts, so they override VERSION to the real tag below.
VERSION    ?= dev
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "")
BUILD_DATE ?= $(shell date -u +%Y-%m-%d)
INSTALL_METHOD ?=
LDFLAGS     = -s -w \
              -X github.com/dylanvgils/agentic-cli/internal/infra/buildinfo.Version=$(VERSION) \
              -X github.com/dylanvgils/agentic-cli/internal/infra/buildinfo.Commit=$(COMMIT) \
              -X github.com/dylanvgils/agentic-cli/internal/infra/buildinfo.BuildDate=$(BUILD_DATE) \
              $(if $(INSTALL_METHOD),-X github.com/dylanvgils/agentic-cli/internal/infra/buildinfo.InstallMethod=$(INSTALL_METHOD))
GOFLAGS   := CGO_ENABLED=0
GOVULNCHECK_VERSION ?= v1.9.0
GOVULNCHECK := go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

.PHONY: build install uninstall dist docker-dist test test-coverage test-integration test-integration-msb lint audit audit-json clean verify-checksums fix-checksums verify-checksums-all fix-checksums-all

build:
	$(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/cli

install: INSTALL_METHOD = make
install:
	$(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) ./cmd/cli
	@mkdir -p ~/.local/bin
	cp bin/$(BINARY) ~/.local/bin/$(BINARY)
	@if ! echo "$$PATH" | grep -q "$$HOME/.local/bin"; then \
		echo "Note: add ~/.local/bin to your PATH (e.g. export PATH=\"\$$HOME/.local/bin:\$$PATH\")"; \
	fi

uninstall:
	rm -f ~/.local/bin/$(BINARY)

dist: VERSION = $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
dist:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux   GOARCH=amd64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/cli
	GOOS=linux   GOARCH=arm64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-arm64 ./cmd/cli
	GOOS=darwin  GOARCH=arm64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 ./cmd/cli
	GOOS=windows GOARCH=amd64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe ./cmd/cli

docker-dist:
	docker buildx build --file build/Dockerfile --target export --output $(BUILD_DIR)/ .

test:
	go test ./...

test-coverage:
	go test -cover -coverprofile=coverage.out ./...

# Needs a Docker daemon; builds an agentic-itest-claude image on first run.
test-integration:
	go run -tags integration ./test/integration/cmd/buildimages
	AGENTIC_ITEST_IMAGES_BUILT=1 go test -tags integration -count=1 -timeout 20m -v ./test/integration/...; \
		status=$$?; go run -tags integration ./test/integration/cmd/cleanup; exit $$status

# Needs msb and KVM; fails instead of skipping without msb.
test-integration-msb:
	go run -tags integration ./test/integration/cmd/buildimages
	AGENTIC_ITEST_IMAGES_BUILT=1 AGENTIC_ITEST_MICROSANDBOX=1 go test -tags integration -count=1 -timeout 20m -v ./test/integration/microsandbox; \
		status=$$?; go run -tags integration ./test/integration/cmd/cleanup; exit $$status

lint:
	golangci-lint run ./...

# Dependency integrity + known vulnerabilities; also run nightly by the audit workflow.
audit:
	go mod verify
	$(GOVULNCHECK) ./...

# Findings as JSON for the audit workflow's issue report; exits 0 even when vulnerable.
audit-json:
	@$(GOVULNCHECK) -format json ./...

# Pinned checksums only (currently just nvm) - safe to run on every PR, since these
# only change when this repo bumps the pinned version itself.
verify-checksums:
	.github/scripts/verify-install-checksums.sh --pinned-only

fix-checksums:
	.github/scripts/verify-install-checksums.sh --fix --pinned-only

# All checksums, including installer scripts (claude/copilot/opencode) that track a
# stable "latest" URL and can drift upstream at any time - used by the daily
# scheduled refresh workflow, not per-PR CI.
verify-checksums-all:
	.github/scripts/verify-install-checksums.sh

fix-checksums-all:
	.github/scripts/verify-install-checksums.sh --fix

clean:
	rm -rf bin/ $(BUILD_DIR)
