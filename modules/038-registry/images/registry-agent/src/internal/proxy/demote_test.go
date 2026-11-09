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

package proxy

import (
	"context"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// fakeClock is a clock the test moves by hand, so a cooldown is crossed without waiting it out.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(by)
}

// window measures how long a target stays demoted after one failure from the clock's present.
func window(t *testing.T, d *demotions, clock *fakeClock, target *Target) time.Duration {
	t.Helper()

	start := clock.Now()
	d.fail(target, clock.Now())
	for step := time.Duration(0); step <= MaxDemotionCooldown+time.Second; step += time.Second {
		if !d.demoted(target) {
			return clock.Now().Sub(start)
		}
		clock.advance(time.Second)
	}
	t.Fatalf("%s stayed demoted past the maximum", target.Host)
	return 0
}

// TestDemotionWindowsDoubleUpToTheMaximum: a target that keeps failing each time it is tried
// again is passed over for longer each time, up to a bound, so that a recovered leader is never
// ignored for longer than that.
func TestDemotionWindowsDoubleUpToTheMaximum(t *testing.T) {
	clock := newFakeClock()
	d := &demotions{now: clock.Now}
	leader := &Target{Name: "Storage", Scheme: registryv1alpha1.SchemeHTTPS, Host: "10.0.0.5:5001"}

	var windows []time.Duration
	for range 6 {
		windows = append(windows, window(t, d, clock, leader))
	}

	assert.Equal(t, []time.Duration{
		30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute,
	}, windows)
}

// TestDemotionIsNotLengthenedByFailuresInsideTheWindow: every request in flight to a dead leader
// fails at about the same moment. Those are one outage seen many times, not many outages, and
// must not send the target straight to the maximum.
func TestDemotionIsNotLengthenedByFailuresInsideTheWindow(t *testing.T) {
	clock := newFakeClock()
	d := &demotions{now: clock.Now}
	leader := &Target{Scheme: registryv1alpha1.SchemeHTTPS, Host: "10.0.0.5:5001"}

	for range 20 {
		d.fail(leader, clock.Now())
	}
	clock.advance(10 * time.Second)
	d.fail(leader, clock.Now())

	clock.advance(DemotionCooldown - 10*time.Second)
	assert.False(t, d.demoted(leader), "the window is the first failure's, untouched by the rest")
}

// TestDemotionIsNotLengthenedByAttemptsThatBeganBeforeItClosed: a registry that keeps its
// connections up and never answers releases the requests sent to it one by one over the response
// timeout, which outlasts the shortest window. Those that come back after the window closed began
// before it did; they say the target is still down, not that it was tried again and failed again.
func TestDemotionIsNotLengthenedByAttemptsThatBeganBeforeItClosed(t *testing.T) {
	clock := newFakeClock()
	d := &demotions{now: clock.Now}
	leader := &Target{Scheme: registryv1alpha1.SchemeHTTPS, Host: "10.0.0.5:5001"}

	began := clock.Now()
	d.fail(leader, began)

	clock.advance(40 * time.Second)
	d.fail(leader, began.Add(10*time.Second))
	require.True(t, d.demoted(leader), "still failing, so passed over again")

	clock.advance(DemotionCooldown - time.Second)
	require.True(t, d.demoted(leader))
	clock.advance(time.Second)
	assert.False(t, d.demoted(leader), "for the same window as before, not twice as long")

	// The first attempt that began after that is a repeat, and only it doubles the window.
	assert.Equal(t, 2*DemotionCooldown, window(t, d, clock, leader))
}

// TestDemotionIsClearedByAnAnswer: a target that answered is back in its place, and its next
// failure starts from the shortest window, not from where the last outage left off.
func TestDemotionIsClearedByAnAnswer(t *testing.T) {
	clock := newFakeClock()
	d := &demotions{now: clock.Now}
	leader := &Target{Scheme: registryv1alpha1.SchemeHTTPS, Host: "10.0.0.5:5001"}

	window(t, d, clock, leader)
	window(t, d, clock, leader)
	d.fail(leader, clock.Now())
	require.True(t, d.demoted(leader))

	d.succeed(leader)
	assert.False(t, d.demoted(leader))
	assert.Equal(t, DemotionCooldown, window(t, d, clock, leader))
}

