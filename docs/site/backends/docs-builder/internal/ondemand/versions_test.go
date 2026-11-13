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

package ondemand

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/pkg/log"

	"github.com/flant/docs-builder/internal/registry"
)

func TestVersionsList(t *testing.T) {
	var calls atomic.Int32
	tags := []string{"stable", "alpha", "v1.2.0", "v1.10.0", "vgpu-v100-demo", "v1.10.0", "v1.11.0-rc.1", "v1.9", "latest", "v1.2.0+build"}
	v := NewVersions(func(context.Context, string) ([]string, error) {
		if calls.Add(1) > 1 {
			return []string{"v1.12.0"}, nil
		}
		return tags, nil
	}, time.Minute, time.Minute, nil)

	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	want := []string{"v1.11.0-rc.1", "v1.10.0", "v1.2.0"}
	for range 2 {
		got, err := v.List(context.Background(), "echo")
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("versions = %v, want %v", got, want)
		}
	}
	if n := calls.Load(); n != 1 {
		t.Fatalf("registry listed %d times within the TTL, want 1", n)
	}

	// After the TTL the reader gets the cached list at once, and the
	// registry is listed in the background.
	c.add(time.Minute)
	got, err := v.List(context.Background(), "echo")
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("versions after the TTL = %v, %v; want the cached %v", got, err, want)
	}
	waitFor(t, func() bool {
		got, _ := v.List(context.Background(), "echo")
		return slices.Equal(got, []string{"v1.12.0"})
	})
	if n := calls.Load(); n != 2 {
		t.Fatalf("registry listed %d times after the TTL, want 2", n)
	}
}

func TestVersionsListKeepsTheListOnError(t *testing.T) {
	var calls atomic.Int32
	v := NewVersions(func(context.Context, string) ([]string, error) {
		if calls.Add(1) > 1 {
			return nil, errors.New("registry is down")
		}
		return []string{"v1.0.0"}, nil
	}, 5*time.Minute, time.Minute, nil)
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	if _, err := v.List(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	c.add(5 * time.Minute)
	v.List(context.Background(), "echo") //nolint:errcheck // starts the listing that fails
	waitFor(t, func() bool { return calls.Load() == 2 })

	// The failure keeps the list and waits failedTTL.
	for range 3 {
		got, err := v.List(context.Background(), "echo")
		if err != nil || !slices.Equal(got, []string{"v1.0.0"}) {
			t.Fatalf("versions = %v, %v; want the last list", got, err)
		}
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("registry listed %d times within the failure TTL, want 2", n)
	}
}

// waitFor polls cond until it holds, for a background listing.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in 5 s")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestVersionsListError(t *testing.T) {
	calls := 0
	v := NewVersions(func(context.Context, string) ([]string, error) {
		calls++
		return nil, errors.New("registry is down")
	}, 5*time.Minute, time.Minute, nil)

	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	for range 3 {
		if _, err := v.List(context.Background(), "echo"); err == nil {
			t.Fatal("want an error")
		}
	}
	if calls != 1 {
		t.Fatalf("registry listed %d times within the failure TTL, want 1", calls)
	}

	c.add(time.Minute)
	if _, err := v.List(context.Background(), "echo"); err == nil {
		t.Fatal("want an error")
	}
	if calls != 2 {
		t.Fatalf("registry listed %d times after the failure TTL, want 2", calls)
	}
}

func TestVersionsListIgnoresCanceledRequest(t *testing.T) {
	release := make(chan struct{})
	calls := atomic.Int32{}
	v := NewVersions(func(ctx context.Context, _ string) ([]string, error) {
		calls.Add(1)
		select {
		case <-release:
			return []string{"v1.0.0"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, time.Minute, time.Minute, nil)

	// The reader who triggered the listing closes the page.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() {
		_, err := v.List(ctx, "echo")
		done <- err
	}()
	for calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want canceled", err)
	}

	// The listing goes on and serves the next reader without a new request.
	close(release)
	got, err := v.List(context.Background(), "echo")
	if err != nil || !slices.Equal(got, []string{"v1.0.0"}) || calls.Load() != 1 {
		t.Fatalf("versions %v, err %v, listings %d; want [v1.0.0], nil, 1", got, err, calls.Load())
	}
}

func TestVersionsListSharesConcurrentMisses(t *testing.T) {
	release := make(chan struct{})
	calls := atomic.Int32{}
	v := NewVersions(func(context.Context, string) ([]string, error) {
		calls.Add(1)
		<-release
		return []string{"v1.0.0"}, nil
	}, time.Minute, time.Minute, nil)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := v.List(context.Background(), "echo"); err != nil {
				t.Error(err)
			}
		}()
	}
	for calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(10 * time.Millisecond)
	close(release)
	wg.Wait()

	if n := calls.Load(); n != 1 {
		t.Fatalf("listings = %d, want 1", n)
	}
}

