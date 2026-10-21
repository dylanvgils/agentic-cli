package config

import (
	"fmt"
	"strings"
	"unicode"
)

// Wildcard segments of a path pattern.
const (
	segmentWildcard = "*"  // exactly one non-empty segment
	restWildcard    = "**" // zero or more segments, last only
)

// patternForbidden are characters a path pattern can't hold: encodings, queries and separators other servers split on.
const patternForbidden = "%?#;\\"

// ValidatePathPattern checks that pattern is an absolute path of literal segments, "*" or a final "**", with no dot
// segments, empty inner segments or partial wildcards.
func ValidatePathPattern(pattern string) error {
	if !strings.HasPrefix(pattern, "/") {
		return fmt.Errorf("path %q must start with /", pattern)
	}
	if strings.ContainsAny(pattern, patternForbidden) || strings.ContainsFunc(pattern, unicode.IsSpace) {
		return fmt.Errorf("path %q must not contain spaces or any of %s", pattern, patternForbidden)
	}
	if strings.ContainsFunc(pattern, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return fmt.Errorf("path %q contains control or non-printable characters", pattern)
	}

	segments := strings.Split(pattern[1:], "/")
	for i, segment := range segments {
		last := i == len(segments)-1
		switch {
		case segment == "." || segment == "..":
			return fmt.Errorf("path %q must not contain . or .. segments", pattern)
		case segment == "" && !last:
			return fmt.Errorf("path %q must not contain empty segments", pattern)
		case segment == restWildcard && !last:
			return fmt.Errorf("path %q may only use ** as its last segment", pattern)
		case segment != segmentWildcard && segment != restWildcard && strings.Contains(segment, "*"):
			return fmt.Errorf("path %q: * must be a whole segment", pattern)
		}
	}
	return nil
}

// MatchPathPattern reports whether path matches a pattern ValidatePathPattern accepts: literal segments match exactly,
// "*" one non-empty segment and a final "**" whatever is left, including nothing.
func MatchPathPattern(pattern, path string) bool {
	if !strings.HasPrefix(pattern, "/") || !strings.HasPrefix(path, "/") {
		return false
	}

	patternSegments := strings.Split(pattern[1:], "/")
	pathSegments := strings.Split(path[1:], "/")
	for i, segment := range patternSegments {
		if segment == restWildcard && i == len(patternSegments)-1 {
			return true
		}
		if i >= len(pathSegments) {
			return false
		}
		if segment == segmentWildcard && pathSegments[i] != "" {
			continue
		}
		if segment != pathSegments[i] {
			return false
		}
	}
	return len(pathSegments) == len(patternSegments)
}