// TestDemotionForgetsAnOldOutage: a failure long after the last window closed is a new outage
// rather than a repeat of the old one.
func TestDemotionForgetsAnOldOutage(t *testing.T) {
	clock := newFakeClock()
	d := &demotions{now: clock.Now}
	leader := &Target{Scheme: registryv1alpha1.SchemeHTTPS, Host: "10.0.0.5:5001"}

	window(t, d, clock, leader)
	clock.advance(MaxDemotionCooldown + time.Minute)

	assert.Equal(t, DemotionCooldown, window(t, d, clock, leader))
}

// TestDemotionOrderIsByAddressAndStable: a demoted target goes to the end and the rest keep
// the order the layout gave them. The key is the address, so two targets that share a name —
// the replicas are all "Storage", the mirrors of a route all carry its match — are judged
// apart, while one host written in two cases is one target.
func TestDemotionOrderIsByAddressAndStable(t *testing.T) {
	clock := newFakeClock()
	d := &demotions{now: clock.Now}

	targets := []Target{
		{Name: "Storage", Scheme: registryv1alpha1.SchemeHTTPS, Host: "10.0.0.5:5001"},
		{Name: "Storage", Scheme: registryv1alpha1.SchemeHTTPS, Host: "10.0.0.6:5001"},
		{Name: "Upstream", Scheme: registryv1alpha1.SchemeHTTPS, Host: "registry.example.com"},
		{Name: "Upstream", Scheme: registryv1alpha1.SchemeHTTP, Host: "registry.example.com"},
	}

	d.fail(&Target{Scheme: registryv1alpha1.SchemeHTTPS, Host: "10.0.0.5:5001"}, clock.Now())
	d.fail(&Target{Scheme: registryv1alpha1.SchemeHTTPS, Host: "REGISTRY.example.com"}, clock.Now())

	promoted := d.order(targets)

	hosts := make([]string, 0, len(targets))
	for _, target := range targets {
		hosts = append(hosts, target.Scheme.Lower()+"://"+target.Host)
	}
	assert.Equal(t, []string{
		"https://10.0.0.6:5001",
		"http://registry.example.com",
		"https://10.0.0.5:5001",
		"https://registry.example.com",
	}, hosts)
	assert.Equal(t, []bool{true, true, false, false}, promoted,
		"the two that now come before a target the layout put ahead of them")

	// Once the windows close nothing is moved. Every request resolves a list of its own, so the
	// next one is in the layout's order again.
	clock.advance(DemotionCooldown)
	fresh := []Target{targets[2], targets[0], targets[3], targets[1]}
	promoted = d.order(fresh)
	assert.Equal(t, []Target{targets[2], targets[0], targets[3], targets[1]}, fresh)
	assert.Equal(t, []bool{false, false, false, false}, promoted)
}

// blackHole accepts connections and never answers on them: a leader whose node went silent
// rather than one that refuses. It returns the address and how many connections it took.
func blackHole(t *testing.T) (string, *atomic.Int32) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	accepted := new(atomic.Int32)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			go func() {
				// Read whatever arrives and say nothing, until the client hangs up.
				_, _ = io.Copy(io.Discard, conn)
				_ = conn.Close()
			}()
		}
	}()
	return listener.Addr().String(), accepted
}