func TestVersionsListDropsExpiredEntries(t *testing.T) {
	v := NewVersions(func(_ context.Context, module string) ([]string, error) {
		return nil, errors.New("no module " + module)
	}, 5*time.Minute, time.Minute, nil)
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	for _, m := range []string{"a", "b", "c"} {
		v.List(context.Background(), m) //nolint:errcheck // the failure is what is cached
	}
	c.add(time.Minute)
	v.List(context.Background(), "d") //nolint:errcheck

	if len(v.cache) != 1 {
		t.Fatalf("cache entries = %d, want only the fresh one", len(v.cache))
	}
}

func TestVersionsHasListsAgainForANewVersion(t *testing.T) {
	var calls atomic.Int32
	v := NewVersions(func(context.Context, string) ([]string, error) {
		if calls.Add(1) > 1 {
			return []string{"v1.0.0", "v1.1.0"}, nil
		}
		return []string{"v1.0.0"}, nil
	}, 30*time.Minute, time.Minute, nil)
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	has := func(version string) bool {
		t.Helper()
		ok, err := v.Has(context.Background(), "echo", version)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}

	if !has("v1.0.0") {
		t.Fatal("v1.0.0 is published")
	}
	// A list younger than minRelist is trusted: a crawler does not list the
	// registry on every made-up version.
	c.add(30 * time.Second)
	if has("v1.1.0") || calls.Load() != 1 {
		t.Fatalf("a young list was listed again: %d listings", calls.Load())
	}
	// An older one is listed again, and the version published since opens.
	c.add(31 * time.Second)
	if !has("v1.1.0") || calls.Load() != 2 {
		t.Fatalf("a version published since the listing is not found: %d listings", calls.Load())
	}
	if has("v9.9.9") || calls.Load() != 2 {
		t.Fatalf("a made-up version listed the registry again: %d listings", calls.Load())
	}
}

func TestVersionsListRetriesAfterTheFailureTTL(t *testing.T) {
	var calls atomic.Int32
	v := NewVersions(func(context.Context, string) ([]string, error) {
		switch calls.Add(1) {
		case 1:
			return []string{"v1.0.0"}, nil
		case 2:
			return nil, errors.New("registry is down")
		}
		return []string{"v1.1.0", "v1.0.0"}, nil
	}, 5*time.Minute, time.Minute, nil)
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	v.List(context.Background(), "echo") //nolint:errcheck // the first list
	c.add(5 * time.Minute)
	v.List(context.Background(), "echo") //nolint:errcheck // starts the listing that fails
	waitFor(t, func() bool { cv, _ := v.cached("echo"); return cv.listed.Equal(c.now()) })
	// Not again within failedTTL.
	c.add(30 * time.Second)
	v.List(context.Background(), "echo") //nolint:errcheck
	if n := calls.Load(); n != 2 {
		t.Fatalf("listings within the failure TTL = %d, want 2", n)
	}
	c.add(30 * time.Second)
	v.List(context.Background(), "echo") //nolint:errcheck // the last list, and the registry again
	waitFor(t, func() bool {
		got, _ := v.List(context.Background(), "echo")
		return slices.Equal(got, []string{"v1.1.0", "v1.0.0"})
	})
}

func TestVersionsListDropsAModuleGoneFromTheCluster(t *testing.T) {
	var calls atomic.Int32
	v := NewVersions(func(context.Context, string) ([]string, error) {
		if calls.Add(1) > 1 {
			return nil, fmt.Errorf("module echo: %w", registry.ErrNoSource)
		}
		return []string{"v1.0.0"}, nil
	}, 5*time.Minute, time.Minute, nil)
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	v.List(context.Background(), "echo") //nolint:errcheck // the first list
	c.add(5 * time.Minute)
	v.List(context.Background(), "echo") //nolint:errcheck // starts the listing that finds no source
	waitFor(t, func() bool {
		_, err := v.List(context.Background(), "echo")
		return Unpublished(err)
	})
}

func TestVersionsHasWaitsLongerForEveryRelistingWithNothingNew(t *testing.T) {
	var calls atomic.Int32
	v := NewVersions(func(context.Context, string) ([]string, error) {
		calls.Add(1)
		return []string{"v1.0.0"}, nil
	}, 30*time.Minute, time.Minute, nil)
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	// A crawler asks for a made-up version every 61 seconds.
	for range 8 {
		c.add(61 * time.Second)
		if ok, err := v.Has(context.Background(), "echo", "v9.9.9"); ok || err != nil {
			t.Fatalf("made-up version: %v, %v", ok, err)
		}
	}
	// Listings: the first one, then after 1, 2 and 4 minutes, not every minute.
	if n := calls.Load(); n != 4 {
		t.Fatalf("listings in 8 minutes = %d, want 4", n)
	}
}

