/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rpp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// goneEndpoint is an address nothing listens on: the proxy a node was pointed at before its
// Deployment rolled to other masters.
func goneEndpoint(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	addr := listener.Addr().String()
	assertNoError(t, listener.Close())
	return addr
}

func packageArchive(t *testing.T) []byte {
	t.Helper()

	return buildArchive(t, map[string]packageArchiveFile{
		"install":   {mode: 0o755, body: shellScript("exit 0")},
		"uninstall": {mode: 0o755, body: shellScript("exit 0")},
	})
}

func TestAFetchFollowsTheProxyWhereItMoved(t *testing.T) {
	server, count := newArchiveServer(t, "moved-token", map[string][]byte{"sha256:moved": packageArchive(t)})

	client := newTestClient(t, []string{goneEndpoint(t)})
	client.cfg.Retries = 3
	var asked atomic.Int32
	client.SetRediscover(func(context.Context) ([]string, string, error) {
		asked.Add(1)
		return []string{serverHost(t, server)}, "moved-token", nil
	})
	ref := mustPackageRef(t, client, "theta:sha256:moved")

	if err := client.installPackage(context.Background(), ref); err != nil {
		t.Fatalf("installPackage() error = %v", err)
	}

	if got := asked.Load(); got != 1 {
		t.Fatalf("rediscover calls = %d, want 1", got)
	}
	if got := count.Load(); got != 1 {
		t.Fatalf("requests to the proxy where it moved = %d, want 1", got)
	}
	assertFileContent(t, filepath.Join(ref.installedDir, "digest"), ref.digest+"\n")
}

func TestAFetchThatSucceedsDoesNotAskWhereTheProxyIs(t *testing.T) {
	server, _ := newArchiveServer(t, "test-token", map[string][]byte{"sha256:here": packageArchive(t)})

	client := newTestClient(t, []string{serverHost(t, server)})
	client.SetRediscover(func(context.Context) ([]string, string, error) {
		t.Fatal("rediscover called after a fetch that succeeded")
		return nil, "", nil
	})
	ref := mustPackageRef(t, client, "iota:sha256:here")

	if err := client.installPackage(context.Background(), ref); err != nil {
		t.Fatalf("installPackage() error = %v", err)
	}
}

func TestAFailedRediscoveryKeepsTheEndpoints(t *testing.T) {
	for name, rediscover := range map[string]func(context.Context) ([]string, string, error){
		"the cluster is unreachable": func(context.Context) ([]string, string, error) {
			return nil, "", errors.New("connection refused")
		},
		"no proxy is running": func(context.Context) ([]string, string, error) {
			return nil, "token", nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			endpoint := goneEndpoint(t)
			client := newTestClient(t, []string{endpoint})
			client.SetRediscover(rediscover)

			client.rediscover(context.Background(), mustPackageRef(t, client, "kappa:sha256:kept"))

			if got := client.http().endpoints; len(got) != 1 || got[0] != endpoint {
				t.Fatalf("endpoints = %v, want [%s]", got, endpoint)
			}
			if got := client.http().token; got != "test-token" {
				t.Fatalf("token = %q, want the one in use", got)
			}
		})
	}
}

func TestWorkersAskWhereTheProxyIsOncePerRetryDelay(t *testing.T) {
	client := newTestClient(t, []string{goneEndpoint(t)})
	client.cfg.RetryDelay = 1 << 40
	var asked atomic.Int32
	client.SetRediscover(func(context.Context) ([]string, string, error) {
		asked.Add(1)
		return []string{goneEndpoint(t)}, "test-token", nil
	})
	ref := mustPackageRef(t, client, "lambda:sha256:once")

	for range 5 {
		client.rediscover(context.Background(), ref)
	}

	if got := asked.Load(); got != 1 {
		t.Fatalf("rediscover calls = %d, want 1", got)
	}
}

// flakyAuthServer answers 401 to its first `refusals` requests — a kube-rbac-proxy that cannot reach
// the API server to review the token — and serves the archive after that.
func flakyAuthServer(t *testing.T, refusals int32, archives map[string][]byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var count atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count.Add(1) <= refusals {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		archive, ok := archives[r.URL.Query().Get("digest")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(archive)
	}))
	t.Cleanup(server.Close)
	return server, &count
}

// TestAJoiningNodeOutlastsAProxyThatCannotReviewTokens is master-2 of scenarios, job 7318483: the only
// ready proxy answered 401 for the minute its API server was out of reach, and the node spent every
// attempt it had inside that minute.
func TestAJoiningNodeOutlastsAProxyThatCannotReviewTokens(t *testing.T) {
	server, count := flakyAuthServer(t, 2, map[string][]byte{"sha256:review": packageArchive(t)})

	client := newTestClient(t, []string{serverHost(t, server)})
	client.cfg.Retries = 5
	client.SetRediscover(func(context.Context) ([]string, string, error) {
		return []string{serverHost(t, server)}, "test-token", nil
	})
	ref := mustPackageRef(t, client, "mu:sha256:review")

	if err := client.installPackage(context.Background(), ref); err != nil {
		t.Fatalf("installPackage() error = %v", err)
	}
	if got := count.Load(); got != 3 {
		t.Fatalf("requests = %d, want two refused and one served", got)
	}
}