// TestServeStopsBeginningWithASilentLeader is the defect this exists for: a leader whose node
// stopped answering held every pull on the node for the full response timeout, one after another,
// until the layout moved on. Once it has failed, the pulls that follow go to the upstream
// directly, and it is asked first again when its window closes.
func TestServeStopsBeginningWithASilentLeader(t *testing.T) {
	leader, dialled := blackHole(t)
	upstream := (&registryStub{name: "upstream", body: "from-the-upstream"}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Cache: true,
		Backends: []registryv1alpha1.Backend{
			{
				Name: registryv1alpha1.BackendStorage,
				Endpoint: registryv1alpha1.Endpoint{
					Scheme: registryv1alpha1.SchemeHTTP, Host: leader, Path: constant.Path,
				},
			},
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
		},
	}

	clock := newFakeClock()
	server := newServer(spec)
	server.ResponseTimeout = 200 * time.Millisecond
	server.demoted.now = clock.Now

	response := pull(t, server, constant.Host, "/v2/system/deckhouse/one/manifests/v1")
	assert.Equal(t, "from-the-upstream", bodyOf(t, response))
	require.Equal(t, int32(1), dialled.Load(), "the first pull still has to find out")

	for range 3 {
		response = pull(t, server, constant.Host, "/v2/system/deckhouse/one/manifests/v1")
		assert.Equal(t, "from-the-upstream", bodyOf(t, response))
	}
	assert.Equal(t, int32(1), dialled.Load(), "the pulls that follow do not wait for it again")
	assert.Len(t, upstream.asked(), 4)

	clock.advance(DemotionCooldown)

	response = pull(t, server, constant.Host, "/v2/system/deckhouse/one/manifests/v1")
	assert.Equal(t, "from-the-upstream", bodyOf(t, response))
	assert.Equal(t, int32(2), dialled.Load(), "once the window closes, it is asked first again")
}

// TestServeKeepsADemotedTargetAsTheLastResort: demotion reorders and never removes. When
// everything ahead of a demoted target fails too — which in an air-gapped cluster is always,
// since nothing is ahead of the leader — it is still the one that might answer.
func TestServeKeepsADemotedTargetAsTheLastResort(t *testing.T) {
	leader := (&registryStub{name: "leader", body: "from-the-leader"}).start(t)
	upstream := (&registryStub{name: "upstream", status: http.StatusBadGateway}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Cache: true,
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendStorage, leader, t, constant.Path, nil),
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
		},
	}

	clock := newFakeClock()
	server := newServer(spec)
	server.demoted.now = clock.Now
	server.demoted.fail(&Target{Scheme: registryv1alpha1.SchemeHTTP, Host: leader.host(t)}, clock.Now())

	response := pull(t, server, constant.Host, "/v2/system/deckhouse/one/manifests/v1")

	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "from-the-leader", bodyOf(t, response))
	assert.Len(t, upstream.asked(), 1, "the upstream went first while the leader was demoted")
	assert.False(t, server.demoted.demoted(&Target{Scheme: registryv1alpha1.SchemeHTTP, Host: leader.host(t)}),
		"and the leader's answer cleared it")
	assert.True(t, server.demoted.demoted(&Target{Scheme: registryv1alpha1.SchemeHTTP, Host: upstream.host(t)}),
		"a server error with another target behind it is a failure like any other")
}

