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

package memberlist

import (
	"errors"
	"fmt"
	"net"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/hashicorp/go-multierror"
	hcml "github.com/hashicorp/memberlist"

	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	seedA    = "10.0.0.1:7946"
	seedB    = "10.0.0.2:7946"
	seedC    = "10.0.0.3:7946"
	seedHost = "fencing-peers.example:7946"
)

type barrierJoin struct {
	t    *testing.T
	want int
	all  chan struct{}

	mu    sync.Mutex
	seeds []string
}

func (f *barrierJoin) join(seed string) (int, error) {
	f.mu.Lock()
	f.seeds = append(f.seeds, seed)
	if len(f.seeds) == f.want {
		close(f.all)
	}
	f.mu.Unlock()

	select {
	case <-f.all:
		return 1, nil
	case <-time.After(time.Minute):
		f.t.Errorf("join through %s waited a minute for %d concurrent joins", seed, f.want)
		return 0, errors.New("barrier not reached")
	}
}

func TestJoinDialsEachSeedInItsOwnConcurrentJoin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		seeds := []string{seedA, seedB, seedC}
		fake := &barrierJoin{t: t, want: len(seeds), all: make(chan struct{})}

		start := time.Now()
		joined, err := joinEach(seeds, fake.join)
		elapsed := time.Since(start)

		if joined != len(seeds) || err != nil {
			t.Errorf("Join = (%d, %v), want (%d, nil)", joined, err, len(seeds))
		}
		if elapsed != 0 {
			t.Errorf("Join took %s of virtual time, want every seed dialled at once", elapsed)
		}

		fake.mu.Lock()
		dialled := slices.Sorted(slices.Values(fake.seeds))
		fake.mu.Unlock()
		if !slices.Equal(dialled, seeds) {
			t.Errorf("seeds dialled one per join = %v, want each of %v exactly once", dialled, seeds)
		}
	})
}

func TestJoinWaitsForTheSlowestSeed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const slow = 5 * time.Second

		join := func(seed string) (int, error) {
			if seed == seedB {
				time.Sleep(slow)
			}
			return 1, nil
		}

		start := time.Now()
		joined, err := joinEach([]string{seedA, seedB}, join)
		elapsed := time.Since(start)

		if joined != 2 || err != nil {
			t.Errorf("Join = (%d, %v), want (2, nil)", joined, err)
		}
		if elapsed != slow {
			t.Errorf("Join returned after %s, want %s: the slowest seed", elapsed, slow)
		}
	})
}

func TestJoinCountsEveryAcceptedSeedAndDropsErrorsOnSuccess(t *testing.T) {
	tests := []struct {
		name       string
		seeds      []string
		failing    []string
		addresses  map[string]int
		wantJoined int
		wantErr    bool
	}{
		{
			name:       "every seed accepts",
			seeds:      []string{seedA, seedB, seedC},
			wantJoined: 3,
		},
		{
			name:       "one seed fails",
			seeds:      []string{seedA, seedB, seedC},
			failing:    []string{seedB},
			wantJoined: 2,
		},
		{
			name:       "a seed that reaches two peers counts both",
			seeds:      []string{seedA, seedHost, seedC},
			failing:    []string{seedC},
			addresses:  map[string]int{seedHost: 2},
			wantJoined: 3,
		},
		{
			name:       "every seed fails",
			seeds:      []string{seedA, seedB, seedC},
			failing:    []string{seedA, seedB, seedC},
			wantJoined: 0,
			wantErr:    true,
		},
		{
			name:       "no seeds",
			seeds:      nil,
			wantJoined: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			join := func(seed string) (int, error) {
				calls.Add(1)
				if slices.Contains(tt.failing, seed) {
					return 0, fmt.Errorf("failed to join %s: connection refused", seed)
				}
				if n, ok := tt.addresses[seed]; ok {
					return n, nil
				}
				return 1, nil
			}

			joined, err := joinEach(tt.seeds, join)

			if joined != tt.wantJoined {
				t.Errorf("Join joined %d, want %d", joined, tt.wantJoined)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("Join error = %T(%v), want error: %t", err, err, tt.wantErr)
			}
			if got := int(calls.Load()); got != len(tt.seeds) {
				t.Errorf("join called %d times, want %d", got, len(tt.seeds))
			}
		})
	}
}

func TestJoinErrorsKeepSeedOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		delays := map[string]time.Duration{
			seedA: 3 * time.Second,
			seedB: 2 * time.Second,
			seedC: 1 * time.Second,
		}

		var mu sync.Mutex
		var finished []string
		join := func(seed string) (int, error) {
			time.Sleep(delays[seed])

			mu.Lock()
			finished = append(finished, seed)
			mu.Unlock()

			return 0, multierror.Append(nil, fmt.Errorf("failed to join %s: i/o timeout", seed))
		}

		joined, err := joinEach([]string{seedA, seedB, seedC}, join)

		mu.Lock()
		order := slices.Clone(finished)
		mu.Unlock()
		if want := []string{seedC, seedB, seedA}; !slices.Equal(order, want) {
			t.Fatalf("joins finished in order %v, want %v", order, want)
		}

		if joined != 0 {
			t.Errorf("Join joined %d, want 0", joined)
		}
		want := "3 errors occurred:\n" +
			"\t* failed to join 10.0.0.1:7946: i/o timeout\n" +
			"\t* failed to join 10.0.0.2:7946: i/o timeout\n" +
			"\t* failed to join 10.0.0.3:7946: i/o timeout\n\n"
		if err == nil {
			t.Fatalf("Join error = nil, want %q", want)
		}
		if got := err.Error(); got != want {
			t.Errorf("Join error = %q, want %q", got, want)
		}
	})
}

