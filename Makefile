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
              -X github.com/dylanvgils/agentic-cli/internal/buildinfo.Version=$(VERSION) \
              -X github.com/dylanvgils/agentic-cli/internal/buildinfo.Commit=$(COMMIT) \
              -X github.com/dylanvgils/agentic-cli/internal/buildinfo.BuildDate=$(BUILD_DATE) \
              $(if $(INSTALL_METHOD),-X github.com/dylanvgils/agentic-cli/internal/buildinfo.InstallMethod=$(INSTALL_METHOD))
GOFLAGS   := CGO_ENABLED=0

.PHONY: build install uninstall dist docker-dist test coverage clean

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
	docker buildx build --target export --output $(BUILD_DIR)/ .

test:
	go test ./...

coverage:
	go test -cover ./...

clean:
	rm -rf bin/ $(BUILD_DIR)
