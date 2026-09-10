/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheRenew(t *testing.T) {
	cache := newTestCache()

	err := cache.renewCacheOnceNoRetry("test")
	if err != nil {
		t.Fatal(err)
	}

	apiGroup, ok := cache.getFromCache("test")
	if ok != true {
		t.Fatal("api group is not found")
	}

	if apiGroup.AddTime != cache.now() {
		t.Fatalf("time: %v != %v", apiGroup.AddTime, cache.now())
	}

	namespaced, ok := apiGroup.Data["configmaps"]
	if ok != true {
		t.Fatal("configmaps is not found in cache")
	}

	if namespaced != true {
		t.Fatalf("configmap namespaced: %v != %v", namespaced, true)
	}

	namespaced, ok = apiGroup.Data["nodes"]
	if ok != true {
		t.Fatal("nodes is not found in cache")
	}

	if namespaced != false {
		t.Fatalf("nodes namespaced: %v != %v", namespaced, false)
	}

	if len(apiGroup.Data) != 2 {
		t.Fatalf("cached objects: %v != %v", len(apiGroup.Data), 2)

	}
}

func TestCacheGet(t *testing.T) {
	cache := newTestCache()

	namespaced, err := cache.Get("test", "nodes")
	if err != nil {
		t.Fatal(err)
	}

	if namespaced != false {
		t.Fatalf("nodes namespaced: %v != %v", namespaced, false)
	}

	namespaced, err = cache.Get("test", "configmaps")
	if err != nil {
		t.Fatal(err)
	}

	if namespaced != true {
		t.Fatalf("configmaps namespaced: %v != %v", namespaced, false)
	}
}

func TestCachePreferredVersionGet(t *testing.T) {
	cache := newTestPreferredVersionCache()

	const resource = "challenges"
	const group = "acme.cert-manager.io"
	const expectedVersion = "v1"

	version := cache.preferredVersionFromCache(group, resource)
	if version != "" {
		t.Fatalf("cache is not empty")
	}

	version, err := cache.GetPreferredVersion(group, resource)
	if err != nil {
		t.Fatal(err)
	}

	if version != expectedVersion {
		t.Fatalf("acme.cert-manager.io: %v != %v", version, expectedVersion)
	}

	version = cache.preferredVersionFromCache(group, resource)
	if version == "" {
		t.Fatal("version for group is not saved in cache")
	}

	now := cache.now()
	// change client here to not be able to update the cache
	cache.now = func() time.Time { return now.Add(time.Hour * 3) }

	version = cache.preferredVersionFromCache(group, resource)
	if version != "" {
		t.Fatalf("version is not expired")
	}

	version, err = cache.GetPreferredVersion(group, resource)
	if err != nil {
		t.Fatal(err)
	}

	if version != expectedVersion {
		t.Fatalf("acme.cert-manager.io did not get after expire: %v != %v", version, expectedVersion)
	}
}

// A resource no version of the group serves is an ANSWER, not a failure to ask.
//
// This used to come back as a plain error, which denies - the 403-for-a-resource-that-does-not-
// exist that ErrResourceAbsent exists to prevent, on the one path where it was still happening.
// The path is not exotic: it is taken by every review that arrives without an apiVersion, which is
// what `kubectl auth can-i` sends, so the wrong answer landed in the tool people debug with.
func TestCachePreferredVersionAbsentResourceIsAbsentNotAnError(t *testing.T) {
	cache := newTestPreferredVersionCache()

	_, err := cache.GetPreferredVersion("acme.cert-manager.io", "ghosts")
	if !errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("got %v, want ErrResourceAbsent so the caller lets RBAC answer and the request 404s", err)
	}
}

