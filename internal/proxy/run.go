package proxy

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
)

// Port is the TCP port the proxy listens on inside its container.
const Port = "3128"

// DefaultAddr is the address the proxy listens on inside its container. It binds
// all interfaces so the tool container can reach it over the internal network.
const DefaultAddr = ":" + Port

// Environment variables passed from the host to the proxy container. They are
// internal wiring between StartProxy (host) and the agentic-proxy binary
// (container, see cmd/proxy/main.go), not user-facing configuration.
const (
	EnvAllow    = "AGENTIC_PROXY_ALLOW"     // comma-separated allowed hosts
	EnvLog      = "AGENTIC_PROXY_LOG"       // JSON-lines access-log path
	EnvAddr     = "AGENTIC_PROXY_ADDR"      // override listen address
	EnvTZOffset = "AGENTIC_PROXY_TZ_OFFSET" // host UTC offset in seconds, for the human-readable log line
)

// ConfigFromEnv builds a Config from the proxy environment variables.
func ConfigFromEnv() Config {
	addr := os.Getenv(EnvAddr)
	allowedHosts := config.SplitEnvValues(os.Getenv(EnvAllow))
	logPath := os.Getenv(EnvLog)
	offset, _ := strconv.Atoi(os.Getenv(EnvTZOffset))

	return Config{
		Addr:            addr,
		AllowedHosts:    allowedHosts,
		LogPath:         logPath,
		TZOffsetSeconds: offset,
	}
}

// Config controls a proxy run.
type Config struct {
	Addr            string   // listen address; empty uses DefaultAddr
	AllowedHosts    []string // permitted hosts (exact or leading-dot/"*." wildcard)
	LogPath         string   // JSON-lines access log file; always also written to stdout
	TZOffsetSeconds int      // host UTC offset shown in the stdout human-readable line; the JSON log always stays UTC
}

// Run starts the forward proxy and blocks until it stops serving.
func Run(cfg Config) error {
	addr := cfg.Addr
	if addr == "" {
		addr = DefaultAddr
	}

	logFile, closeLog, err := openLog(cfg.LogPath)
	if err != nil {
		return err
	}
	defer closeLog()

	location := time.FixedZone("", cfg.TZOffsetSeconds)
	server := NewServer(NewAllowlist(cfg.AllowedHosts), NewLogger(jsonWriter(logFile), os.Stdout, location))

	httpServer := &http.Server{
		Addr:    addr,
		Handler: server,
	}
	return httpServer.ListenAndServe()
}

// openLog opens the JSON-lines log file. An empty path means no file is
// configured; entries are still printed to stdout via Logger's human-readable
// output, so `docker logs -f` on the proxy container shows them live either way.
func openLog(path string) (file *os.File, closeFn func(), err error) {
	if path == "" {
		return nil, func() {}, nil
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("open proxy log %q: %w", path, err)
	}
	return f, func() { _ = f.Close() }, nil
}

// jsonWriter converts a possibly-nil *os.File to a possibly-nil io.Writer.
// Passing a nil *os.File directly as an io.Writer argument would produce a
// non-nil interface value wrapping a nil pointer, breaking nil checks.
func jsonWriter(f *os.File) io.Writer {
	if f == nil {
		return nil
	}
	return f
}
