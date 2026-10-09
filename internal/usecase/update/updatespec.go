// Package update resolves and applies `agentic update` targets: which tool images to rebuild, with which recovered build options, and whether to throttle an automatic --pull.
package update

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/dylanvgils/agentic-cli/internal/config"
	"github.com/dylanvgils/agentic-cli/internal/docker"
	"github.com/dylanvgils/agentic-cli/internal/logging"
	"github.com/dylanvgils/agentic-cli/internal/tools"
	"github.com/dylanvgils/agentic-cli/internal/usecase/toolupdate"
)

// autoPullInterval bounds how often an automatic --pull may hit the registry again for the same image.
const autoPullInterval = 24 * time.Hour

// Service resolves and rebuilds tool images for `agentic update`.
type Service struct {
	docker Docker
	home   string // agentic data dir, for the custom installs store
}

// Target is one tool image to rebuild, with its resolved build options.
type Target struct {
	Name  string
	Image string
	Opts  tools.BuildOptions
}

// Scope describes which tools/images Resolve should target.
type Scope struct {
	// Names is the tool name list to resolve in scoped mode, already expanded to every known tool when none was given.
	Names   []string
	HasArgs bool
	// FilterTool narrows an --all resolve to one tool; empty means every tool.
	FilterTool string
	Namespace  string
	All        bool
}

// New returns a Service that talks to Docker through d and keeps custom installs under home.
func New(d Docker, home string) *Service {
	return &Service{docker: d, home: home}
}

// Resolve returns the update targets for scope: every image across all namespaces (All), or the named tools in one namespace,
// plus the images skipped because they were never built (scoped mode without tool args only).
func (s *Service) Resolve(scope Scope, opts tools.BuildOptions, pullExplicit bool) (targets []Target, skipped []string, err error) {
	if scope.All {
		targets, err = s.resolveAll(scope.FilterTool, opts, pullExplicit)
		return targets, nil, err
	}
	return s.resolveScoped(scope.Names, scope.HasArgs, scope.Namespace, opts, pullExplicit)
}

// DryRun prints the generated Dockerfile for tool instead of building it, recovering build options from the existing image's labels first.
func (s *Service) DryRun(tool, namespace string, opts tools.BuildOptions) error {
	if tool == "" {
		return fmt.Errorf("--dry-run requires a tool argument")
	}

	image, err := tools.ImageName(tool, namespace)
	if err == nil {
		if info, iErr := s.docker.InspectImage(image); iErr == nil && info != nil {
			opts = recoverOpts(s.home, info, opts)
		}
	}

	logging.Step(tool)
	content, err := tools.GenerateDockerfile(tool, opts)
	if err != nil {
		return err
	}

	_, err = fmt.Println(content)
	return err
}

// ApplyAll rebuilds every target in order, stopping at the first failure, then prunes what the builds left behind. All targets
// share one cache-bust value, so a tool updated in several namespaces (e.g. --all) can reuse its cached layers.
// Each update resets `agentic run`'s update check timer.
func (s *Service) ApplyAll(targets []Target) error {
	cacheBust := docker.NewCacheBust()
	for _, t := range targets {
		t.Opts.CacheBust = cacheBust
		if err := s.Apply(t.Name, t.Image, t.Opts); err != nil {
			return err
		}

		if err := toolupdate.MarkChecked(s.home, t.Name); err != nil {
			logging.Warnf("could not record update check for %s: %v", t.Name, err)
		}
	}

	// Best effort: a failed cleanup never fails the update
	_ = s.docker.PruneDangling()
	return nil
}

// ApplyRecovered rebuilds image for tool, reusing installed build options where possible; it's the toolupdate.Updater for `agentic run`'s auto-update prompt, so unlike `agentic update` it never exits the process.
func (s *Service) ApplyRecovered(tool, image string, rc *config.AgenticRC) error {
	opts := tools.BuildOptions{CustomInstalls: rc.Build.CustomInstalls}
	if info, err := s.docker.InspectImage(image); err == nil && info != nil {
		opts = recoverOpts(s.home, info, opts)
	}
	return s.Apply(tool, image, opts)
}

