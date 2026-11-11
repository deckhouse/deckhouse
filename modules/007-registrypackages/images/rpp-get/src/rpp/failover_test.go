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
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

const (
	otherZoneMaster      = "192.0.2.1:4219"
	thirdZoneMaster      = "192.0.2.2:4219"
	sameZonePackageCount = 10
)

func goneEndpoint(t *testing.T) string {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
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

// unroutable makes dials to the given endpoints fail as they do to a master in another zone before
// the node's routes are up. A one-nanosecond dial gives the dialer's own "i/o timeout" error.
func unroutable(t *testing.T, client *Client, endpoints ...string) *atomic.Int32 {
	t.Helper()

	var dials atomic.Int32

	transport := client.httpClient.client.Transport.(*http.Transport)
	dial := transport.DialContext
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if !slices.Contains(endpoints, addr) {
			return dial(ctx, network, addr)
		}

		dials.Add(1)

		return (&net.Dialer{Timeout: time.Nanosecond}).DialContext(ctx, network, addr)
	}

	return &dials
}

func TestAFetchMovesOnToAnEndpointThatAnswers(t *testing.T) {
	archives := make(map[string][]byte, sameZonePackageCount)
	for i := range sameZonePackageCount {
		archives[fmt.Sprintf("sha256:failover%d", i)] = packageArchive(t)
	}

	server, count := newArchiveServer(t, "test-token", archives)

	client := newTestClient(t, []string{goneEndpoint(t), serverHost(t, server), goneEndpoint(t)})
	for i := range sameZonePackageCount {
		ref := mustPackageRef(t, client, fmt.Sprintf("pkg%d:sha256:failover%d", i, i))
		if err := client.fetchArchive(t.Context(), ref); err != nil {
			t.Fatalf("fetchArchive(%s) error = %v, want the archive from the endpoint that answers", ref.name, err)
		}
	}

	if got := count.Load(); got != sameZonePackageCount {
		t.Fatalf("requests to the endpoint that answers = %d, want %d", got, sameZonePackageCount)
	}
}

func TestAFetchSpreadsRequestsOverEndpoints(t *testing.T) {
	const fetches = 60

	archives := map[string][]byte{"sha256:spread": packageArchive(t)}

	var (
		servers []string
		counts  []*atomic.Int32
	)

	for range 3 {
		server, count := newArchiveServer(t, "test-token", archives)
		servers = append(servers, serverHost(t, server))
		counts = append(counts, count)
	}

	client := newTestClient(t, servers)
	ref := mustPackageRef(t, client, "spread:sha256:spread")

	for range fetches {
		if err := client.fetchArchive(t.Context(), ref); err != nil {
			t.Fatalf("fetchArchive() error = %v", err)
		}
	}

	for i, count := range counts {
		if count.Load() == 0 {
			t.Fatalf("endpoint %s got no requests out of %d", servers[i], fetches)
		}
	}
}

func TestAFetchThatFailsEverywhereReportsEveryEndpoint(t *testing.T) {
	endpoints := []string{goneEndpoint(t), goneEndpoint(t), goneEndpoint(t)}
	client := newTestClient(t, endpoints)

	err := client.fetchArchive(t.Context(), mustPackageRef(t, client, "rho:sha256:nowhere"))

	var failures interface{ Unwrap() []error }
	if !errors.As(err, &failures) {
		t.Fatalf("fetchArchive() error = %v, want the failure of every endpoint", err)
	}

	if got := len(failures.Unwrap()); got != len(endpoints) {
		t.Fatalf("failures = %d, want one per endpoint (%d): %v", got, len(endpoints), err)
	}
}

