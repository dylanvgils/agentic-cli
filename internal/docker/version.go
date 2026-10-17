package docker

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var versionRe = regexp.MustCompile(`[0-9]+(\.[0-9]+)*`)

// serverComponent is one of the parts docker version lists for the daemon, such as its engine or containerd.
type serverComponent struct {
	Name    string
	Version string
}

// Version returns the docker CLI's own version, which needs no running daemon.
func (c *Client) Version() (string, error) {
	out, err := c.run("version", arg("format", "{{.Client.Version}}"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ServerVersion returns the name and version of the daemon's engine, e.g. "Engine" for Docker; unlike Version it needs
// the daemon, and with a context it reads that context's daemon.
func (c *Client) ServerVersion() (engine, version string, err error) {
	out, err := c.run("version", arg("format", "{{json .Server.Components}}"))
	if err != nil {
		return "", "", err
	}

	var components []serverComponent
	if err := json.Unmarshal([]byte(out), &components); err != nil {
		return "", "", fmt.Errorf("parse docker version output: %w", err)
	}

	component, err := engineComponent(components)
	if err != nil {
		return "", "", err
	}
	return component.Name, component.Version, nil
}

// runVersionScript runs script network-less so a self-updating tool can't change its reported version mid-detection.
func (c *Client) runVersionScript(image, script string) string {
	out, err := c.run("run", arg("rm"), arg("network", "none"), arg("entrypoint", ""), image, script)
	if err != nil {
		return ""
	}
	return extractVersion(out)
}

// collectExtraVersions detects the installed version for each extra layer in image, keyed by layer name (empty string on detection failure).
func (c *Client) collectExtraVersions(image string, extras []string) map[string]string {
	versions := make(map[string]string)
	for _, extra := range extras {
		versions[extra] = c.runVersionScript(image, versionScript(extra))
	}
	return versions
}

// collectBaseLabel detects all extra-layer versions from the image and assembles the agentic.base label value.
func (c *Client) collectBaseLabel(image string, extras []string) string {
	extraVersions := c.collectExtraVersions(image, extras)
	return buildBaseLabel(extras, extraVersions)
}

func extractVersion(out string) string {
	line, _, _ := strings.Cut(out, "\n")
	line = strings.TrimRight(line, "\r")
	return versionRe.FindString(line)
}

// engineComponent returns the component named like an engine, e.g. "Engine" or "Podman Engine", else the first one.
func engineComponent(components []serverComponent) (serverComponent, error) {
	if len(components) == 0 {
		return serverComponent{}, errors.New("docker version lists no server components")
	}

	for _, c := range components {
		if strings.HasSuffix(c.Name, "Engine") {
			return c, nil
		}
	}
	return components[0], nil
}