func TestVersionsHasGivesUpWithTheReader(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	var calls atomic.Int32
	v := NewVersions(func(context.Context, string) ([]string, error) {
		if calls.Add(1) > 1 {
			<-release
		}
		return []string{"v1.0.0"}, nil
	}, 30*time.Minute, time.Minute, nil)
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	v.List(context.Background(), "echo") //nolint:errcheck // the first list
	c.add(2 * time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if ok, err := v.Has(ctx, "echo", "v1.1.0"); ok || !errors.Is(err, context.Canceled) {
		t.Fatalf("has = %v, %v; want the reader's cancel", ok, err)
	}
}

func TestRegistryTagsWithoutReleaseRepository(t *testing.T) {
	r := newTestRegistry(t)
	_, err := RegistryTags(r.sources(), registry.NewClients(), log.NewNop())(context.Background(), "echo")
	if !errors.Is(err, ErrNoVersions) || !Unpublished(err) {
		t.Fatalf("err = %v; want ErrNoVersions", err)
	}
}

func TestVersionsHasAfterTheWait(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int32
	v := NewVersions(func(context.Context, string) ([]string, error) {
		if calls.Add(1) > 1 {
			<-release
			return []string{"v1.1.0", "v1.0.0"}, nil
		}
		return []string{"v1.0.0"}, nil
	}, 30*time.Minute, time.Minute, nil)
	v.hasWait = 10 * time.Millisecond
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	v.List(context.Background(), "echo") //nolint:errcheck // the first list
	c.add(2 * time.Minute)
	// The registry is slow: the reader gets "not found", the listing goes on.
	if ok, err := v.Has(context.Background(), "echo", "v1.1.0"); ok || err != nil {
		t.Fatalf("has = %v, %v; want not found after the wait", ok, err)
	}
	close(release)
	waitFor(t, func() bool { ok, _ := v.Has(context.Background(), "echo", "v1.1.0"); return ok })
}

func TestVersionsRelistAfterAChangedOrFailedList(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	lists := [][]string{{"v1.0.0"}, {"v1.0.0"}, {"v1.0.0"}, {"v1.1.0", "v1.0.0"}}
	v := NewVersions(func(context.Context, string) ([]string, error) {
		if fail.Load() {
			calls.Add(1)
			return nil, errors.New("registry is down")
		}
		n := int(calls.Add(1))
		return lists[min(n, len(lists))-1], nil
	}, 30*time.Minute, time.Minute, nil)
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	relist := func() time.Duration { cv, _ := v.cached("echo"); return cv.relist }
	v.List(context.Background(), "echo") //nolint:errcheck // the first list
	c.add(2 * time.Minute)
	v.Has(context.Background(), "echo", "v9.9.9") //nolint:errcheck // nothing new: the wait doubles
	if relist() != 2*minRelist {
		t.Fatalf("relist = %v, want %v", relist(), 2*minRelist)
	}

	// A failed relisting keeps the list and does not double the wait.
	fail.Store(true)
	c.add(3 * time.Minute)
	v.Has(context.Background(), "echo", "v9.9.9") //nolint:errcheck
	if relist() != 2*minRelist {
		t.Fatalf("relist after a failure = %v, want %v", relist(), 2*minRelist)
	}

	// A listing that finds a new version lets the next one relist after minRelist.
	fail.Store(false)
	c.add(31 * time.Minute)
	v.List(context.Background(), "echo") //nolint:errcheck // starts the listing by the TTL
	waitFor(t, func() bool { got, _ := v.List(context.Background(), "echo"); return len(got) == 2 })
	if relist() != 0 {
		t.Fatalf("relist after a changed list = %v, want 0", relist())
	}
}

func TestVersionsHasDoesNotKnowWithAStaleList(t *testing.T) {
	var fail atomic.Bool
	v := NewVersions(func(context.Context, string) ([]string, error) {
		if fail.Load() {
			return nil, errors.New("registry is down")
		}
		return []string{"v1.0.0"}, nil
	}, 5*time.Minute, time.Minute, nil)
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now

	v.List(context.Background(), "echo") //nolint:errcheck // the first list
	if v.Incomplete("echo") || v.Incomplete("unknown") {
		t.Fatal("a full list or a module without one is reported as incomplete")
	}
	fail.Store(true)
	c.add(5 * time.Minute)
	v.List(context.Background(), "echo") //nolint:errcheck // the listing fails, the list stays
	waitFor(t, func() bool { cv, _ := v.cached("echo"); return cv.failed })

	if !v.Incomplete("echo") {
		t.Fatal("a list kept after a failed listing is not reported as incomplete")
	}

	// A version missing from the kept list may be published: the render
	// finds out, the reader does not get "not found".
	if ok, err := v.Has(context.Background(), "echo", "v1.1.0"); ok || !errors.Is(err, errStaleList) || Unpublished(err) {
		t.Fatalf("has = %v, %v; want errStaleList", ok, err)
	}
	if ok, err := v.Has(context.Background(), "echo", "v1.0.0"); !ok || err != nil {
		t.Fatalf("has v1.0.0 = %v, %v; want true from the kept list", ok, err)
	}
}

func TestRegistryTagsOfASourceThatDidNotAnswer(t *testing.T) {
	r := newTestRegistry(t)
	r.push("src1/echo/release:v1.0.0", release("v1.0.0")...)
	r.failing.Store(&[]string{"/v2/src2/"})

	tags, err := RegistryTags(r.sources(), registry.NewClients(), log.NewNop())(context.Background(), "echo")
	if !errors.Is(err, errPartialList) || !slices.Equal(tags, []string{"v1.0.0"}) {
		t.Fatalf("tags %v, err %v; want the tags of src1 and errPartialList", tags, err)
	}

	// The versions of the source that answered are kept, read again after
	// failedTTL.
	v := NewVersions(RegistryTags(r.sources(), registry.NewClients(), log.NewNop()), 30*time.Minute, time.Minute, nil)
	c := &clock{t: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	v.now = c.now
	got, err := v.List(context.Background(), "echo")
	if err != nil || !slices.Equal(got, []string{"v1.0.0"}) {
		t.Fatalf("versions %v, %v; want v1.0.0", got, err)
	}
	if cv, _ := v.cached("echo"); !cv.failed || cv.until.Sub(cv.listed) != time.Minute {
		t.Fatalf("a partial list is cached as %+v; want failed for a minute", cv)
	}
	if !v.Incomplete("echo") {
		t.Fatal("a list without a source that did not answer is not reported as incomplete")
	}
}

// The tags of the store are the list, complete, although the upstream behind
// it refuses docs-builder: the upstream is read only where the store fails.
func TestRegistryTagsOfTheStore(t *testing.T) {
	r := newTestRegistry(t)
	r.push("store/echo/release:v1.0.0", release("v1.0.0")...)
	r.failing.Store(&[]string{"/v2/upstream/"})

	tags, err := RegistryTags(r.storeThenUpstream(), registry.NewClients(), log.NewNop())(context.Background(), "echo")
	if err != nil || !slices.Equal(tags, []string{"v1.0.0"}) {
		t.Fatalf("tags %v, %v; want v1.0.0 of the store, complete", tags, err)
	}
}

func TestVersionsForget(t *testing.T) {
	var calls atomic.Int32
	v := NewVersions(func(context.Context, string) ([]string, error) {
		calls.Add(1)
		return []string{"v1.0.0"}, nil
	}, time.Hour, time.Minute, nil)

	for range 2 {
		if _, err := v.List(context.Background(), "echo"); err != nil {
			t.Fatal(err)
		}
	}
	v.Forget("echo")
	if _, err := v.List(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	if n := calls.Load(); n != 2 {
		t.Fatalf("listings = %d, want 2: one cached, one after Forget", n)
	}
}

// A listing in flight when the module is forgotten does not cache what it
// found, and the next request lists the module anew.
func TestVersionsForgetDuringAListing(t *testing.T) {
	release := make(chan struct{})
	var calls atomic.Int32
	v := NewVersions(func(context.Context, string) ([]string, error) {
		if calls.Add(1) == 1 {
			<-release
			return []string{"v1.0.0"}, nil
		}
		return []string{"v2.0.0"}, nil
	}, time.Hour, time.Minute, nil)

	done := make(chan []string)
	go func() {
		got, _ := v.List(context.Background(), "echo")
		done <- got
	}()
	for calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	v.Forget("echo")
	close(release)
	// The reader that started it still gets its answer.
	if got := <-done; !slices.Equal(got, []string{"v1.0.0"}) {
		t.Fatalf("in-flight listing = %v", got)
	}

	if got, err := v.List(context.Background(), "echo"); err != nil || !slices.Equal(got, []string{"v2.0.0"}) {
		t.Fatalf("after Forget = %v, %v, want [v2.0.0]", got, err)
	}
}