// And that answer is remembered, so asking for it repeatedly does not list the group every time.
func TestCachePreferredVersionAbsentResourceIsNotResolvedOnEveryRequest(t *testing.T) {
	var listings int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&listings, 1)
		w.Header().Add("Content-Type", "application/json")
		switch r.URL.Path {
		case "/apis/acme.cert-manager.io":
			w.Write([]byte(preferredVersionResponse))
		case "/apis/acme.cert-manager.io/v1":
			w.Write([]byte(discoveryByVersionResponse))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte{})
		}
	}))
	defer server.Close()

	cache := NamespacedDiscoveryCache{
		client:               server.Client(),
		kubernetesAPIAddress: server.URL,
		logger:               log.New(io.Discard, "", log.LstdFlags),
		data:                 make(map[string]*namespacedCacheEntry),
		preferredVersions:    make(map[string]*preferredVersionCacheEntry),
		negative:             make(map[string]negativeEntry),
		inflight:             make(map[string]chan struct{}),
	}
	now := time.Now()
	cache.now = func() time.Time { return now }

	// The first resolution walks the group: the versions, then each version until one serves the
	// resource or all of them have answered.
	if _, err := cache.GetPreferredVersion("acme.cert-manager.io", "ghosts"); !errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("first lookup: got %v, want ErrResourceAbsent", err)
	}
	first := atomic.LoadInt32(&listings)
	if first == 0 {
		t.Fatal("the first lookup asked the API server nothing at all")
	}

	for i := 0; i < 50; i++ {
		if _, err := cache.GetPreferredVersion("acme.cert-manager.io", "ghosts"); !errors.Is(err, ErrResourceAbsent) {
			t.Fatalf("lookup %d: got %v, want ErrResourceAbsent", i, err)
		}
	}
	if got := atomic.LoadInt32(&listings); got != first {
		t.Errorf("50 more lookups produced %d further requests to the API server, want 0", got-first)
	}

	// Once the interval passes it asks again, so a resource installed meanwhile is found.
	cache.now = func() time.Time { return now.Add(negativeRenewInterval) }
	if _, err := cache.GetPreferredVersion("acme.cert-manager.io", "ghosts"); !errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("after the interval: got %v, want ErrResourceAbsent", err)
	}
	if got := atomic.LoadInt32(&listings); got <= first {
		t.Errorf("after the interval the group was not asked about again (%d requests in total)", got)
	}
}

func TestCacheGetIfNoResource(t *testing.T) {
	cache := newTestCache()

	err := cache.renewCacheOnceNoRetry("test")
	if err != nil {
		t.Fatal(err)
	}

	delete(cache.data["test"].Data, "nodes")

	// A resource missing from the listing is looked up again - but only once the listing is older
	// than negativeRenewInterval, so move past it.
	now := cache.now()
	cache.now = func() time.Time { return now.Add(negativeRenewInterval) }

	namespaced, err := cache.Get("test", "nodes")
	if err != nil {
		t.Fatal(err)
	}

	if namespaced != false {
		t.Fatalf("nodes namespaced: %v != %v", namespaced, false)
	}
}

// A refresh that fails must not throw away an authoritative negative. The whole point of answering
// 404 rather than 403 for a resource that does not exist is that the answer is stable; if every API
// server blip turned it back into a denial, the fix would hold only while the cluster is healthy.
func TestCacheGetKeepsTheNegativeWhenTheRefreshFails(t *testing.T) {
	var fail atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte(testResponse))
	}))
	defer server.Close()

	cache := NamespacedDiscoveryCache{
		client:               server.Client(),
		kubernetesAPIAddress: server.URL,
		logger:               log.New(io.Discard, "", log.LstdFlags),
		data:                 make(map[string]*namespacedCacheEntry),
		preferredVersions:    make(map[string]*preferredVersionCacheEntry),
	}
	now := time.Now()
	cache.now = func() time.Time { return now }

	// A successful listing that does not carry the resource.
	if _, err := cache.Get("test", "ghosts"); !errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("with discovery healthy: got %v, want ErrResourceAbsent", err)
	}

	// The interval passes and discovery is now unreachable. The answer must not change: the
	// listing that produced it succeeded, and a resource cannot be installed while the API server
	// cannot be reached.
	cache.now = func() time.Time { return now.Add(negativeRenewInterval) }
	fail.Store(true)
	if _, err := cache.Get("test", "ghosts"); !errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("with discovery unreachable: got %v, want ErrResourceAbsent (a denial here is a 403 for a resource that does not exist)", err)
	}

	// A group that was never listed is a different matter: nothing authoritative is known, so the
	// failure has to propagate and the decision fails closed.
	if _, err := cache.Get("never-listed/v1", "things"); err == nil || errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("an unlisted group during an outage: got %v, want a plain error so the decision denies", err)
	}
}

