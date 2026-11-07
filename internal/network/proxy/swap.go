package proxy

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// scrubWriter scrubs the headers of 1xx responses, which ReverseProxy writes without ModifyResponse.
type scrubWriter struct {
	http.ResponseWriter
	scrub *strings.Replacer
}

// WriteHeader scrubs the header before a 1xx response; final headers are already scrubbed in ModifyResponse.
func (w *scrubWriter) WriteHeader(code int) {
	if code < http.StatusOK {
		scrubHeader(w.Header(), w.scrub)
	}
	w.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController flush and hijack the wrapped writer.
func (w *scrubWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// substitute swaps cred's placeholder for its secret in its headers and drops their aliases.
func substitute(header http.Header, cred Credential) {
	for _, name := range cred.Headers {
		canonical := http.CanonicalHeaderKey(name)
		for key := range header {
			if key != canonical && headerAlias(key) == headerAlias(canonical) {
				delete(header, key)
			}
		}

		values := header[canonical]
		for i, value := range values {
			values[i] = substituteValue(value, cred.Placeholder, cred.Secret)
		}
	}
}

// scrubHeader swaps the secret back to its placeholder in every header, so an echo can't leak it; bodies aren't scrubbed.
func scrubHeader(header http.Header, scrub *strings.Replacer) {
	for _, values := range header {
		for i, value := range values {
			values[i] = scrub.Replace(value)
		}
	}
}

// newScrubber returns a replacer from cred's secret to its placeholder, plain and in the sent Basic forms.
func newScrubber(header http.Header, cred Credential) *strings.Replacer {
	// An empty secret would put the placeholder between every byte
	if cred.Secret == "" {
		return strings.NewReplacer()
	}

	// Encoded forms first, as the replacer tries its pairs in order
	var pairs []string
	for _, name := range cred.Headers {
		for _, value := range header.Values(name) {
			sent, ok := basicCredential(substituteValue(value, cred.Placeholder, cred.Secret))
			original, _ := basicCredential(value)
			if ok && sent != original {
				pairs = append(pairs, sent, original)
			}
		}
	}
	return strings.NewReplacer(append(pairs, cred.Secret, cred.Placeholder)...)
}

// hasPlaceholder reports whether any of cred's headers in header carries its placeholder, plain or Basic-encoded.
func hasPlaceholder(header http.Header, cred Credential) bool {
	for _, name := range cred.Headers {
		for _, value := range header.Values(name) {
			if substituteValue(value, cred.Placeholder, cred.Secret) != value {
				return true
			}
		}
	}
	return false
}

// substituteValue replaces placeholder with secret, inside a Basic credential too (git, curl -u).
func substituteValue(value, placeholder, secret string) string {
	if encoded, ok := basicCredential(value); ok {
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err == nil && strings.Contains(string(decoded), placeholder) {
			swapped := strings.ReplaceAll(string(decoded), placeholder, secret)
			return strings.Replace(value, encoded, base64.StdEncoding.EncodeToString([]byte(swapped)), 1)
		}
	}
	return strings.ReplaceAll(value, placeholder, secret)
}

// basicCredential returns the encoded credential of a Basic value.
func basicCredential(value string) (string, bool) {
	scheme, encoded, found := strings.Cut(value, " ")
	if !found || !strings.EqualFold(scheme, "Basic") {
		return "", false
	}
	return strings.TrimSpace(encoded), true
}

// headerAlias folds case and treats "_" as "-", the way CGI-style backends compare header names.
func headerAlias(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "_", "-")
}