// TestServeAsksADemotedLeaderForWhatTheUpstreamLacks: the upstream's 404 does not speak for the
// cache the way the cache's speaks for the upstream. While the leader is demoted the upstream is
// asked first, and an image it does not have — gone from it, or never in it — is still asked of
// the leader, as it would have been without demotion. The 404 is the answer only when the leader
// has no better one.
func TestServeAsksADemotedLeaderForWhatTheUpstreamLacks(t *testing.T) {
	tests := []struct {
		name   string
		leader *registryStub
		// gone replaces the leader with an address that refuses connections.
		gone       bool
		wantStatus int
		wantBody   string
		wantDigest string
	}{
		{
			name:       "the leader has it",
			leader:     &registryStub{name: "leader", body: "from-the-leader"},
			wantStatus: http.StatusOK,
			wantBody:   "from-the-leader",
			wantDigest: "sha256:leader",
		},
		{
			name:       "the leader does not have it either",
			leader:     &registryStub{name: "leader", status: http.StatusNotFound},
			wantStatus: http.StatusNotFound,
			wantDigest: "sha256:leader",
		},
		{
			name:       "the leader fails with a server error",
			leader:     &registryStub{name: "leader", status: http.StatusServiceUnavailable},
			wantStatus: http.StatusNotFound,
			wantDigest: "sha256:upstream",
		},
		{
			name:       "the leader cannot be reached",
			gone:       true,
			wantStatus: http.StatusNotFound,
			wantDigest: "sha256:upstream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := (&registryStub{name: "upstream", status: http.StatusNotFound}).start(t)

			var leaderHost string
			if tt.gone {
				// A port that was open a moment ago and is not any more: refused at once.
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				leaderHost = listener.Addr().String()
				require.NoError(t, listener.Close())
			} else {
				leaderHost = tt.leader.start(t).host(t)
			}

			spec := &registryv1alpha1.RegistryNodeSpec{
				Cache: true,
				Backends: []registryv1alpha1.Backend{
					{
						Name: registryv1alpha1.BackendStorage,
						Endpoint: registryv1alpha1.Endpoint{
							Scheme: registryv1alpha1.SchemeHTTP, Host: leaderHost, Path: constant.Path,
						},
					},
					insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
				},
			}

			clock := newFakeClock()
			server := newServer(spec)
			server.demoted.now = clock.Now
			server.demoted.fail(&Target{Scheme: registryv1alpha1.SchemeHTTP, Host: leaderHost}, clock.Now())

			response := pull(t, server, constant.Host, "/v2/system/deckhouse/one/manifests/v1")

			assert.Equal(t, tt.wantStatus, response.StatusCode)
			assert.Equal(t, tt.wantDigest, response.Header.Get("Docker-Content-Digest"),
				"whose answer reached the client")
			assert.Equal(t, tt.wantBody, bodyOf(t, response))
			assert.Len(t, upstream.asked(), 1, "the upstream went first")
			if !tt.gone {
				assert.Len(t, tt.leader.asked(), 1, "and the leader was asked after its 404")
			}
		})
	}

	// The other way round nothing changes: a 404 from the leader, asked first because the
	// upstream is the one demoted, is the cache's answer for both and is relayed as it is.
	t.Run("the upstream is the one demoted", func(t *testing.T) {
		leader := (&registryStub{name: "leader", status: http.StatusNotFound}).start(t)
		upstream := (&registryStub{name: "upstream", body: "from-the-upstream"}).start(t)

		spec := &registryv1alpha1.RegistryNodeSpec{
			Cache: true,
			Backends: []registryv1alpha1.Backend{
				insecureBackend(registryv1alpha1.BackendStorage, leader, t, constant.Path, nil),
				insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
			},
		}

		clock := newFakeClock()
		server := newServer(spec)
		server.demoted.now = clock.Now
		server.demoted.fail(&Target{Scheme: registryv1alpha1.SchemeHTTP, Host: upstream.host(t)}, clock.Now())

		response := pull(t, server, constant.Host, "/v2/system/deckhouse/one/manifests/v1")

		assert.Equal(t, http.StatusNotFound, response.StatusCode)
		assert.Empty(t, upstream.asked())
	})
}