// A listing that FAILS must be remembered too, not only one that answered.
//
// The rate limit used to key on the cache entry, which a failed listing never creates - so a group
// whose listing kept failing was retried on every single request. An API server having a bad
// minute then turned every authorization request in the cluster into another attempt against it,
// which is the shape of an outage that feeds itself.
func TestCacheFailedListingIsNotRetriedOnEveryRequest(t *testing.T) {
	var attempts int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cache := NamespacedDiscoveryCache{
		client:               server.Client(),
		kubernetesAPIAddress: server.URL,
		logger:               log.New(io.Discard, "", log.LstdFlags),
		data:                 make(map[string]*namespacedCacheEntry),
		preferredVersions:    make(map[string]*preferredVersionCacheEntry),
		negative:             make(map[string]negativeEntry),
		inflight:             make(map[string]chan struct{}),
	}
	now := time.Now()
	cache.now = func() time.Time { return now }

	for i := 0; i < 30; i++ {
		if _, err := cache.Get("unreachable/v1", "things"); err == nil {
			t.Fatalf("lookup %d: a listing that failed must not report success", i)
		}
		// And it must deny rather than let RBAC answer: a failure is not proof of absence.
		if _, err := cache.Get("unreachable/v1", "things"); errors.Is(err, ErrResourceAbsent) {
			t.Fatalf("lookup %d: a failed listing must not be reported as the resource being absent", i)
		}
	}
	if got := atomic.LoadInt32(&attempts); got != 1 {
		t.Errorf("60 lookups produced %d attempts against the API server, want 1", got)
	}

	// Once the interval passes it tries again, so a recovered API server is noticed.
	cache.now = func() time.Time { return now.Add(negativeRenewInterval) }
	_, _ = cache.Get("unreachable/v1", "things")
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Errorf("after the interval: %d attempts in total, want 2", got)
	}
}

// Concurrent lookups of the same group share one listing.
//
// The rate limit only applies once an attempt has finished, so without this everything that arrives
// while the first listing is in flight goes out on its own - and the concurrency of this path is
// the whole cluster's authorization traffic.
func TestCacheConcurrentLookupsShareOneListing(t *testing.T) {
	var listings int32
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&listings, 1)
		<-release // hold the first listing open so the others pile up behind it
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte(testResponse))
	}))
	defer server.Close()

	cache := NamespacedDiscoveryCache{
		client:               server.Client(),
		kubernetesAPIAddress: server.URL,
		logger:               log.New(io.Discard, "", log.LstdFlags),
		data:                 make(map[string]*namespacedCacheEntry),
		preferredVersions:    make(map[string]*preferredVersionCacheEntry),
		negative:             make(map[string]negativeEntry),
		inflight:             make(map[string]chan struct{}),
	}
	now := time.Now()
	cache.now = func() time.Time { return now }

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cache.Get("test", "nodes")
		}()
	}

	// Let the first listing through once the others have had time to arrive.
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := atomic.LoadInt32(&listings); got > 2 {
		t.Errorf("20 concurrent lookups of one group produced %d listings, want at most 2", got)
	}
}

// The memory of unserved groups must be bounded. Its keys come out of the request path, so a
// subject a rule covers can name a different made-up group on every request; remembering them all
// would turn a rate limit into a way to grow the webhook's heap until the node reclaims it - on the
// component every authorization request in the cluster is waiting for.
func TestCacheUnservedGroupsAreBounded(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cache := NamespacedDiscoveryCache{
		client:               server.Client(),
		kubernetesAPIAddress: server.URL,
		logger:               log.New(io.Discard, "", log.LstdFlags),
		data:                 make(map[string]*namespacedCacheEntry),
		preferredVersions:    make(map[string]*preferredVersionCacheEntry),
		negative:             make(map[string]negativeEntry),
	}
	now := time.Now()
	cache.now = func() time.Time { return now }

	for i := 0; i < maxUnservedGroups*4; i++ {
		if _, err := cache.Get(fmt.Sprintf("made-up-%d.example.com/v1", i), "things"); !errors.Is(err, ErrResourceAbsent) {
			t.Fatalf("lookup %d: got %v, want ErrResourceAbsent", i, err)
		}
	}

	cache.muNegative.Lock()
	unserved := len(cache.negative)
	cache.muNegative.Unlock()
	if unserved > maxUnservedGroups {
		t.Errorf("the negative cache holds %d groups after %d distinct lookups, cap is %d",
			unserved, maxUnservedGroups*4, maxUnservedGroups)
	}

	// And the groups that ARE served stay out of it entirely, so the cap can never evict something
	// a real request depends on.
	cache.mu.RLock()
	served := len(cache.data)
	cache.mu.RUnlock()
	if served != 0 {
		t.Errorf("the positive cache grew to %d entries from lookups of groups that do not exist", served)
	}
}

