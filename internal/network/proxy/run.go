package proxy

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/network/certs"
)

// Environment variables passed from the host to the proxy container - internal wiring between
// StartProxy (host) and the agentic-proxy binary (see cmd/proxy/main.go), not user-facing config.
const (
	EnvRules       = "AGENTIC_PROXY_RULES"       // JSON allow rule list (see LoadRules)
	EnvLog         = "AGENTIC_PROXY_LOG"         // JSON-lines access-log path
	EnvAddr        = "AGENTIC_PROXY_ADDR"        // listen address, required
	EnvTZOffset    = "AGENTIC_PROXY_TZ_OFFSET"   // host UTC offset in seconds, for log times
	EnvMonitor     = "AGENTIC_PROXY_MONITOR"     // "true" to log without enforcing the allowlist
	EnvCADir       = "AGENTIC_PROXY_CA_DIR"      // dir holding CACertFile and CAKeyFile
	EnvCredentials = "AGENTIC_PROXY_CREDENTIALS" // JSON credential list path (see LoadCredentials)
	EnvClientNets  = "AGENTIC_PROXY_CLIENT_NETS" // comma-separated CIDRs allowed to use the proxy; required with credentials
)

// Files in the EnvCADir directory.
const (
	CACertFile = "ca.pem"
	CAKeyFile  = "ca-key.pem"
)

// ConfigFromEnv builds a Config from the proxy environment variables.
func ConfigFromEnv() (Config, error) {
	rules, err := LoadRules(os.Getenv(EnvRules))
	if err != nil {
		return Config{}, err
	}

	addr := os.Getenv(EnvAddr)
	logPath := os.Getenv(EnvLog)
	offset, _ := strconv.Atoi(os.Getenv(EnvTZOffset))
	monitor, _ := strconv.ParseBool(os.Getenv(EnvMonitor))

	return Config{
		Addr:            addr,
		Rules:           rules,
		LogPath:         logPath,
		TZOffsetSeconds: offset,
		Monitor:         monitor,
		CADir:           os.Getenv(EnvCADir),
		CredentialsPath: os.Getenv(EnvCredentials),
		ClientNets:      config.SplitEnvValues(os.Getenv(EnvClientNets)),
	}, nil
}

// Config controls a proxy run.
type Config struct {
	Addr            string   // listen address, required
	Rules           []Rule   // allow rules; none blocks everything unless Monitor; hosts with paths need CADir
	LogPath         string   // JSON-lines access log file; entries are always logged too
	TZOffsetSeconds int      // host UTC offset for log times; the JSON log always stays UTC
	Monitor         bool     // log the allowlist verdict without enforcing it
	CADir           string   // per-run CA dir; set with CredentialsPath, which may list no credentials
	CredentialsPath string   // JSON credential list; empty disables injection
	ClientNets      []string // CIDRs allowed to use the proxy; empty allows any client
}

// Location returns the host's time zone from TZOffsetSeconds, for log times.
func (c Config) Location() *time.Location {
	return time.FixedZone("", c.TZOffsetSeconds)
}

// Run starts the forward proxy and blocks until it stops serving.
func Run(cfg Config) error {
	if cfg.Addr == "" {
		return fmt.Errorf("%s is not set", EnvAddr)
	}

	logFile, closeLog, err := openLog(cfg.LogPath)
	if err != nil {
		return err
	}
	defer closeLog()

	inject, err := loadInjector(cfg.CADir, cfg.CredentialsPath, len(PathHosts(cfg.Rules)) > 0)
	if err != nil {
		return err
	}

	server := NewServer(NewAllowlist(cfg.Rules), NewLogger(jsonWriter(logFile)), cfg.Monitor, inject)

	handler, err := clientHandler(server, cfg.ClientNets, inject != nil)
	if err != nil {
		return err
	}

	httpServer := &http.Server{
		Addr:     cfg.Addr,
		Handler:  handler,
		ErrorLog: httpErrorLog(),
	}
	return httpServer.ListenAndServe()
}

// openLog opens the JSON-lines log file; an empty path means no file (entries are still logged).
func openLog(path string) (file *os.File, closeFn func(), err error) {
	if path == "" {
		return nil, func() { /* no file to close */ }, nil
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

// loadInjector returns nil when both are empty and fails when only one is set, or neither while path rules need decrypting;
// once loaded, the CA key and credentials files are removed, so the secrets don't stay on the Docker host's disk for the
// whole run.
func loadInjector(caDir, credsPath string, pathRules bool) (*Injector, error) {
	if caDir == "" && credsPath == "" {
		if pathRules {
			return nil, fmt.Errorf("%s with paths needs %s and %s to decrypt their hosts", EnvRules, EnvCADir, EnvCredentials)
		}
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

	removeSecretFiles(filepath.Join(caDir, CAKeyFile), credsPath)
	return NewInjector(ca, creds), nil
}

// removeSecretFiles deletes paths, warning instead of failing since the volume is removed with the container anyway.
func removeSecretFiles(paths ...string) {
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			slog.Warn("could not remove secret file", "path", path, "err", err)
		}
	}
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
	if !ca.Scoped() {
		return certs.CA{}, fmt.Errorf("proxy CA must be name-constrained to the hosts it decrypts")
	}
	return ca, nil
}