func TestAFetchThatTimedOutIsRetried(t *testing.T) {
	client := newTestClient(t, []string{otherZoneMaster})
	client.cfg.Retries = 3
	dials := unroutable(t, client, otherZoneMaster)

	err := client.fetchArchive(t.Context(), mustPackageRef(t, client, "sigma:sha256:timeout"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("fetchArchive() error = %v, want the dial timeout", err)
	}

	if got := dials.Load(); got != 3 {
		t.Fatalf("dials = %d, want 3: every attempt after a dial timeout", got)
	}
}

func TestAnInstallRetriesAFetchThatTimedOut(t *testing.T) {
	client := newTestClient(t, []string{otherZoneMaster})
	dials := unroutable(t, client, otherZoneMaster)

	if err := client.installPackage(t.Context(), mustPackageRef(t, client, "tau:sha256:timeout")); err == nil {
		t.Fatal("installPackage() error = nil, want the dial timeout")
	}

	if got := dials.Load(); got != packageInstallAttempts {
		t.Fatalf("dials = %d, want %d: one fetch per install attempt", got, packageInstallAttempts)
	}
}

// TestANodeInstallsFromTheMasterOfItsZone is a node in one zone of three before bootstrap-networks
// gives it routes to the others: it reaches one master of three, and only by its address.
func TestANodeInstallsFromTheMasterOfItsZone(t *testing.T) {
	const nodes = 5

	archives := map[string][]byte{
		"sha256:jq":      packageArchive(t),
		"sha256:curl":    packageArchive(t),
		"sha256:taillog": packageArchive(t),
	}
	packages := []string{"jq:sha256:jq", "curl:sha256:curl", "tailLog:sha256:taillog"}

	for node := range nodes {
		server, _ := newArchiveServer(t, "test-token", archives)
		client := newTestClient(t, []string{otherZoneMaster, serverHost(t, server), thirdZoneMaster})
		unroutable(t, client, otherZoneMaster, thirdZoneMaster)

		if err := client.InstallAll(t.Context(), packages); err != nil {
			t.Fatalf("node %d: InstallAll() error = %v", node, err)
		}

		for _, name := range []string{"jq", "curl", "tailLog"} {
			assertPathExists(t, filepath.Join(client.cfg.InstalledStore, name, "digest"))
		}
	}
}

func TestACanceledFetchIsNotRetried(t *testing.T) {
	server, count := newArchiveServer(t, "test-token", map[string][]byte{"sha256:canceled": packageArchive(t)})
	client := newTestClient(t, []string{serverHost(t, server)})
	client.cfg.Retries = 5

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := client.installPackage(ctx, mustPackageRef(t, client, "upsilon:sha256:canceled"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("installPackage() error = %v, want context.Canceled", err)
	}

	if got := count.Load(); got != 0 {
		t.Fatalf("requests = %d, want none after the caller gave up", got)
	}
}

func TestAFetchIsRetriedWhileAnEndpointDidNotAnswer(t *testing.T) {
	server, count := newArchiveServer(t, "test-token", nil)
	client := newTestClient(t, []string{otherZoneMaster, serverHost(t, server)})
	client.cfg.Retries = 3
	dials := unroutable(t, client, otherZoneMaster)

	err := client.fetchArchive(t.Context(), mustPackageRef(t, client, "phi:sha256:absent"))

	var statusErr *httpStatusError
	if !errors.As(err, &statusErr) || statusErr.statusCode != http.StatusNotFound {
		t.Fatalf("fetchArchive() error = %v, want the 404 among the failures", err)
	}

	if count.Load() != 3 || dials.Load() != 3 {
		t.Fatalf("requests = %d and dials = %d, want 3 each: the endpoint that did not answer may have the package",
			count.Load(), dials.Load())
	}
}

func TestDownloadsStopWhenTheOutageBudgetIsSpent(t *testing.T) {
	client := newTestClient(t, []string{otherZoneMaster})
	client.cfg.Retries = 30
	dials := unroutable(t, client, otherZoneMaster)

	started := time.Now()
	client.now = func() time.Time {
		return started.Add(time.Duration(dials.Load()) * connectTimeout)
	}

	if err := client.installPackage(t.Context(), mustPackageRef(t, client, "chi:sha256:outage")); err == nil {
		t.Fatal("installPackage() error = nil, want the dial timeout")
	}

	want := int32((outageBudget + connectTimeout - 1) / connectTimeout)
	if got := dials.Load(); got != want {
		t.Fatalf("dials = %d, want %d: one every %s for %s", got, want, connectTimeout, outageBudget)
	}
}

func TestARoundStopsBetweenEndpointsWhenTheOutageBudgetIsSpent(t *testing.T) {
	client := newTestClient(t, []string{otherZoneMaster, thirdZoneMaster})
	dials := unroutable(t, client, otherZoneMaster, thirdZoneMaster)

	started := time.Now()
	client.now = func() time.Time {
		return started.Add(time.Duration(dials.Load()) * outageBudget)
	}
	client.OutageBegan(started)

	if err := client.fetchArchive(t.Context(), mustPackageRef(t, client, "psi:sha256:round")); err == nil {
		t.Fatal("fetchArchive() error = nil, want the dial timeout")
	}

	if got := dials.Load(); got != 1 {
		t.Fatalf("dials = %d, want 1: no request starts after the budget is spent", got)
	}
}

// TestADeadEndpointDoesNotStartTheOutage is a cluster with one master down and a run longer than
// the budget: a failed endpoint is not an outage while another one serves.
func TestADeadEndpointDoesNotStartTheOutage(t *testing.T) {
	const packages = 20

	archives := make(map[string][]byte, packages)
	for i := range packages {
		archives[fmt.Sprintf("sha256:long%d", i)] = packageArchive(t)
	}

	server, _ := newArchiveServer(t, "test-token", archives)
	client := newTestClient(t, []string{otherZoneMaster, serverHost(t, server)})
	unroutable(t, client, otherZoneMaster)

	clock := time.Now()
	client.now = func() time.Time { return clock }

	for i := range packages {
		if i == packages/2 {
			clock = clock.Add(outageBudget + time.Second)
		}

		ref := mustPackageRef(t, client, fmt.Sprintf("long%d:sha256:long%d", i, i))
		if err := client.fetchArchive(t.Context(), ref); err != nil {
			t.Fatalf("fetchArchive(%s) error = %v, want the archive from the endpoint that serves", ref.name, err)
		}
	}
}

func TestTheOutageRunsFromTheFirstFailedRoundToADownload(t *testing.T) {
	client := newTestClient(t, []string{otherZoneMaster})

	began := time.Now()
	clock := began.Add(responseHeaderTimeout)
	client.now = func() time.Time { return clock }

	client.roundFailed(began)
	client.roundFailed(clock)

	clock = began.Add(outageBudget - time.Second)

	if client.outageBudgetSpent() {
		t.Fatalf("outageBudgetSpent() = true a second before %s of failures", outageBudget)
	}

	clock = began.Add(outageBudget)

	if !client.outageBudgetSpent() {
		t.Fatalf("outageBudgetSpent() = false %s after the first failed round began", outageBudget)
	}

	server, _ := newArchiveServer(t, "test-token", map[string][]byte{"sha256:back": packageArchive(t)})
	client.UpdateAuth([]string{serverHost(t, server)}, "test-token")

	if err := client.fetchArchive(t.Context(), mustPackageRef(t, client, "omega:sha256:back")); err != nil {
		t.Fatalf("fetchArchive() error = %v", err)
	}

	if client.outageBudgetSpent() {
		t.Fatal("outageBudgetSpent() = true after a download that succeeded")
	}
}

func TestAnInstallIsRetriedDuringAnOutageUnlessTheDownloadFailed(t *testing.T) {
	client := newTestClient(t, []string{otherZoneMaster})
	client.OutageBegan(time.Now().Add(-outageBudget))

	if !client.shouldRetryInstall(errors.New("run install script: exit status 1")) {
		t.Fatal("shouldRetryInstall() = false for an install script that failed")
	}

	if client.shouldRetryInstall(&downloadError{err: errors.New("connection refused")}) {
		t.Fatal("shouldRetryInstall() = true for a download that failed after the budget is spent")
	}
}

func TestAPackageEveryEndpointRefusedIsAskedForAgain(t *testing.T) {
	for name, status := range map[string]int{"404": http.StatusNotFound, "403": http.StatusForbidden} {
		t.Run(name, func(t *testing.T) {
			var count atomic.Int32

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				count.Add(1)
				http.Error(w, http.StatusText(status), status)
			}))
			t.Cleanup(server.Close)

			client := newTestClient(t, []string{serverHost(t, server)})
			client.cfg.Retries = 30

			if err := client.installPackage(t.Context(), mustPackageRef(t, client, "refused:sha256:refused")); err == nil {
				t.Fatalf("installPackage() error = nil, want the %d", status)
			}

			if got, want := count.Load(), int32(refusalAttempts*packageInstallAttempts); got != want {
				t.Fatalf("requests = %d, want %d: %d per install attempt", got, want, refusalAttempts)
			}
		})
	}
}