// TestServeDemotesOnlyForFailures draws the line between a registry that did not work and one
// that answered something unwelcome. Only the first is a reason to send the next request
// elsewhere; the second is an answer, and the next target would be asked the same question.
func TestServeDemotesOnlyForFailures(t *testing.T) {
	refusing := &registryv1alpha1.Auth{Username: "registry-ro", Password: "dh-test-secret-0123456789abcdef"}

	tests := []struct {
		name    string
		storage *registryStub
		auth    *registryv1alpha1.Auth
		demoted bool
	}{
		{
			name:    "a missing image is an answer",
			storage: &registryStub{name: "storage", status: http.StatusNotFound},
		},
		{
			name:    "credentials refused by the registry",
			storage: &registryStub{name: "storage", challenge: "basic", username: "someone-else", password: "x"},
			auth:    refusing,
		},
		{
			name:    "credentials refused by its token service",
			storage: &registryStub{name: "storage", challenge: "bearer", username: "someone-else", password: "x"},
			auth:    refusing,
		},
		{
			name:    "a server error with another target behind it",
			storage: &registryStub{name: "storage", status: http.StatusServiceUnavailable},
			demoted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := tt.storage.start(t)
			upstream := (&registryStub{name: "upstream", body: "from-the-upstream"}).start(t)

			spec := &registryv1alpha1.RegistryNodeSpec{
				Cache: true,
				Backends: []registryv1alpha1.Backend{
					insecureBackend(registryv1alpha1.BackendStorage, storage, t, constant.Path, tt.auth),
					insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
				},
			}

			server := newServer(spec)
			server.demoted.now = newFakeClock().Now
			_ = bodyOf(t, pull(t, server, constant.Host, "/v2/system/deckhouse/one/manifests/v1"))

			assert.Equal(t, tt.demoted,
				server.demoted.demoted(&Target{Scheme: registryv1alpha1.SchemeHTTP, Host: storage.host(t)}))
		})
	}

	t.Run("a server error from the last target", func(t *testing.T) {
		storage := (&registryStub{name: "storage", status: http.StatusServiceUnavailable}).start(t)
		upstream := (&registryStub{name: "upstream", status: http.StatusServiceUnavailable}).start(t)
		upstreamTarget := &Target{Scheme: registryv1alpha1.SchemeHTTP, Host: upstream.host(t)}

		spec := &registryv1alpha1.RegistryNodeSpec{
			Backends: []registryv1alpha1.Backend{
				insecureBackend(registryv1alpha1.BackendStorage, storage, t, constant.Path, nil),
				insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
			},
		}

		clock := newFakeClock()
		server := newServer(spec)
		server.demoted.now = clock.Now

		// Demoted already, so that it is the last one asked, and the last one answering 503.
		server.demoted.fail(upstreamTarget, clock.Now())

		response := pull(t, server, constant.Host, "/v2/system/deckhouse/one/manifests/v1")
		assert.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
		assert.Len(t, storage.asked(), 1)
		assert.Len(t, upstream.asked(), 1)

		assert.True(t, server.demoted.demoted(upstreamTarget),
			"relayed because nothing was left to try, which clears nothing")
	})

	t.Run("a single target that cannot be reached", func(t *testing.T) {
		// A port that was open a moment ago and is not any more: refused at once.
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		gone := listener.Addr().String()
		require.NoError(t, listener.Close())

		server := newServer(&registryv1alpha1.RegistryNodeSpec{})
		server.demoted.now = newFakeClock().Now

		response := pull(t, server, gone, "/v2/one/manifests/v1")
		assert.Equal(t, http.StatusBadGateway, response.StatusCode)

		assert.Empty(t, server.demoted.entries,
			"nothing to prefer instead, so nothing is kept about a registry some pod named")
	})
}

// TestServeDoesNotDemoteForAClientThatGaveUp: containerd abandons pulls for reasons of its own,
// and an attempt cut short by that says nothing about the registry. Held against it, every
// cancelled pull would send the next ones past a leader that is perfectly well.
func TestServeDoesNotDemoteForAClientThatGaveUp(t *testing.T) {
	storage := (&registryStub{name: "storage", body: "from-the-cache"}).start(t)
	upstream := (&registryStub{name: "upstream", body: "from-the-upstream"}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		Cache: true,
		Backends: []registryv1alpha1.Backend{
			insecureBackend(registryv1alpha1.BackendStorage, storage, t, constant.Path, nil),
			insecureBackend(registryv1alpha1.BackendUpstream, upstream, t, "/deckhouse/ee", nil),
		},
	}

	server := newServer(spec)
	server.demoted.now = newFakeClock().Now

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequestWithContext(ctx, http.MethodGet,
		"/v2/system/deckhouse/one/manifests/v1?ns="+url.QueryEscape(constant.Host), nil)
	server.Handler().ServeHTTP(httptest.NewRecorder(), request)

	assert.False(t, server.demoted.demoted(&Target{Scheme: registryv1alpha1.SchemeHTTP, Host: storage.host(t)}))
	assert.False(t, server.demoted.demoted(&Target{Scheme: registryv1alpha1.SchemeHTTP, Host: upstream.host(t)}))
}

