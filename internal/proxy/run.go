package proxy

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/certs"
	"github.com/dylanvgils/agentic-cli/internal/config"
)

// Port is the TCP port the proxy listens on inside its container.
const Port = "3128"

// DefaultAddr is the address the proxy listens on inside its container, binding all interfaces.
const DefaultAddr = ":" + Port

// Environment variables passed from the host to the proxy container - internal wiring between
// StartProxy (host) and the agentic-proxy binary (see cmd/proxy/main.go), not user-facing config.
const (
	EnvAllow       = "AGENTIC_PROXY_ALLOW"       // comma-separated allowed hosts
	EnvLog         = "AGENTIC_PROXY_LOG"         // JSON-lines access-log path
	EnvAddr        = "AGENTIC_PROXY_ADDR"        // override listen address
	EnvTZOffset    = "AGENTIC_PROXY_TZ_OFFSET"   // host UTC offset in seconds, for the human-readable log line
	EnvMonitor     = "AGENTIC_PROXY_MONITOR"     // "true" to log without enforcing the allowlist
	EnvCADir       = "AGENTIC_PROXY_CA_DIR"      // dir holding CACertFile and CAKeyFile
	EnvCredentials = "AGENTIC_PROXY_CREDENTIALS" // JSON credential list path (see LoadCredentials)
)

// Files in the EnvCADir directory.
const (
	CACertFile = "ca.pem"
	CAKeyFile  = "ca-key.pem"
)

// ConfigFromEnv builds a Config from the proxy environment variables.
func ConfigFromEnv() Config {
	addr := os.Getenv(EnvAddr)
	allowedHosts := config.SplitEnvValues(os.Getenv(EnvAllow))
	logPath := os.Getenv(EnvLog)
	offset, _ := strconv.Atoi(os.Getenv(EnvTZOffset))
	monitor, _ := strconv.ParseBool(os.Getenv(EnvMonitor))

	return Config{
		Addr:            addr,
		AllowedHosts:    allowedHosts,
		LogPath:         logPath,
		TZOffsetSeconds: offset,
		Monitor:         monitor,
		CADir:           os.Getenv(EnvCADir),
		CredentialsPath: os.Getenv(EnvCredentials),
	}
}

// Config controls a proxy run.
type Config struct {
	Addr            string   // listen address; empty uses DefaultAddr
	AllowedHosts    []string // permitted hosts (exact or leading-dot/"*." wildcard)
	LogPath         string   // JSON-lines access log file; always also written to stdout
	TZOffsetSeconds int      // host UTC offset shown in the stdout human-readable line; the JSON log always stays UTC
	Monitor         bool     // log the allowlist verdict without enforcing it
	CADir           string   // per-run CA dir; set with CredentialsPath
	CredentialsPath string   // JSON credential list; empty disables injection
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

	inject, err := loadInjector(cfg.CADir, cfg.CredentialsPath)
	if err != nil {
		return err
	}

	location := time.FixedZone("", cfg.TZOffsetSeconds)
	server := NewServer(NewAllowlist(cfg.AllowedHosts), NewLogger(jsonWriter(logFile), os.Stdout, location), cfg.Monitor, inject)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: server,
	}
	return httpServer.ListenAndServe()
}

// openLog opens the JSON-lines log file; an empty path means no file (entries still print to stdout).
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

// jsonWriter converts a possibly-nil *os.File to a possibly-nil io.Writer, avoiding a non-nil interface wrapping a nil pointer.
func jsonWriter(f *os.File) io.Writer {
	if f == nil {
		return nil
	}
	return f
}

// loadInjector returns nil when both are empty and fails when only one is set.
func loadInjector(caDir, credsPath string) (*Injector, error) {
	if caDir == "" && credsPath == "" {
		return nil, nil
	}
	if caDir == "" || credsPath == "" {
		return nil, fmt.Errorf("%s and %s must be set together", EnvCADir, EnvCredentials)
	}

	ca, err := loadCA(caDir)
	if err != nil {
		return nil, err
	}

	creds, err := LoadCredentials(credsPath)
	if err != nil {
		return nil, err
	}
	return NewInjector(ca, creds), nil
}

// loadCA reads the CA from CACertFile and CAKeyFile in dir.
func loadCA(dir string) (certs.CA, error) {
	certPEM, err := os.ReadFile(filepath.Join(dir, CACertFile))
	if err != nil {
		return certs.CA{}, fmt.Errorf("read proxy CA: %w", err)
	}

	keyPEM, err := os.ReadFile(filepath.Join(dir, CAKeyFile))
	if err != nil {
		return certs.CA{}, fmt.Errorf("read proxy CA key: %w", err)
	}

	ca, err := certs.LoadCA(certPEM, keyPEM)
	if err != nil {
		return certs.CA{}, fmt.Errorf("load proxy CA: %w", err)
	}
	return ca, nil
}
