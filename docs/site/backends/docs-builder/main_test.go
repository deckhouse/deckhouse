// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

func TestCheckFlags(t *testing.T) {
	set := func(cache, registries, local, s3, s3Size, s3Prefix string) {
		onDemandCache, onDemandRegistries, onDemandLocalDir = cache, registries, local
		onDemandS3, onDemandS3Size, onDemandS3Prefix = s3, s3Size, s3Prefix
	}
	t.Cleanup(func() { set("", "", "", "", "", "") })

	for name, tc := range map[string]struct {
		flags [6]string
		ok    bool
	}{
		"a cluster":                 {[6]string{"/cache"}, true},
		"a cluster with S3":         {[6]string{"/cache", "", "", "/s3", "5Gi", "docs/"}, true},
		"the public site":           {[6]string{"/cache", "reg.example/modules", "", "/s3", "5Gi"}, true},
		"local modules":             {[6]string{"/cache", "", "/modules"}, true},
		"no cache":                  {[6]string{""}, false},
		"registries and local":      {[6]string{"/cache", "reg.example/modules", "/modules"}, false},
		"the S3 size without S3":    {[6]string{"/cache", "", "", "", "5Gi"}, false},
		"the S3 prefix without S3":  {[6]string{"/cache", "", "", "", "", "docs/"}, false},
		"local with the S3 prefix":  {[6]string{"/cache", "", "/modules", "", "", "docs/"}, false},
		"registries with S3 prefix": {[6]string{"/cache", "reg.example/modules", "", "", "", "docs/"}, false},
	} {
		f := tc.flags
		set(f[0], f[1], f[2], f[3], f[4], f[5])
		if err := checkFlags(); (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok = %v", name, err, tc.ok)
		}
	}
}

// The process exits only after the background work has stopped (the leader
// Lease is released there), but does not wait for it forever.
func TestWaitStopped(t *testing.T) {
	var eg errgroup.Group
	release := make(chan struct{})
	var stopped atomic.Bool
	eg.Go(func() error {
		<-release
		stopped.Store(true)
		return errors.New("failed")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := waitStopped(ctx, &eg); err == nil || stopped.Load() {
		t.Fatalf("err = %v, stopped = %v; want an error while the work runs", err, stopped.Load())
	}

	time.AfterFunc(20*time.Millisecond, func() { close(release) })
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := waitStopped(ctx, &eg); err == nil || err.Error() != "failed" || !stopped.Load() {
		t.Fatalf("err = %v, stopped = %v; want the error of the work once it stopped", err, stopped.Load())
	}
}

// A template that can not be hashed fails the start: without a revision the
// replicas would share S3 with nothing in it.
func TestNewColdBucket(t *testing.T) {
	ms := metricsstorage.NewMetricStorage()
	template := t.TempDir()
	if err := os.WriteFile(filepath.Join(template, "hugo.yaml"), []byte("title: docs\n"), 0600); err != nil {
		t.Fatal(err)
	}

	bucket, quota, revision, err := newColdBucket(t.TempDir(), "1Ki", "docs/", template, log.NewNop(), ms)
	if err != nil || bucket == nil || quota != 1024 || revision == "" {
		t.Fatalf("bucket %v, quota %d, revision %q, err %v", bucket, quota, revision, err)
	}

	for name, args := range map[string][3]string{
		"no template":  {"1Ki", "docs/", filepath.Join(template, "missing")},
		"no size":      {"", "docs/", template},
		"a bad prefix": {"1Ki", "docs", template},
	} {
		if _, _, _, err := newColdBucket(t.TempDir(), args[0], args[1], args[2], log.NewNop(), ms); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// No bucket is no cold cache for the store, not a cold cache that is nil
// inside.
func TestColdCacheWithoutBucket(t *testing.T) {
	if cold := coldCache(nil); cold != nil {
		t.Fatalf("cold cache %#v without a bucket", cold)
	}
}

// A term of the leader: the follower of the replica stands by and the gauge
// is up while it lasts, the pod is ready once the pinned versions are
// restored, and both are down again when the Lease is gone.
func TestLeaderTerm(t *testing.T) {
	var mu sync.Mutex
	var gauge []float64
	var ready atomic.Bool
	var running sync.WaitGroup
	running.Add(3)
	work := func(ctx context.Context) {
		running.Done()
		<-ctx.Done()
	}
	term := leaderTerm{
		setLeader: func(v float64) {
			mu.Lock()
			defer mu.Unlock()
			gauge = append(gauge, v)
		},
		pin: func(ctx context.Context, onRestored func()) {
			onRestored()
			work(ctx)
		},
		evictCold: work,
		rerender:  work,
		markReady: func() { ready.Store(true) },
	}

	ctx, loseLease := context.WithCancel(context.Background())
	defer loseLease()
	done := make(chan struct{})
	go func() {
		term.run(ctx)
		close(done)
	}()

	running.Wait()
	mu.Lock()
	during := append([]float64(nil), gauge...)
	mu.Unlock()
	if !term.leading.Load() || len(during) != 1 || during[0] != 1 || !ready.Load() {
		t.Fatalf("leading %v, gauge %v, ready %v during the term", term.leading.Load(), during, ready.Load())
	}
	select {
	case <-done:
		t.Fatal("the term ended while the Lease is held")
	default:
	}

	loseLease()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the term did not end with the Lease")
	}
	if term.leading.Load() || len(gauge) != 2 || gauge[1] != 0 {
		t.Fatalf("leading %v, gauge %v after the term", term.leading.Load(), gauge)
	}
}
