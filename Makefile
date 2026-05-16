BINARY    := agentic
BUILD_DIR := dist
VERSION   ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
LDFLAGS   := -s -w -X github.com/dylanvgils/agentic-cli/cmd.version=$(VERSION)
GOFLAGS   := CGO_ENABLED=0

.PHONY: build install uninstall dist test test-integration clean

build:
	$(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o bin/$(BINARY) .

install:
	$(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS) -X github.com/dylanvgils/agentic-cli/internal/platform.repoRoot=$(CURDIR)" -o bin/$(BINARY) .
	@mkdir -p ~/.local/bin
	cp bin/$(BINARY) ~/.local/bin/$(BINARY)
	@if ! echo "$$PATH" | grep -q "$$HOME/.local/bin"; then \
		echo "Note: add ~/.local/bin to your PATH (e.g. export PATH=\"\$$HOME/.local/bin:\$$PATH\")"; \
	fi

uninstall:
	rm -f ~/.local/bin/$(BINARY)

dist:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux   GOARCH=amd64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=arm64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe .

test:
	go test ./...

clean:
	rm -rf bin/ $(BUILD_DIR)
