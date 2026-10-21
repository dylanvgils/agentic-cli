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
// encoded separators and dots, backslashes and ; path parameters.
var unsafePathParts = []string{"%2f", "%5c", "%2e", "%3b", "\\", ";"}

// checkPaths answers requests whose path entry's host rules don't allow with a 403, logging each; monitor mode logs
// and forwards them.
func (s *Server) checkPaths(next http.Handler, entry Entry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if blocked, ok := s.pathVerdict(entry, r.URL); !ok {
			if blocked.Enforced {
				s.refusePath(w, blocked)
				return
			}
			s.logger.Log(blocked)
		}
		next.ServeHTTP(w, r)
	})
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

// requestPath returns u's decoded path ("/" when empty), or ok=false when its escaped form holds unsafePathParts or it
// has dot segments, which a rule could match while the upstream serves another path.
func requestPath(u *url.URL) (string, bool) {
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

	for segment := range strings.SplitSeq(u.Path, "/") {
		if segment == "." || segment == ".." {
			return "", false
		}
	}
	return u.Path, true
}