// A group the API server does not serve must be remembered too. The caller picks the group out of
// the request path, so an unknown group is the cheapest possible way to ask for a listing; if the
// 404 leaves no entry behind, the rate limit never applies to it and every request is a fresh round
// trip to the API server from the authorization path.
func TestCacheGetUnknownGroupIsNegativelyCached(t *testing.T) {
	var listings int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&listings, 1)
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cache := NamespacedDiscoveryCache{
		client:               server.Client(),
		kubernetesAPIAddress: server.URL,
		logger:               log.New(io.Discard, "", log.LstdFlags),
		data:                 make(map[string]*namespacedCacheEntry),
		preferredVersions:    make(map[string]*preferredVersionCacheEntry),
	}
	now := time.Now()
	cache.now = func() time.Time { return now }

	// A group that is not served is an answer - the resource is absent - not a failure to ask.
	if _, err := cache.Get("nosuchgroup/v1", "things"); !errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("first lookup: got %v, want ErrResourceAbsent", err)
	}
	first := atomic.LoadInt32(&listings)
	if first == 0 {
		t.Fatal("the first lookup did not ask the API server at all")
	}

	// And every lookup within the interval is answered without asking again.
	for i := 0; i < 50; i++ {
		if _, err := cache.Get("nosuchgroup/v1", "things"); !errors.Is(err, ErrResourceAbsent) {
			t.Fatalf("lookup %d: got %v, want ErrResourceAbsent", i, err)
		}
	}
	if got := atomic.LoadInt32(&listings); got != first {
		t.Fatalf("50 further lookups produced %d discovery requests in total, want %d", got, first)
	}

	// A different made-up group is a different key, so it costs its own listing and no more. This
	// is what bounds the amplifier: work is proportional to distinct groups over the interval, not
	// to the request rate.
	before := atomic.LoadInt32(&listings)
	for i := 0; i < 20; i++ {
		if _, err := cache.Get("othergroup/v1", "things"); !errors.Is(err, ErrResourceAbsent) {
			t.Fatalf("other group lookup %d: got %v", i, err)
		}
	}
	perGroup := atomic.LoadInt32(&listings) - before
	if perGroup != first {
		t.Fatalf("20 lookups of a second unknown group produced %d discovery requests, want %d", perGroup, first)
	}

	// Once the interval passes the group is asked about again, so a CRD installed since then is
	// picked up.
	cache.now = func() time.Time { return now.Add(negativeRenewInterval) }
	if _, err := cache.Get("nosuchgroup/v1", "things"); !errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("after the interval: got %v, want ErrResourceAbsent", err)
	}
	if got := atomic.LoadInt32(&listings); got <= perGroup+before {
		t.Fatal("the lookup after the interval did not re-ask the API server")
	}
}