func startLoopbackCluster(t *testing.T, name string) *Cluster {
	t.Helper()

	cluster, err := New(Config{
		NodeName:      name,
		NodeGroup:     "worker",
		AdvertiseAddr: "127.0.0.1",
		Port:          0,
		Tuning:        testTuning(),
	}, log.NewNop())
	if err != nil {
		t.Fatalf("create cluster %s: %v", name, err)
	}

	t.Cleanup(func() {
		if err := cluster.Shutdown(); err != nil {
			t.Errorf("shutdown %s: %v", name, err)
		}
	})

	return cluster
}

func TestJoinErrorTextMatchesTheSequentialJoin(t *testing.T) {
	cluster := startLoopbackCluster(t, "worker-1")

	listeners := make([]net.Listener, 0, 3)
	for range 3 {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("reserve a loopback port: %v", err)
		}
		listeners = append(listeners, ln)
	}

	seeds := make([]string, 0, len(listeners))
	for _, ln := range listeners {
		seeds = append(seeds, ln.Addr().String())
		if err := ln.Close(); err != nil {
			t.Fatalf("release %s: %v", ln.Addr(), err)
		}
	}

	seqJoined, seqErr := cluster.list.Join(seeds)
	parJoined, parErr := cluster.Join(seeds)

	if seqJoined != 0 || parJoined != 0 {
		t.Errorf("joined sequential=%d parallel=%d, want 0 through refused ports", seqJoined, parJoined)
	}
	if seqErr == nil || parErr == nil {
		t.Fatalf("errors sequential=%v parallel=%v, want both set", seqErr, parErr)
	}
	if seqErr.Error() != parErr.Error() {
		t.Errorf("parallel Join error text differs from memberlist's own Join:\nparallel:   %q\nsequential: %q",
			parErr.Error(), seqErr.Error())
	}
}

func TestParallelJoinAgainstLivePeersIsRaceClean(t *testing.T) {
	const pollTimeout = 5 * time.Second

	peers := make([]*Cluster, 0, 3)
	seeds := make([]string, 0, 3)
	for _, name := range []string{"peer-1", "peer-2", "peer-3"} {
		peer := startLoopbackCluster(t, name)
		peers = append(peers, peer)
		seeds = append(seeds, peer.list.LocalNode().Address())
	}

	joiner := startLoopbackCluster(t, "joiner")

	joined, err := joiner.Join(seeds)
	if joined != len(seeds) || err != nil {
		t.Fatalf("Join = (%d, %v), want (%d, nil)", joined, err, len(seeds))
	}

	converged := func() bool {
		if joiner.NumMembers() != len(peers)+1 {
			return false
		}
		for _, peer := range peers {
			if !slices.Contains(peer.Members(), "joiner") {
				return false
			}
		}
		return true
	}

	deadline := time.Now().Add(pollTimeout)
	for !converged() {
		if time.Now().After(deadline) {
			views := make([]string, 0, len(peers))
			for _, peer := range peers {
				views = append(views, fmt.Sprintf("%v", peer.Members()))
			}
			t.Fatalf("no convergence within %s: joiner sees %v, peers see %v",
				pollTimeout, joiner.Members(), views)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestBuildConfigKeepsJoinConcurrencySafe(t *testing.T) {
	events := newEventDelegate(log.NewNop())

	cfg := buildConfig(Config{
		NodeName:      "worker-1",
		NodeGroup:     "worker",
		AdvertiseAddr: "10.0.0.1",
		Port:          8500,
		Tuning:        testTuning(),
	}, log.NewNop(), events)

	unset := []struct {
		name string
		set  bool
	}{
		{"Delegate", cfg.Delegate != nil},
		{"Merge", cfg.Merge != nil},
		{"Conflict", cfg.Conflict != nil},
		{"Alive", cfg.Alive != nil},
		{"Ping", cfg.Ping != nil},
		{"Keyring", cfg.Keyring != nil},
		{"SecretKey", len(cfg.SecretKey) > 0},
		{"Transport", cfg.Transport != nil},
	}
	for _, field := range unset {
		if field.set {
			t.Errorf("buildConfig sets %s, want it unset: parallel Join is only proven safe without it", field.name)
		}
	}

	if cfg.Events != hcml.EventDelegate(events) {
		t.Errorf("Events = %T(%v), want the event delegate passed to buildConfig", cfg.Events, cfg.Events)
	}
}