// Apply rebuilds image for tool, reporting base/apt/version before the build starts.
func (s *Service) Apply(name, image string, opts tools.BuildOptions) error {
	logging.Step(image)
	if len(opts.BaseOverride) > 0 {
		logging.Detailf("base: %s", strings.Join(opts.BaseOverride, ", "))
	} else if opts.BaseExact {
		logging.Detail("base: (none, exact)")
	}
	if len(opts.AptPackages) > 0 {
		logging.Detailf("apt: %s", strings.Join(opts.AptPackages, ", "))
	} else if opts.AptExact {
		logging.Detail("apt: (none, exact)")
	}

	info, err := s.docker.InspectImage(image)
	if err != nil {
		info = nil
	}

	upToDate := false
	if info != nil {
		latest, newer, ok := LatestToolVersion(name, info.Version)
		reportBeforeUpdate(info.Version, latest, newer, ok)
		upToDate = ok && !newer
	}

	return s.rebuild(name, image, info, upToDate, opts)
}

func (s *Service) resolveAll(filterTool string, opts tools.BuildOptions, pullExplicit bool) ([]Target, error) {
	var filters []docker.ImageFilter
	if filterTool != "" {
		filters = append(filters, docker.ToolFilter(filterTool))
	}

	images, err := s.docker.ListAllImages(filters...)
	if err != nil {
		return nil, err
	}

	var targets []Target
	for _, info := range images {
		if _, ok := tools.Configs[info.Tool]; !ok {
			continue
		}
		toolOpts := applyPullThrottle(recoverOpts(s.home, info, opts), info, pullExplicit)
		targets = append(targets, Target{Name: info.Tool, Image: info.Image, Opts: toolOpts})
	}
	return targets, nil
}

func (s *Service) resolveScoped(names []string, hasArgs bool, namespace string, opts tools.BuildOptions, pullExplicit bool) (targets []Target, skipped []string, err error) {
	skipUnbuilt := !hasArgs

	for _, name := range names {
		image, err := tools.ImageName(name, namespace)
		if err != nil {
			return nil, nil, err
		}

		info, err := s.docker.InspectImage(image)
		if err != nil {
			return nil, nil, err
		}

		if skipUnbuilt && info == nil {
			skipped = append(skipped, image)
			continue
		}

		toolOpts := opts
		if info != nil {
			toolOpts = recoverOpts(s.home, info, opts)
		}
		toolOpts = applyPullThrottle(toolOpts, info, pullExplicit)

		targets = append(targets, Target{Name: name, Image: image, Opts: toolOpts})
	}

	return targets, skipped, nil
}

// rebuild restamps an up-to-date image, or rebuilds it with base/version/apt recovered from info's labels.
func (s *Service) rebuild(tool, image string, info *docker.ImageInfo, upToDate bool, opts tools.BuildOptions) error {
	hasUserApt := len(opts.AptPackages) > 0
	userPkgs := opts.AptPackages
	opts.VerifyApt = hasUserApt

	if info != nil {
		if !opts.NoCache && !opts.Pull && upToDate {
			s.docker.RestampImage(image, *info)
			return nil
		}

		opts.BaseOverride = docker.RecoveredBaseOverride(info, opts)

		if info.VersionArgs != "" {
			opts.Versions = mergeVersions(docker.RecoverVersionArgs(info.VersionArgs), opts.Versions)
		}

		var recoveredPkgs []string
		opts.AptPackages, recoveredPkgs = docker.RecoveredAptPackages(info, opts)
		if recoveredPkgs != nil {
			opts.VerifyApt = hasUserApt && hasNewAptPackages(userPkgs, recoveredPkgs)
		}
	}

	if !opts.NoCache && upToDate {
		// Reuse the existing CacheBust (LabelCacheBust) instead of clearing it,
		// or Docker's cache falls back to a stale unrelated build.
		if info != nil {
			opts.CacheBust = info.CacheBust
		}
	} else if opts.CacheBust == "" {
		opts.CacheBust = docker.NewCacheBust()
	}

	if err := config.SaveCustomInstalls(s.home, opts.CustomInstalls); err != nil {
		return fmt.Errorf("saving custom installs: %w", err)
	}

	return s.docker.BuildTool(tool, image, opts)
}