// A resource that is not there must not make the webhook list its group on every request: the
// listing is a request to the API server made from the authorization path, and the caller picks
// the resource name.
func TestCacheGetAbsentResourceDoesNotRelistWithinTheInterval(t *testing.T) {
	var listings int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&listings, 1)
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte(testResponse))
	}))
	defer server.Close()

	cache := NamespacedDiscoveryCache{
		client:               server.Client(),
		kubernetesAPIAddress: server.URL,
		logger:               log.New(io.Discard, "", log.LstdFlags),
		data:                 make(map[string]*namespacedCacheEntry),
		preferredVersions:    make(map[string]*preferredVersionCacheEntry),
	}
	now := time.Now()
	cache.now = func() time.Time { return now }

	// The first lookup of an unknown group lists it once.
	if _, err := cache.Get("test", "ghosts"); !errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("first lookup: got %v, want ErrResourceAbsent", err)
	}
	if got := atomic.LoadInt32(&listings); got != 1 {
		t.Fatalf("the first lookup listed the group %d times, want 1", got)
	}

	// Every further lookup within the interval is answered from what was listed.
	for i := 0; i < 50; i++ {
		if _, err := cache.Get("test", "ghosts"); !errors.Is(err, ErrResourceAbsent) {
			t.Fatalf("lookup %d: got %v, want ErrResourceAbsent", i, err)
		}
	}
	if got := atomic.LoadInt32(&listings); got != 1 {
		t.Fatalf("50 more lookups listed the group %d times in total, want 1", got)
	}

	// Once the interval has passed, the group is listed again - a resource installed since then
	// has to become visible.
	cache.now = func() time.Time { return now.Add(negativeRenewInterval) }
	if _, err := cache.Get("test", "ghosts"); !errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("after the interval: got %v, want ErrResourceAbsent", err)
	}
	if got := atomic.LoadInt32(&listings); got != 2 {
		t.Fatalf("the lookup after the interval listed the group %d times in total, want 2", got)
	}
}

// The same rate limit has to hold for a resource missing from a group that IS listed.
//
// This is the sibling of the path above and it had the same hole from the other side: the failed
// attempt was recorded but never read here, because this branch keys the rate limit on the age of
// the last successful listing, which a failure does not move. So while the API server was
// unreachable, every request naming an absent resource in a known group produced another attempt
// against it - and the caller writes the resource name, so any subject a rule covers can pick one.
func TestCacheAbsentResourceDoesNotRelistWhileTheListingKeepsFailing(t *testing.T) {
	var listings int32
	var fail atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&listings, 1)
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte(testResponse))
	}))
	defer server.Close()

	cache := NamespacedDiscoveryCache{
		client:               server.Client(),
		kubernetesAPIAddress: server.URL,
		logger:               log.New(io.Discard, "", log.LstdFlags),
		data:                 make(map[string]*namespacedCacheEntry),
		preferredVersions:    make(map[string]*preferredVersionCacheEntry),
		negative:             make(map[string]negativeEntry),
		inflight:             make(map[string]chan struct{}),
	}
	now := time.Now()
	cache.now = func() time.Time { return now }

	// One successful listing that does not carry the resource.
	if _, err := cache.Get("test", "ghosts"); !errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("first lookup: got %v, want ErrResourceAbsent", err)
	}
	atomic.StoreInt32(&listings, 0)

	// The interval passes and the API server stops answering. The refresh is attempted once; the
	// requests that follow must be answered from the listing already held.
	cache.now = func() time.Time { return now.Add(negativeRenewInterval) }
	fail.Store(true)
	for i := 0; i < 30; i++ {
		if _, err := cache.Get("test", "ghosts"); !errors.Is(err, ErrResourceAbsent) {
			t.Fatalf("lookup %d during the outage: got %v, want ErrResourceAbsent", i, err)
		}
	}
	if got := atomic.LoadInt32(&listings); got != 1 {
		t.Errorf("30 lookups during the outage produced %d attempts against the API server, want 1", got)
	}

	// Once the interval passes again, one more attempt is made, so a recovered API server and a
	// resource installed meanwhile are noticed.
	cache.now = func() time.Time { return now.Add(2 * negativeRenewInterval) }
	fail.Store(false)
	if _, err := cache.Get("test", "ghosts"); !errors.Is(err, ErrResourceAbsent) {
		t.Fatalf("after the outage: got %v, want ErrResourceAbsent", err)
	}
	if got := atomic.LoadInt32(&listings); got != 2 {
		t.Errorf("after the outage: %d attempts in total, want 2", got)
	}
}

func TestCacheStale(t *testing.T) {
	cache := newTestCache()

	err := cache.renewCacheOnceNoRetry("test")
	if err != nil {
		t.Fatal(err)
	}

	now := cache.now()

	// change client here to not be able to update the cache
	cache.now = func() time.Time { return now.Add(time.Hour * 3) }
	cache.client = http.DefaultClient

	namespaced, err := cache.Get("test", "nodes")
	if err != nil {
		t.Fatal(err)
	}

	if namespaced != false {
		t.Fatalf("nodes namespaced: %v != %v", namespaced, false)
	}

	apiGroup, ok := cache.getFromCache("test")
	if ok != true {
		t.Fatal("api group is not found")
	}

	if apiGroup.AddTime != now {
		t.Fatal("cache was updated")
	}
}