// TestAGivenTokenThatIsRefusedIsNotRetried is the other side: endpoints and token passed in by the
// caller cannot be asked for again, so a 401 is the answer and not a moment.
func TestAGivenTokenThatIsRefusedIsNotRetried(t *testing.T) {
	server, count := flakyAuthServer(t, 100, map[string][]byte{"sha256:given": packageArchive(t)})

	client := newTestClient(t, []string{serverHost(t, server)})
	client.cfg.Retries = 5
	ref := mustPackageRef(t, client, "nu:sha256:given")

	if err := client.fetchArchive(context.Background(), ref); err == nil {
		t.Fatal("fetchArchive() error = nil, want the 401")
	}
	if got := count.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1: a given token refused is not asked for again", got)
	}
}

// TestAForbiddenFetchIsNotRetried: 403 is a token the proxy reviewed and refused, not one it could not
// review, so asking again — for up to half an hour on every node — would change nothing.
func TestAForbiddenFetchIsNotRetried(t *testing.T) {
	var count atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count.Add(1)
		http.Error(w, "Forbidden", http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	client := newTestClient(t, []string{serverHost(t, server)})
	client.cfg.Retries = 5
	client.SetRediscover(func(context.Context) ([]string, string, error) {
		return []string{serverHost(t, server)}, "test-token", nil
	})
	ref := mustPackageRef(t, client, "xi:sha256:forbidden")

	if err := client.fetchArchive(context.Background(), ref); err == nil {
		t.Fatal("fetchArchive() error = nil, want the 403")
	}
	if got := count.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

// TestARefusedTokenTheClusterGivesBackEndsTheSeries: a token that is simply wrong comes back the same
// from the cluster, and the 401 after that is the answer rather than one more of thirty.
func TestARefusedTokenTheClusterGivesBackEndsTheSeries(t *testing.T) {
	server, count := flakyAuthServer(t, 100, map[string][]byte{"sha256:same": packageArchive(t)})

	client := newTestClient(t, []string{serverHost(t, server)})
	client.cfg.Retries = 30
	var asked atomic.Int32
	client.SetRediscover(func(context.Context) ([]string, string, error) {
		asked.Add(1)
		return []string{serverHost(t, server)}, "test-token", nil
	})
	ref := mustPackageRef(t, client, "omicron:sha256:same")

	if err := client.fetchArchive(context.Background(), ref); err == nil {
		t.Fatal("fetchArchive() error = nil, want the 401")
	}
	if got := asked.Load(); got != 1 {
		t.Fatalf("rediscover calls = %d, want 1", got)
	}
	if got := count.Load(); got != 2 {
		t.Fatalf("requests = %d, want 2: one refused, one refused again after the same answer", got)
	}
}

// TestATokenTheClusterChangedIsTriedAgain is the joining master whose proxy hands out a new token more
// than once: every new answer is worth another attempt.
func TestATokenTheClusterChangedIsTriedAgain(t *testing.T) {
	server, count := newArchiveServer(t, "rotated-2", map[string][]byte{"sha256:rotated": packageArchive(t)})

	client := newTestClient(t, []string{serverHost(t, server)})
	client.cfg.Retries = 5
	var asked atomic.Int32
	client.SetRediscover(func(context.Context) ([]string, string, error) {
		return []string{serverHost(t, server)}, fmt.Sprintf("rotated-%d", asked.Add(1)), nil
	})
	ref := mustPackageRef(t, client, "pi:sha256:rotated")

	if err := client.fetchArchive(context.Background(), ref); err != nil {
		t.Fatalf("fetchArchive() error = %v", err)
	}
	if got := count.Load(); got != 3 {
		t.Fatalf("requests = %d, want two refused and one served", got)
	}
}

// TestDecidingOnARetryDoesNotWaitForARediscovery: a worker whose fetch failed decides at once, rather
// than behind another worker's request to the API server.
func TestDecidingOnARetryDoesNotWaitForARediscovery(t *testing.T) {
	client := newTestClient(t, []string{goneEndpoint(t)})
	client.SetRediscover(func(context.Context) ([]string, string, error) {
		return nil, "", nil
	})

	client.rediscoverMu.Lock()
	defer client.rediscoverMu.Unlock()

	decided := make(chan bool, 1)
	go func() {
		decided <- client.shouldRetryFetch(&httpStatusError{statusCode: http.StatusUnauthorized}, false)
	}()

	select {
	case retry := <-decided:
		if !retry {
			t.Fatal("shouldRetryFetch() = false, want a 401 retried while the cluster can be asked")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("shouldRetryFetch() waited for the rediscovery in flight")
	}
}