// applyPullThrottle leaves opts.Pull untouched if --pull was explicit or there's no image to check; otherwise it disables auto-pull if agentic.pulled shows a pull within autoPullInterval.
func applyPullThrottle(opts tools.BuildOptions, info *docker.ImageInfo, pullExplicit bool) tools.BuildOptions {
	if pullExplicit || info == nil {
		return opts
	}

	if docker.PullIsFresh(info.Pulled, autoPullInterval) {
		opts.Pull = false
	}

	return opts
}

// recoverOpts fills opts from what info was built with; opts.CustomInstalls (the project config's) is only a fallback, see recoverCustomInstalls.
func recoverOpts(home string, info *docker.ImageInfo, opts tools.BuildOptions) tools.BuildOptions {
	opts.BaseOverride = docker.RecoveredBaseOverride(info, opts)
	opts.AptPackages, _ = docker.RecoveredAptPackages(info, opts)
	opts.CustomInstalls = recoverCustomInstalls(home, info, opts.CustomInstalls)
	return opts
}

// recoverCustomInstalls returns the custom installs info was built with from the store under home. When they aren't stored
// (an older image, or built from another machine), it looks the image's install names up in configInstalls and warns about any missing.
func recoverCustomInstalls(home string, info *docker.ImageInfo, configInstalls []config.RCCustomInstall) []config.RCCustomInstall {
	if installs, ok := config.LoadCustomInstalls(home, info.CustomInstallsHash); ok {
		return installs
	}

	var installs []config.RCCustomInstall
	var missing []string
	for name := range strings.SplitSeq(info.CustomInstalls, ",") {
		if name == "" {
			continue
		}

		idx := slices.IndexFunc(configInstalls, func(ci config.RCCustomInstall) bool { return ci.Name == name })
		if idx < 0 {
			missing = append(missing, name)
			continue
		}
		installs = append(installs, configInstalls[idx])
	}

	if len(missing) > 0 {
		logging.Warnf("%s: custom installs %s not found in .agenticrc.toml, rebuilding without them", info.Image, strings.Join(missing, ", "))
	}
	return installs
}

// reportBeforeUpdate prints the predicted version line from an upstream check, skipping inconclusive ones.
func reportBeforeUpdate(installedLabel, latest string, newer, ok bool) {
	if !ok {
		return
	}

	before := docker.ParseVersion(installedLabel)
	if newer {
		reportVersionChange(before, latest)
	} else {
		reportVersionChange(before, before)
	}
}

// reportVersionChange prints "version: X -> Y" or "version: X (up to date)".
func reportVersionChange(before, after string) {
	if after == "" {
		return
	}

	if before == "" {
		logging.Detailf("version: %s", after)
		return
	}

	if before != after {
		logging.Detailf("version: %s -> %s", before, after)
	} else {
		logging.Detailf("version: %s (up to date)", after)
	}
}

// mergeVersions combines recovered per-layer versions with user overrides, with overrides winning.
func mergeVersions(recovered, overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(recovered)+len(overrides))
	maps.Copy(merged, recovered)

	for name, ver := range overrides {
		if ver != "" {
			merged[name] = ver
		}
	}

	return merged
}

// hasNewAptPackages returns true if any package in requested is not present in existing.
func hasNewAptPackages(requested, existing []string) bool {
	for _, pkg := range requested {
		if !slices.Contains(existing, pkg) {
			return true
		}
	}
	return false
}
