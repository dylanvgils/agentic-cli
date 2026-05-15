BINARY    := agentic-cli
BUILD_DIR := dist
LDFLAGS   := -s -w
GOFLAGS   := CGO_ENABLED=0

.PHONY: build install dist test test-integration clean

build:
	$(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) .

install: build
	@mkdir -p ~/.local/bin
	cp $(BINARY) ~/.local/bin/$(BINARY)
	@if ! echo "$$PATH" | grep -q "$$HOME/.local/bin"; then \
		echo "Note: add ~/.local/bin to your PATH (e.g. export PATH=\"\$$HOME/.local/bin:\$$PATH\")"; \
	fi

dist:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux   GOARCH=amd64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=arm64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GOFLAGS) go build -trimpath -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY)-windows-amd64.exe .

test:
	go test ./...

test-integration:
	go test -tags=integration ./...

clean:
	rm -f $(BINARY)
	rm -rf $(BUILD_DIR)