func TestOnlyA404OrA403FromEveryEndpointCountsAsRefused(t *testing.T) {
	notFound := &httpStatusError{statusCode: http.StatusNotFound}
	forbidden := &httpStatusError{statusCode: http.StatusForbidden}

	for name, tt := range map[string]struct {
		err  error
		want bool
	}{
		"404":                 {err: notFound, want: true},
		"404 and 403":         {err: errors.Join(notFound, forbidden), want: true},
		"401":                 {err: &httpStatusError{statusCode: http.StatusUnauthorized}, want: false},
		"404 and no answer":   {err: errors.Join(notFound, errors.New("connection refused")), want: false},
		"invalid digest":      {err: errInvalidDigest, want: false},
		"the caller gave up":  {err: context.Canceled, want: false},
		"no endpoints at all": {err: errNoEndpoints, want: false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := everyEndpointRefused(tt.err); got != tt.want {
				t.Fatalf("everyEndpointRefused() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnUnauthorizedFetchIsNotAskedForAgain(t *testing.T) {
	server, count := newArchiveServer(t, "another-token", nil)
	client := newTestClient(t, []string{serverHost(t, server)})
	client.cfg.Retries = 30

	if err := client.fetchArchive(t.Context(), mustPackageRef(t, client, "denied:sha256:denied")); err == nil {
		t.Fatal("fetchArchive() error = nil, want the 401")
	}

	if got := count.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}
