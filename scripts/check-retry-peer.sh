#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
task="$(mktemp -d "${TMPDIR:-/tmp}/go-http-client-retry-peer.XXXXXX")"
trap 'chmod -R u+w "${task}" 2>/dev/null || true; find "${task}" -depth -delete' EXIT

cat >"${task}/go.mod" <<EOF
module httpclientretrypeer

go 1.26.6

require (
	github.com/faustbrian/go-http-client v0.0.0
	github.com/faustbrian/go-circuit-breaker v1.0.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
)

replace github.com/faustbrian/go-http-client => ${root}
EOF
awk '1' "${root}/go.sum" "${root}/specification/retry-peer.sum" >"${task}/go.sum"

cat >"${task}/retry_peer_test.go" <<'EOF'
package httpclientretrypeer

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	httpclient "github.com/faustbrian/go-http-client"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

func TestRetryPolicyDiffersFromMaintainedPeer(t *testing.T) {
	var localAttempts atomic.Int32
	localServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		localAttempts.Add(1)
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer localServer.Close()
	retry, err := httpclient.NewRetryMiddleware(httpclient.RetryOptions{
		Name: "retry", MaximumAttempts: 2, Delays: []time.Duration{time.Nanosecond},
	})
	if err != nil { t.Fatal(err) }
	localClient, err := httpclient.New(httpclient.Config{Middleware: []httpclient.Middleware{retry}})
	if err != nil { t.Fatal(err) }
	defer localClient.Close()
	request, _ := http.NewRequest(http.MethodPost, localServer.URL, strings.NewReader("body"))
	response, err := localClient.Do(request)
	if err != nil { t.Fatal(err) }
	_ = response.Body.Close()
	if localAttempts.Load() != 1 { t.Fatalf("local attempts = %d", localAttempts.Load()) }

	var peerAttempts atomic.Int32
	peerServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if peerAttempts.Add(1) == 1 { writer.WriteHeader(http.StatusServiceUnavailable); return }
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer peerServer.Close()
	peer := retryablehttp.NewClient()
	peer.Logger = nil
	peer.RetryMax = 1
	peer.Backoff = func(time.Duration, time.Duration, int, *http.Response) time.Duration { return 0 }
	peerRequest, _ := retryablehttp.NewRequest(http.MethodPost, peerServer.URL, []byte("body"))
	peerResponse, err := peer.Do(peerRequest)
	if err != nil { t.Fatal(err) }
	_ = peerResponse.Body.Close()
	if peerAttempts.Load() != 2 { t.Fatalf("peer attempts = %d", peerAttempts.Load()) }
}
EOF

cd "${task}"
GOWORK=off go test -mod=readonly -count=1 .