func TestCacheCheck(t *testing.T) {
	server := newErrorServer()

	cache := NamespacedDiscoveryCache{}
	cache.logger = log.New(io.Discard, "", log.LstdFlags)

	cache.client = server.Client()
	cache.kubernetesAPIAddress = server.URL

	// One attempt, and the error says what happened. Check used to retry ten times over about
	// twenty seconds; the readiness probe that calls it gives up long before that and fires again,
	// so the retries only accumulated overlapping requests against an API server that is already
	// unreachable. The probe period is the retry.
	expectedErr := "check API: kube response error: 500 ERROR"

	err := cache.Check(context.Background())
	if err.Error() != expectedErr {
		t.Fatalf("%q received, expected %q", err.Error(), expectedErr)
	}

	server = newTestServer()
	cache.client = server.Client()
	cache.kubernetesAPIAddress = server.URL

	err = cache.Check(context.Background())
	if err != nil {
		t.Fatalf("%q received, expected nil", err.Error())
	}
}

// A caller that stops waiting stops the work. The readiness handler passes the request's context,
// so a probe the kubelet has already given up on does not leave a request in flight against an API
// server that is struggling.
func TestCacheCheckHonoursTheCallersContext(t *testing.T) {
	blocked := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-blocked
	}))
	defer server.Close()
	defer close(blocked)

	cache := NamespacedDiscoveryCache{
		client:               server.Client(),
		kubernetesAPIAddress: server.URL,
		logger:               log.New(io.Discard, "", log.LstdFlags),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { done <- cache.Check(ctx) }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a cancelled check returned success")
		}
	case <-time.After(requestTimeout):
		t.Fatal("the check outlived its cancelled context")
	}
}

func TestNewNamespacedDiscoveryCacheAPIAddress(t *testing.T) {
	logger := log.New(io.Discard, "", log.LstdFlags)

	c := NewNamespacedDiscoveryCache(logger, "")
	if c.kubernetesAPIAddress != kubernetesAPIAddress {
		t.Fatalf("empty apiAddress: got %q, want %q", c.kubernetesAPIAddress, kubernetesAPIAddress)
	}

	c = NewNamespacedDiscoveryCache(logger, "https://10.0.0.1:6443")
	if want := "https://10.0.0.1:6443"; c.kubernetesAPIAddress != want {
		t.Fatalf("explicit apiAddress: got %q, want %q", c.kubernetesAPIAddress, want)
	}
}

func newTestCache() *NamespacedDiscoveryCache {
	server := newTestServer()

	cache := NamespacedDiscoveryCache{}

	cache.client = server.Client()
	cache.kubernetesAPIAddress = server.URL

	now := time.Now()
	cache.now = func() time.Time { return now }

	cache.logger = log.New(io.Discard, "", log.LstdFlags)
	cache.data = make(map[string]*namespacedCacheEntry)
	cache.preferredVersions = make(map[string]*preferredVersionCacheEntry)

	server.Config.ErrorLog = cache.logger

	return &cache
}

func newTestPreferredVersionCache() *NamespacedDiscoveryCache {
	server := newPreferredVersionTestServer()

	cache := NamespacedDiscoveryCache{}

	cache.client = server.Client()
	cache.kubernetesAPIAddress = server.URL

	now := time.Now()
	cache.now = func() time.Time { return now }

	cache.logger = log.New(io.Discard, "", log.LstdFlags)
	cache.data = make(map[string]*namespacedCacheEntry)
	cache.preferredVersions = make(map[string]*preferredVersionCacheEntry)

	server.Config.ErrorLog = cache.logger

	return &cache
}

func newTestCoreResourcesCache() *NamespacedDiscoveryCache {
	server := newCoreResourcesTestServer()

	cache := NamespacedDiscoveryCache{}

	cache.client = server.Client()
	cache.kubernetesAPIAddress = server.URL

	now := time.Now()
	cache.now = func() time.Time { return now }

	cache.logger = log.New(io.Discard, "", log.LstdFlags)
	cache.data = make(map[string]*namespacedCacheEntry)
	cache.preferredVersions = make(map[string]*preferredVersionCacheEntry)

	server.Config.ErrorLog = cache.logger

	return &cache
}

