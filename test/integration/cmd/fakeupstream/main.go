//go:build integration

// Command fakeupstream is the proxy service tests' upstream: HTTP on :80 and HTTPS on :443, answering every request
// with its path and whether it carried the expected secret.
package main

import (
	"fmt"
	"net/http"
	"os"
)

const (
	// certFile and keyFile are the HTTPS leaf, mounted by platform.FakeResource.ServingUpstream.
	certFile = "/certs/upstream.pem"
	keyFile  = "/certs/upstream-key.pem"

	// secretEnv holds the Authorization value a credential request must arrive with.
	secretEnv = "EXPECT_SECRET"
)

func main() {
	handler := http.HandlerFunc(answer)
	errs := make(chan error, 2)
	go func() { errs <- http.ListenAndServe(":80", handler) }()
	go func() { errs <- http.ListenAndServeTLS(":443", certFile, keyFile, handler) }()

	fmt.Println(<-errs)
	os.Exit(1)
}

// answer writes the request path and whether its Authorization is the secret, never the secret itself, and echoes the
// Authorization back as a header, so a test sees what the proxy scrubs.
func answer(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	secret := "mismatch"
	if want := os.Getenv(secretEnv); want != "" && auth == want {
		secret = "match"
	}

	w.Header().Set("X-Echo-Authorization", auth)
	_, _ = fmt.Fprintf(w, "path=%s\nsecret=%s\n", r.URL.Path, secret)
}
