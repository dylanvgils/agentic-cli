package proxy

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// pathBlockedText is the body of a 403 for a path no rule of its host allows.
const pathBlockedText = "path not allowed by agentic proxy allowlist"

// unsafePathParts are what upstreams may decode or split on differently than the rule check, in a lowercased escaped path:
// encoded separators, dots and %, backslashes and ; path parameters.
var unsafePathParts = []string{"%2f", "%5c", "%2e", "%25", "%3b", "\\", ";"}

// checkPaths answers requests whose path entry's host rules don't allow with a 403, logging each; monitor mode logs
// and forwards them, dropping upgrades on path rule hosts.
func (s *Server) checkPaths(next http.Handler, entry Entry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.passPath(w, entry, r.URL) {
			return
		}
		if s.allow.pathRestricted(entry.Host) {
			dropUpgrade(r.Header)
		}
		next.ServeHTTP(w, r)
	})
}

// passPath reports whether a request to u may go on; monitor mode logs a refused path on its own line first.
func (s *Server) passPath(w http.ResponseWriter, entry Entry, u *url.URL) bool {
	blocked, ok := s.pathVerdict(entry, u)
	if ok {
		return true
	}
	if blocked.Enforced {
		s.refusePath(w, blocked)
		return false
	}
	s.logger.Log(blocked)
	return true
}

// refusePath logs entry and answers 403 for a path its host's rules don't allow.
func (s *Server) refusePath(w http.ResponseWriter, entry Entry) {
	s.logger.Log(entry)
	http.Error(w, pathBlockedText, http.StatusForbidden)
}

// pathVerdict reports whether u's path is allowed on entry's host, always for a host without path rules, returning
// entry marked as path-blocked when it isn't.
func (s *Server) pathVerdict(entry Entry, u *url.URL) (Entry, bool) {
	if !s.allow.pathRestricted(entry.Host) {
		return entry, true
	}

	path, safe := requestPath(u)
	if safe && s.allow.AllowsPath(entry.Host, path) {
		return entry, true
	}

	entry = s.markPathBlocked(entry)
	entry.Path = u.EscapedPath()
	return entry, false
}

// markPathBlocked returns entry denied for its path, enforced unless in monitor mode.
func (s *Server) markPathBlocked(entry Entry) Entry {
	entry.Decision = DecisionDeny
	entry.Enforced = !s.monitor
	entry.Reason = ReasonPathBlocked
	return entry
}

// requestPath returns u's decoded path ("/" when empty), or ok=false for one an upstream may read differently.
func requestPath(u *url.URL) (string, bool) {
	if u.Opaque != "" {
		return "", false
	}

	escaped := strings.ToLower(u.EscapedPath())
	if escaped == "" {
		return "/", true
	}
	if !strings.HasPrefix(escaped, "/") {
		return "", false
	}
	if slices.ContainsFunc(unsafePathParts, func(part string) bool { return strings.Contains(escaped, part) }) {
		return "", false
	}

	if strings.ContainsFunc(u.Path, func(r rune) bool { return r < ' ' || r > '~' }) {
		return "", false
	}

	for segment := range strings.SplitSeq(u.Path, "/") {
		if strings.HasSuffix(segment, ".") || strings.HasSuffix(segment, " ") {
			return "", false
		}
	}
	return u.Path, true
}

// dropUpgrade removes non-WebSocket upgrades, whose connections would skip the path check.
func dropUpgrade(h http.Header) {
	if strings.EqualFold(strings.TrimSpace(h.Get("Upgrade")), "websocket") {
		return
	}
	h.Del("Upgrade")
	h.Del("HTTP2-Settings")
}
