// Package sidecar builds the proxy and DinD sidecar images and decides when they need a refresh.
package sidecar

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/dylanvgils/agentic-cli/internal/image/tools"
	"github.com/dylanvgils/agentic-cli/internal/infra/buildinfo"
	"github.com/dylanvgils/agentic-cli/internal/infra/cleanup"
	"github.com/dylanvgils/agentic-cli/internal/infra/logging"
	"github.com/dylanvgils/agentic-cli/internal/runtime/docker"
)

// proxyArches are the daemon architectures releases publish a proxy binary for.
var proxyArches = []string{"amd64", "arm64"}

// Service builds and refreshes the sidecar images.
type Service struct {
	docker   Docker
	releases Releases
}

// New returns a Service that talks to Docker through d and downloads proxy binaries through r.
func New(d Docker, r Releases) *Service {
	return &Service{docker: d, releases: r}
}

// Ensure refreshes the image of each enabled sidecar when needed.
func (s *Service) Ensure(proxy, dind bool, registry string) error {
	if proxy {
		if err := s.ensureProxy(registry); err != nil {
			return err
		}
	}

	if dind {
		return s.ensureDind(registry)
	}

	return nil
}

// BuildProxy builds the proxy image unconditionally for `agentic proxy build/update`, then prunes what the build left behind.
func (s *Service) BuildProxy(opts tools.BuildOptions) error {
	if err := s.buildProxy(buildinfo.Version, opts); err != nil {
		return err
	}

	// Best effort: a failed cleanup never fails the build
	_, _ = s.docker.PruneImages()
	return nil
}

// ensureProxy builds the proxy image if missing or stamped with a different CLI version, so `--proxy` picks up proxy changes shipped with a CLI update.
func (s *Service) ensureProxy(registry string) error {
	info, err := s.docker.InspectImage(tools.ProxyImage)
	if err != nil {
		return err
	}

	reason := docker.ImageRefreshReason(info, 0)
	if reason == "" {
		return nil
	}

	logging.Infof("building %s (%s)...", tools.ProxyImage, reason)
	return s.buildProxy(buildinfo.Version, tools.BuildOptions{Registry: registry})
}

// ensureDind builds the DinD image if missing, outdated or stale; a failed refresh only warns so offline runs work.
func (s *Service) ensureDind(registry string) error {
	info, err := s.docker.InspectImage(tools.DindImage)
	if err != nil {
		return err
	}

	reason := docker.ImageRefreshReason(info, tools.DindImageMaxAge)
	if reason == "" {
		return nil
	}

	logging.Infof("building %s (%s)...", tools.DindImage, reason)
	err = s.docker.BuildDindImage(tools.DindImage, tools.BuildOptions{Registry: registry})
	if err != nil && info != nil {
		logging.Warnf("could not refresh %s, using the existing image: %v", tools.DindImage, err)
		return nil
	}
	return err
}

// buildProxy builds the proxy image from source for a dev version, else from its released binary; ensureProxy uses it
// directly so `agentic run` never prunes.
func (s *Service) buildProxy(version string, opts tools.BuildOptions) (retErr error) {
	if buildinfo.IsDev(version) {
		sourceDir := buildinfo.DevSourceDir(tools.ProxyModulePath)
		if sourceDir == "" {
			return fmt.Errorf("proxy image: dev build requires the agentic source tree - run \"agentic build\" from the repository, or build a published version")
		}
		return s.docker.BuildProxyImage(tools.ProxyImage, version, sourceDir, opts)
	}

	arch, err := s.docker.ServerArch()
	if err != nil {
		return fmt.Errorf("proxy image: detecting the daemon's architecture: %w", err)
	}
	if !slices.Contains(proxyArches, arch) {
		return fmt.Errorf("proxy image: no released proxy binary for the daemon's architecture %q (supported: amd64, arm64)", arch)
	}

	contextDir, err := os.MkdirTemp("", "agentic-proxy-*")
	if err != nil {
		return fmt.Errorf("proxy image: %w", err)
	}
	defer cleanup.Capture(&retErr, func() error { return os.RemoveAll(contextDir) })

	if err := s.releases.DownloadProxy(version, arch, filepath.Join(contextDir, tools.ProxyBinaryName)); err != nil {
		return fmt.Errorf("proxy image: downloading the %s proxy binary from github.com: %w", version, err)
	}

	return s.docker.BuildProxyImage(tools.ProxyImage, version, contextDir, opts)
}