func newTestServer() *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte(testResponse))
	}))
}

func newPreferredVersionTestServer() *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		switch r.URL.Path {
		case "/apis/acme.cert-manager.io":
			w.Write([]byte(preferredVersionResponse))
		case "/apis/acme.cert-manager.io/v1":
			w.Write([]byte(discoveryByVersionResponse))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte{})
		}
	}))
}

func newCoreResourcesTestServer() *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1":
			w.Write([]byte(coreResourcesResponse))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte{})
		}
	}))
}

func newErrorServer() *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write([]byte("ERROR"))
	}))
}

const preferredVersionResponse = `
{
  "kind": "APIGroup",
  "apiVersion": "v1",
  "name": "acme.cert-manager.io",
  "versions": [
    {
      "groupVersion": "acme.cert-manager.io/v1",
      "version": "v1"
    },
    {
      "groupVersion": "acme.cert-manager.io/v1beta1",
      "version": "v1beta1"
    },
    {
      "groupVersion": "acme.cert-manager.io/v1alpha3",
      "version": "v1alpha3"
    },
    {
      "groupVersion": "acme.cert-manager.io/v1alpha2",
      "version": "v1alpha2"
    }
  ],
  "preferredVersion": {
    "groupVersion": "acme.cert-manager.io/v1",
    "version": "v1"
  }
}`

const discoveryByVersionResponse = `
{
  "kind": "APIResourceList",
  "apiVersion": "v1",
  "groupVersion": "acme.cert-manager.io/v1",
  "resources": [
    {
      "name": "orders",
      "singularName": "order",
      "namespaced": true,
      "kind": "Order",
      "verbs": [
        "delete",
        "deletecollection",
        "get",
        "list",
        "patch",
        "create",
        "update",
        "watch"
      ],
      "categories": [
        "cert-manager",
        "cert-manager-acme"
      ],
      "storageVersionHash": "FQscJvYs/a4="
    },
    {
      "name": "challenges",
      "singularName": "challenge",
      "namespaced": true,
      "kind": "Challenge",
      "verbs": [
        "delete",
        "deletecollection",
        "get",
        "list",
        "patch",
        "create",
        "update",
        "watch"
      ],
      "categories": [
        "cert-manager",
        "cert-manager-acme"
      ],
      "storageVersionHash": "T6RvmdSxRBY="
    }
  ]
}`

const testResponse = `{
  "kind": "APIResourceList",
  "groupVersion": "v1",
  "resources": [
    {
      "name": "configmaps",
      "singularName": "",
      "namespaced": true,
      "kind": "ConfigMap",
      "verbs": [
        "create",
        "delete",
        "deletecollection",
        "get",
        "list",
        "patch",
        "update",
        "watch"
      ],
      "shortNames": [
        "cm"
      ],
      "storageVersionHash": "qFsyl6wFWjQ="
    },
    {
      "name": "nodes",
      "singularName": "",
      "namespaced": false,
      "kind": "Node",
      "verbs": [
        "create",
        "delete",
        "deletecollection",
        "get",
        "list",
        "patch",
        "update",
        "watch"
      ],
      "storageVersionHash": "r2yiGXH7wu8="
    }
  ]
}`