// TestServeDemotesByAddressNotByName: every target of a route carries the route's match as its
// name. Judged by name, one broken mirror would take the working ones down with it.
func TestServeDemotesByAddressNotByName(t *testing.T) {
	broken := (&registryStub{name: "broken", status: http.StatusInternalServerError}).start(t)
	working := (&registryStub{name: "working", body: "from-the-mirror"}).start(t)

	spec := &registryv1alpha1.RegistryNodeSpec{
		AdditionalRoutes: []registryv1alpha1.Route{{
			Match: "images.example.com",
			Endpoint: registryv1alpha1.Endpoint{
				Scheme: registryv1alpha1.SchemeHTTP, Host: broken.host(t),
			},
			Mirrors: []registryv1alpha1.Endpoint{{
				Scheme: registryv1alpha1.SchemeHTTP, Host: working.host(t),
			}},
		}},
	}

	server := newServer(spec)
	server.demoted.now = newFakeClock().Now

	for range 3 {
		response := pull(t, server, "images.example.com", "/v2/one/manifests/v1")
		assert.Equal(t, "from-the-mirror", bodyOf(t, response))
	}

	assert.Len(t, broken.asked(), 1, "asked once, then passed over")
	assert.Len(t, working.asked(), 3)
	assert.Equal(t, []string{"images.example.com"}, server.Usable([]string{"images.example.com"}),
		"the route is usable: the address that answered last is one of its own")
}

// TestServeSpeaksHTTP2ToATargetThatOffersIt: the health pings are an HTTP/2 mechanism, and they
// are only worth anything if the connection to the store is HTTP/2 in the first place. The trust
// the agent configures for a target replaces the transport's TLS settings, which is exactly where
// HTTP/2 tends to be lost without anything failing.
func TestServeSpeaksHTTP2ToATargetThatOffersIt(t *testing.T) {
	store := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	store.EnableHTTP2 = true
	store.StartTLS()
	t.Cleanup(store.Close)

	parsed, err := url.Parse(store.URL)
	require.NoError(t, err)

	authority := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: store.Certificate().Raw})
	target := &Target{
		Name:   string(registryv1alpha1.BackendStorage),
		Scheme: registryv1alpha1.SchemeHTTPS,
		Host:   parsed.Host,
		Path:   "/v2/system/deckhouse/one/manifests/v1",
		CA:     string(authority),
	}

	server := newServer(nil)
	original := httptest.NewRequest(http.MethodGet, "/v2/system/deckhouse/one/manifests/v1", nil)
	response, err := server.attempt(context.Background(), original, target)
	require.NoError(t, err)
	t.Cleanup(func() { _ = response.Body.Close() })

	assert.Equal(t, 2, response.ProtoMajor)

	client, err := server.client(target.CA)
	require.NoError(t, err)
	transport := client.Transport.(*http.Transport)
	require.NotNil(t, transport.HTTP2)
	assert.Equal(t, h2PingAfter, transport.HTTP2.SendPingTimeout)
	assert.Equal(t, h2PingTimeout, transport.HTTP2.PingTimeout)
}

func TestServerDialTimeoutDefaults(t *testing.T) {
	server := newServer(nil)
	assert.Equal(t, DefaultDialTimeout, server.dialTimeout())

	server.DialTimeout = time.Second
	assert.Equal(t, time.Second, server.dialTimeout())
}
