//go:build integration

// Command fakeupstream is the tests' HTTP (:80) and HTTPS (:443) upstream, answering with the path and secret match.
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

// answer writes the path and whether Authorization is the secret, echoing it as a header to check the scrub.
func answer(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	secret := "mismatch"
	if want := os.Getenv(secretEnv); want != "" && auth == want {
		secret = "match"
	}

	w.Header().Set("X-Echo-Authorization", auth)
	_, _ = fmt.Fprintf(w, "path=%s\nsecret=%s\n", r.URL.Path, secret)
}