const coreResourcesResponse = `
{
  "kind": "APIResourceList",
  "groupVersion": "v1",
  "resources": [
    {
      "name": "bindings",
      "singularName": "binding",
      "namespaced": true
    },
    {
      "name": "componentstatuses",
      "singularName": "componentstatus",
      "namespaced": false
    },
    {
      "name": "configmaps",
      "singularName": "configmap",
      "namespaced": true,
      "kind": "ConfigMap"
    },
    {
      "name": "endpoints",
      "singularName": "endpoints",
      "namespaced": true,
      "kind": "Endpoints"
    },
    {
      "name": "events",
      "singularName": "event",
      "namespaced": true,
      "kind": "Event"
    },
    {
      "name": "limitranges",
      "singularName": "limitrange",
      "namespaced": true,
      "kind": "LimitRange"
    },
    {
      "name": "namespaces/finalize",
      "singularName": "",
      "namespaced": false
    },
    {
      "name": "namespaces/status",
      "singularName": "",
      "namespaced": false,
      "kind": "Namespace"
    },
    {
      "name": "nodes",
      "singularName": "node",
      "namespaced": false,
      "kind": "Node"
    },
    {
      "name": "nodes/proxy",
      "singularName": "",
      "namespaced": false,
      "kind": "NodeProxyOptions"
    },
    {
      "name": "nodes/status",
      "singularName": "",
      "namespaced": false,
      "kind": "Node"
    },
    {
      "name": "persistentvolumeclaims",
      "singularName": "persistentvolumeclaim",
      "namespaced": true,
      "kind": "PersistentVolumeClaim"
    },
    {
      "name": "persistentvolumeclaims/status",
      "singularName": "",
      "namespaced": true,
      "kind": "PersistentVolumeClaim"
    },
    {
      "name": "persistentvolumes",
      "singularName": "persistentvolume",
      "namespaced": false,
      "kind": "PersistentVolume"
    },
    {
      "name": "persistentvolumes/status",
      "singularName": "",
      "namespaced": false,
      "kind": "PersistentVolume"
    },
    {
      "name": "pods",
      "singularName": "pod",
      "namespaced": true,
      "kind": "Pod"
    },
    {
      "name": "pods/attach",
      "singularName": "",
      "namespaced": true,
      "kind": "PodAttachOptions"
    },
    {
      "name": "pods/binding",
      "singularName": "",
      "namespaced": true,
      "kind": "Binding"
    },
    {
      "name": "pods/ephemeralcontainers",
      "singularName": "",
      "namespaced": true,
      "kind": "Pod"
    },
    {
      "name": "pods/eviction",
      "singularName": "",
      "namespaced": true,
      "group": "policy",
      "version": "v1",
      "kind": "Eviction"
    },
    {
      "name": "pods/exec",
      "singularName": "",
      "namespaced": true,
      "kind": "PodExecOptions"
    },
    {
      "name": "pods/log",
      "singularName": "",
      "namespaced": true,
      "kind": "Pod"
    },
    {
      "name": "pods/portforward",
      "singularName": "",
      "namespaced": true,
      "kind": "PodPortForwardOptions"
    },
    {
      "name": "pods/proxy",
      "singularName": "",
      "namespaced": true,
      "kind": "PodProxyOptions"
    },
    {
      "name": "pods/status",
      "singularName": "",
      "namespaced": true,
      "kind": "Pod"
    },
    {
      "name": "podtemplates",
      "singularName": "podtemplate",
      "namespaced": true,
      "kind": "PodTemplate"
    },
    {
      "name": "replicationcontrollers",
      "singularName": "replicationcontroller",
      "namespaced": true,
      "kind": "ReplicationController"
    },
    {
      "name": "replicationcontrollers/scale",
      "singularName": "",
      "namespaced": true,
      "group": "autoscaling",
      "version": "v1",
      "kind": "Scale"
    },
    {
      "name": "replicationcontrollers/status",
      "singularName": "",
      "namespaced": true,
      "kind": "ReplicationController"
    },
    {
      "name": "resourcequotas",
      "singularName": "resourcequota",
      "namespaced": true,
      "kind": "ResourceQuota"
    },
    {
      "name": "resourcequotas/status",
      "singularName": "",
      "namespaced": true,
      "kind": "ResourceQuota"
    },
    {
      "name": "secrets",
      "singularName": "secret",
      "namespaced": true,
      "kind": "Secret"
    },
    {
      "name": "serviceaccounts",
      "singularName": "serviceaccount",
      "namespaced": true,
      "kind": "ServiceAccount"
    },
    {
      "name": "serviceaccounts/token",
      "singularName": "",
      "namespaced": true,
      "group": "authentication.k8s.io",
      "version": "v1",
      "kind": "TokenRequest"
    },
    {
      "name": "services",
      "singularName": "service",
      "namespaced": true,
      "kind": "Service"
    },
    {
      "name": "services/proxy",
      "singularName": "",
      "namespaced": true,
      "kind": "ServiceProxyOptions"
    },
    {
      "name": "services/status",
      "singularName": "",
      "namespaced": true,
      "kind": "Service"
    }
  ]
}`
