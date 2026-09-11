/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cache

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	k8sversion "k8s.io/apimachinery/pkg/version"
)

const (
	renewTokenPeriod = 30 * time.Second

	defaultTTL = 1 * time.Hour

	saPath    = "/var/run/secrets/kubernetes.io/serviceaccount/"
	caPath    = saPath + "ca.crt"
	tokenPath = saPath + "token"
	apiV1Path = "/api/v1"

	// kubernetesAPIAddress is the fallback used when the caller doesn't provide one
	// (e.g. running outside the user-authz-webhook DaemonSet's env).
	kubernetesAPIAddress = "https://kubernetes.default"

	// requestTimeout bounds a single discovery/healthz request so it fails fast and
	// deterministically well within the container's livenessProbe budget, instead of
	// relying on the much larger transport-level dial/handshake timeouts.
	requestTimeout = 2 * time.Second

	// negativeRenewInterval bounds how often a group is listed again because the answer to a
	// lookup was not in what we already have.
	//
	// The listing happens on the authorization path, and the caller chooses both the group and the
	// resource name - kube-apiserver parses them lexically out of the request path before it
	// authorizes anything. Without a bound, every request naming something that does not exist
	// costs one discovery request to the API server, and any subject a rule covers can drive that
	// at whatever rate they like, against the component the whole cluster's authorization is
	// waiting on. With it, a group is listed at most this often no matter how many such requests
	// arrive.
	//
	// The cost is a staleness window, and it is worth stating plainly: for up to this long after a
	// group was listed, a resource added to it since - a freshly installed CRD - is reported as
	// absent. For a cluster-scoped request that means no opinion rather than a denial, so a subject
	// a rule limits to some namespaces could list a newly installed namespaced resource
	// cluster-wide until the next listing.
	//
	// The window the cluster sees is longer than this constant, and the two parts add up rather
	// than overlap: an answer computed from a listing up to 10s stale is then cached by the API
	// server for unauthorizedTTL, which is 30s - and it is unauthorizedTTL for every answer this
	// webhook gives, because it never allows. So the worst case is about 40 seconds from the CRD
	// being installed to the filter applying to a request that was already asked once. Nothing
	// here can shorten the second half: the API server's cache is keyed on the whole
	// SubjectAccessReview and there is no way to invalidate it from outside.
	//
	// The trade is still deliberate. Installing a CRD already requires far more privilege than
	// those 40 seconds yield, and the alternative is an unbounded amplifier any tenant can drive
	// against the component the whole cluster's authorization waits on.
	negativeRenewInterval = 10 * time.Second

	// maxUnservedGroups bounds how many "this API group is not served" answers are remembered.
	//
	// The group comes out of the request path, which the caller writes, so remembering them in the
	// same unbounded map as the real groups turned a rate limit into a memory sink: a subject a
	// rule covers could name a different made-up group on every request and grow the map for as
	// long as they cared to. Real clusters have a few dozen groups, so a cap in the hundreds never
	// evicts anything a real request needs, and the eviction below is a bounded scan rather than a
	// full LRU because the entries are interchangeable - every one of them says the same thing.
	maxUnservedGroups = 256
)

type Cache interface {
	Get(string, string) (bool, error)
	GetPreferredVersion(group, resource string) (string, error)
	Check(ctx context.Context) error
}

var _ Cache = (*NamespacedDiscoveryCache)(nil)
var ErrNotFound = errors.New("not found")

// ErrResourceAbsent means discovery answered for the group and the resource is not in the answer.
// It is separate from ErrNotFound, which is the API server's 404 for the group itself, and from
// every other error, which means the lookup did not happen. A caller may let RBAC answer for the
// two absences; it must keep failing closed for the rest.
var ErrResourceAbsent = errors.New("resource is absent from the api group")

type cacheEntry struct {
	TTL     time.Duration
	AddTime time.Time
}

func newCacheEntry(addTime time.Time) *cacheEntry {
	return &cacheEntry{
		AddTime: addTime,
		TTL:     defaultTTL,
	}
}

type namespacedCacheEntry struct {
	*cacheEntry
	Data map[string]bool
}

// groupVersionsCacheEntry is the list of versions a group serves, newest first.
type groupVersionsCacheEntry struct {
	*cacheEntry
	Versions []string
}

func newNamespacedCacheEntry(addTime time.Time) *namespacedCacheEntry {
	return &namespacedCacheEntry{
		cacheEntry: newCacheEntry(addTime),
		Data:       make(map[string]bool),
	}
}

type preferredVersionCacheEntry struct {
	*cacheEntry
	Version string
}

func newPreferredVersionCacheEntry(addTime time.Time, version string) *preferredVersionCacheEntry {
	return &preferredVersionCacheEntry{
		cacheEntry: newCacheEntry(addTime),
		Version:    version,
	}
}

type NamespacedDiscoveryCache struct {
	logger *log.Logger

	client *http.Client

	mu   sync.RWMutex
	data map[string]*namespacedCacheEntry

	muPv              sync.RWMutex
	preferredVersions map[string]*preferredVersionCacheEntry

	// muGroups guards groupVersionLists: the versions each API group serves, cached for the same
	// hour as the resource listings. The list is what resolving a preferred version starts from,
	// and the group name comes out of the request path.
	muGroups          sync.Mutex
	groupVersionLists map[string]*groupVersionsCacheEntry

	// muNegative guards negative, which remembers the groups a listing did not produce an answer
	// for - either because the API server said it does not serve them, or because the attempt
	// failed. It is kept apart from data, and bounded, because its keys are attacker-chosen.
	muNegative sync.Mutex
	negative   map[string]negativeEntry

	// muInflight guards inflight, which collapses concurrent listings of the same group into one.
	muInflight sync.Mutex
	inflight   map[string]chan struct{}

	now func() time.Time

	kubernetesAPIAddress string
}

// NewNamespacedDiscoveryCache builds a cache client talking to apiAddress (e.g.
// config.Host from the same rest.InClusterConfig() the caller already builds for its
// own clientset), so this client and the caller's are provably pointed at the same
// apiserver endpoint. Falls back to the "kubernetes.default" DNS name if apiAddress
// is empty.
func NewNamespacedDiscoveryCache(logger *log.Logger, apiAddress string) *NamespacedDiscoveryCache {
	if apiAddress == "" {
		apiAddress = kubernetesAPIAddress
	}

	c := &NamespacedDiscoveryCache{
		logger:            logger,
		data:              make(map[string]*namespacedCacheEntry),
		preferredVersions: make(map[string]*preferredVersionCacheEntry),
		negative:          make(map[string]negativeEntry),
		inflight:          make(map[string]chan struct{}),
		groupVersionLists: make(map[string]*groupVersionsCacheEntry),
		now:               time.Now,

		kubernetesAPIAddress: apiAddress,
	}
	c.initClient()
	return c
}

// newGetRequest builds a GET request against path with a bounded requestTimeout, so
// callers fail fast on a slow/unreachable apiserver instead of blocking on the much
// larger transport-level dial/handshake timeouts. The caller must call the returned
// cancel func once done with the request.
func (c *NamespacedDiscoveryCache) newGetRequest(path string) (*http.Request, context.CancelFunc, error) {
	return c.newGetRequestWithContext(context.Background(), path)
}

// newGetRequestWithContext is newGetRequest for a caller that has a context of its own - a probe,
// or an authorization request - so the work stops when the caller stops waiting.
func (c *NamespacedDiscoveryCache) newGetRequestWithContext(parent context.Context, path string) (*http.Request, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(parent, requestTimeout)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.kubernetesAPIAddress+path, nil)
	if err != nil {
		cancel()
		return nil, nil, err
	}

	return req, cancel, nil
}

// Check asks the API server whether it is there. One attempt, bounded by the caller's context.
//
// It used to retry: ten attempts of a two-second request with sleeps between them, about twenty-one
// seconds in all, from a readiness handler whose client gives up after four and whose probe fires
// every few seconds. So the one moment the check matters - the API server being unreachable - was
// the moment it accumulated overlapping goroutines, each holding a connection to the API server
// that is already struggling, for answers nobody is waiting for any more. Retrying a liveness
// question is the wrong shape anyway: the probe itself is the retry.
func (c *NamespacedDiscoveryCache) Check(ctx context.Context) error {
	req, cancel, err := c.newGetRequestWithContext(ctx, "/version")
	if err != nil {
		return fmt.Errorf("check Kubernetes API create request: %w", err)
	}
	defer cancel()

	return c.execRequest(req, "check API", nil)
}

func (c *NamespacedDiscoveryCache) initClient() {
	tlsConfig := &tls.Config{}

	contentCA, err := os.ReadFile(caPath)
	if err == nil {
		caPool, err := x509.SystemCertPool()
		if err != nil {
			panic(fmt.Errorf("cannot get system cert pool: %v", err))
		}

		caPool.AppendCertsFromPEM(contentCA)
		tlsConfig.RootCAs = caPool
	} else {
		c.logger.Printf("%v: not in pod?", err)
	}

	baseTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       tlsConfig,
	}

	c.client = &http.Client{Transport: wrapKubeTransport(baseTransport)}
}

func (c *NamespacedDiscoveryCache) renewCacheOnce(apiGroup string, req *http.Request) error {
	var groupedResp Response
	if err := c.execRequest(req, "renew namespaced cache", &groupedResp); err != nil {
		return err
	}

	cache := newNamespacedCacheEntry(c.now())
	for _, resource := range groupedResp.Resources {
		cache.Data[resource.Name] = resource.Namespaced
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[apiGroup] = cache
	return nil
}

// groupVersions is the group's list of served versions, newest first, cached for the same hour as
// the resource listings.
//
// Without the cache every question about a resource nobody serves paid for this listing again, and
// the resource name comes out of the request path - so a subject a rule covers could name a new one
// on every request and drive a listing each time. It is the group that is cached and not the
// question, so a new resource name in a group already listed costs nothing.
//
// Both outcomes are remembered, not only the successes. A group the API server 404s produces no
// list to cache, so without a negative the repeated identical question - the same unknown group and
// the same resource name - paid for a round trip every time, while the varying one was already
// bounded. The failures go in the same bounded negative map Get uses, keyed by the listing's own
// request path, because a listing of the group is a different question from any of its versions.
func (c *NamespacedDiscoveryCache) groupVersions(apiGroup string) ([]string, error) {
	c.muGroups.Lock()
	if entry, ok := c.groupVersionLists[apiGroup]; ok && !c.isEntryExpired(entry.cacheEntry) {
		versions := entry.Versions
		c.muGroups.Unlock()
		return versions, nil
	}
	c.muGroups.Unlock()

	// What a recent listing of the group itself concluded, answered without another round trip.
	// The two conclusions differ the way they do in Get: a group the API server says it does not
	// serve is an answer, and lets RBAC reply; a listing that failed is not, and denies.
	if absent, known := c.recentNegative(groupListingKey(apiGroup)); known {
		if absent {
			return nil, fmt.Errorf("api group %s is not served: %w", apiGroup, ErrNotFound)
		}
		return nil, fmt.Errorf("api group %s could not be listed recently", apiGroup)
	}

	versions, err := c.availableAPIGroupVersionsInDescendingOrder(apiGroup)
	if err != nil {
		c.noteNegative(groupListingKey(apiGroup), errors.Is(err, ErrNotFound))
		return nil, err
	}

	c.clearNegative(groupListingKey(apiGroup))

	c.muGroups.Lock()
	if c.groupVersionLists == nil {
		c.groupVersionLists = make(map[string]*groupVersionsCacheEntry)
	}
	c.groupVersionLists[apiGroup] = &groupVersionsCacheEntry{
		cacheEntry: newCacheEntry(c.now()),
		Versions:   versions,
	}
	c.muGroups.Unlock()

	return versions, nil
}

// groupListingKey is how a listing of the group itself is keyed in the negative map it shares with
// the group/version listings Get records.
//
// A group name carries no slash, so it can never be read as a group/version - but it can be read as
// a bare version, which is how the core group is keyed ("v1"), and a group name is a DNS subdomain,
// so one may legitimately be spelled that way. The listing's own request path is the unambiguous
// name for this question, so that is the key.
func groupListingKey(apiGroup string) string {
	return "/apis/" + apiGroup
}

func (c *NamespacedDiscoveryCache) availableAPIGroupVersionsInDescendingOrder(apiGroup string) ([]string, error) {
	req, cancel, err := c.newGetRequest("/apis/" + apiGroup)
	if err != nil {
		return nil, fmt.Errorf("build request for available apigroup versions: %w", err)
	}
	defer cancel()

	var apiGroupVersions APIGroupResponse
	if err := c.execRequest(req, "request available apigroup versions", &apiGroupVersions); err != nil {
		return nil, fmt.Errorf("request available apigroup versions: %w", err)
	}

	availableVersions := make([]string, 0, len(apiGroupVersions.Versions))
	for _, v := range apiGroupVersions.Versions {
		availableVersions = append(availableVersions, v.Version)
	}

	slices.SortFunc(availableVersions, func(v1, v2 string) int {
		return -(k8sversion.CompareKubeAwareVersionStrings(v1, v2))
	})

	return availableVersions, nil
}

// requestPreferredVersion finds the newest version of the group that serves the resource.
//
// It asks Get, which is the same question one group/version at a time, so the listings come out of
// the hourly cache that path already fills and the rate limit that protects it is the same one.
// Resolving by listing the versions here again meant a second, unbounded copy of that path: the
// listings were not cached, and the answers were remembered under a key carrying the resource name
// - in the same bounded map as the group keys, so cycling resource names evicted the memory that
// bounds Get.
func (c *NamespacedDiscoveryCache) requestPreferredVersion(group, resource string) (string, error) {
	availableVersions, err := c.groupVersions(group)
	if err != nil {
		return "", fmt.Errorf("get available apigroup versions: %w", err)
	}

	for _, version := range availableVersions {
		apiGroup := group + "/" + version
		if group == "" {
			apiGroup = version
		}

		switch _, err := c.Get(apiGroup, resource); {
		case err == nil:
			return version, nil
		case errors.Is(err, ErrResourceAbsent), errors.Is(err, ErrNotFound):
			// This version does not serve it, or has gone away since the group was listed. Neither
			// says anything about the other versions.
			continue
		default:
			return "", fmt.Errorf("get preferred version: %w", err)
		}
	}

	// Every version of the group answered and none of them serves this resource. That is an
	// answer, and it has to say so: the caller lets RBAC reply to an absent resource, and the API
	// server then produces the 404 the caller is owed, instead of a 403 about a resource that was
	// never there. This used to be a plain error, which denies - and the reviews that take this
	// path are the ones with no version in them, which is what `kubectl auth can-i` sends, so the
	// wrong answer landed in the middle of the tool people debug with.
	return "", fmt.Errorf("no version of api group %s serves %s: %w", group, resource, ErrResourceAbsent)
}

func (c *NamespacedDiscoveryCache) preferredVersionFromCache(group, resource string) string {
	c.muPv.RLock()
	defer c.muPv.RUnlock()

	entry, ok := c.preferredVersions[fmt.Sprintf("%s.%s", resource, group)]

	if ok && !c.isEntryExpired(entry.cacheEntry) {
		return entry.Version
	}

	return ""
}

// GetPreferredVersion resolves the newest version of the group that serves the resource, for the
// reviews that arrive without one.
//
// The work it does is bounded by what it asks: the group's versions and each version's resource
// listing are cached for an hour, and both are shared with Get. So the first question about a group
// costs one listing per version and every question after it - about any resource, existing or not -
// is answered from what is already held. There is no memo of its own to keep: a resolved version is
// cached below, and an unresolved one costs nothing to conclude again.
func (c *NamespacedDiscoveryCache) GetPreferredVersion(group, resource string) (string, error) {
	if version := c.preferredVersionFromCache(group, resource); version != "" {
		return version, nil
	}

	version, err := c.requestPreferredVersion(group, resource)
	if err != nil {
		return "", err
	}

	c.muPv.Lock()
	defer c.muPv.Unlock()
	c.preferredVersions[fmt.Sprintf("%s.%s", resource, group)] = newPreferredVersionCacheEntry(c.now(), version)

	return version, nil
}

func (c *NamespacedDiscoveryCache) getFromCache(apiGroup string) (*namespacedCacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[apiGroup]
	return entry, ok
}

func (c *NamespacedDiscoveryCache) Get(apiGroup, resource string) (bool, error) {
	namespacedInfo, ok := c.getFromCache(apiGroup)

	// A recent listing of this group already concluded something. Answered from the bounded
	// negative cache rather than from another round trip.
	if !ok {
		if absent, known := c.recentNegative(apiGroup); known {
			if absent {
				return false, fmt.Errorf("api group %s is not served: %w", apiGroup, ErrResourceAbsent)
			}
			return false, fmt.Errorf("api group %s could not be listed recently", apiGroup)
		}
	}

	switch {
	case !ok:
		// The group has never been listed. One attempt, not the retry loop: this is the
		// authorization path, the caller chose the group, and the API server gives the whole
		// webhook three seconds before it gives up and denies - retries past that only pile up
		// work for an answer nobody is waiting for.
		if err := c.listOnce(apiGroup); err != nil {
			// Both outcomes are remembered, and for the same reason: the caller picks the group out
			// of the request path, so without a record every request for one would produce a fresh
			// round trip. What differs is the answer - a group the API server says it does not
			// serve lets RBAC reply and the request 404s, a listing that failed denies.
			c.noteNegative(apiGroup, errors.Is(err, ErrNotFound))
			if errors.Is(err, ErrNotFound) {
				return false, fmt.Errorf("api group %s is not served: %w", apiGroup, ErrResourceAbsent)
			}
			return false, err
		}

		namespacedInfo, ok = c.getFromCache(apiGroup)
		if !ok {
			// A concurrent listing concluded the group is not there, and recorded it.
			if absent, known := c.recentNegative(apiGroup); known && absent {
				return false, fmt.Errorf("api group %s is not served: %w", apiGroup, ErrResourceAbsent)
			}
			return false, fmt.Errorf("api group %s was not listed", apiGroup)
		}
	case c.isEntryExpired(namespacedInfo.cacheEntry):
		// The hourly TTL has passed. One attempt, like every other listing on this path: the retry
		// loop this used to use took up to twenty seconds, and the API server gives the whole
		// webhook three before it denies - so the retries only produced work for an answer that
		// had already been decided without it. A failure here is harmless anyway, because a stale
		// listing is still an answer.
		if err := c.listOnce(apiGroup); err != nil {
			c.logger.Println(err)
		} else if refreshed, found := c.getFromCache(apiGroup); found {
			namespacedInfo = refreshed
		}
	}

	// The group is listed but does not carry the resource. It may have been installed since the
	// listing, so list the group again - but not more often than negativeRenewInterval, and
	// without the retry loop. This is the authorization path and the caller chooses the resource
	// name, so both the rate and the duration of the work have to be bounded.
	namespaced, ok := namespacedInfo.Data[resource]
	if !ok {
		// Two clocks bound the refresh, and both are needed. The age of the listing is the normal
		// one. The memory of a failed attempt is the other: this branch keys on the age of the
		// last SUCCESSFUL listing, which a failure does not move, so while the API server was
		// unreachable every request naming an absent resource in a known group produced another
		// attempt against it - the failure was recorded and never read here. The caller writes the
		// resource name, so any subject a rule covers can pick one that is absent.
		_, attemptedRecently := c.recentNegative(apiGroup)
		if !attemptedRecently && c.now().Sub(namespacedInfo.AddTime) >= negativeRenewInterval {
			if err := c.listOnce(apiGroup); err != nil {
				// Remembered either way, so a group whose listing keeps failing is not re-attempted
				// on every request for the duration of the failure.
				c.noteNegative(apiGroup, errors.Is(err, ErrNotFound))
				// The group stopped being served since it was listed. That is an answer, not a
				// failure to ask.
				if errors.Is(err, ErrNotFound) {
					c.forget(apiGroup)
					return false, fmt.Errorf("api group %s is not served: %w", apiGroup, ErrResourceAbsent)
				}
				// Anything else is a failure to refresh, and it must not throw away what we
				// already know: a listing of this group succeeded once and did not carry the
				// resource. Returning the error instead would deny, which is the 403-for-a-
				// resource-that-does-not-exist this change exists to remove, brought back for the
				// duration of every API server blip. The refresh only guards against a resource
				// installed since that listing, and a resource cannot be installed while the API
				// server cannot be reached, so nothing is lost by keeping the negative.
				c.logger.Printf("could not re-list %s to confirm that %s is absent, using the previous listing: %v", apiGroup, resource, err)
			} else if refreshed, found := c.getFromCache(apiGroup); found {
				namespacedInfo = refreshed
				namespaced, ok = namespacedInfo.Data[resource]
			}
			// found being false means a concurrent lookup saw a 404 for the group and dropped the
			// entry. Reading Data off the nil that getFromCache returns then panics, and this used
			// to discard the second return value. The answer is the same either way - the resource
			// is not there - so falling through with what we already hold is right.
		}
		if !ok {
			// A listing of the group succeeded and does not carry the resource, so this is an
			// answer, not a failure to ask.
			return false, fmt.Errorf("resource %s/%s is not found in cluster: %w", apiGroup, resource, ErrResourceAbsent)
		}
	}

	return namespaced, nil
}

// negativeEntry is what a listing that produced no answer left behind.
type negativeEntry struct {
	at time.Time
	// absent distinguishes the two: the API server answered and said it does not serve this group,
	// or the attempt failed and we know nothing. The first lets RBAC answer and the request 404s;
	// the second denies.
	absent bool
}

// noteNegative remembers that a listing of this group did not produce an answer, so the lookups
// that follow are answered without another round trip until negativeRenewInterval has passed.
//
// Recording the FAILED attempts too is the half that was missing. Without it, a group whose listing
// keeps failing was retried on every single request - the rate limit only ever applied to groups
// that had answered - so an API server having a bad minute turned every authorization request into
// another attempt against it.
//
// The map is capped. When it is full, entries are dropped until there is room again: everything
// expired first, then the oldest of a bounded sample. Go's map iteration order is random, so the
// sample is a fair one, and evicting the wrong entry costs at most one extra round trip.
func (c *NamespacedDiscoveryCache) noteNegative(apiGroup string, absent bool) {
	now := c.now()

	c.muNegative.Lock()
	defer c.muNegative.Unlock()

	if c.negative == nil {
		c.negative = make(map[string]negativeEntry)
	}

	if len(c.negative) >= maxUnservedGroups {
		for group, e := range c.negative {
			if now.Sub(e.at) >= negativeRenewInterval {
				delete(c.negative, group)
			}
		}
		for len(c.negative) >= maxUnservedGroups {
			oldest, oldestAt, seen := "", now, 0
			for group, e := range c.negative {
				if oldest == "" || e.at.Before(oldestAt) {
					oldest, oldestAt = group, e.at
				}
				if seen++; seen >= 16 {
					break
				}
			}
			delete(c.negative, oldest)
		}
	}

	c.negative[apiGroup] = negativeEntry{at: now, absent: absent}
}

// recentNegative reports what a recent listing of this group concluded, if there was one: the
// first result says the group is not served, the second whether there was such a conclusion at all.
func (c *NamespacedDiscoveryCache) recentNegative(apiGroup string) (bool, bool) {
	c.muNegative.Lock()
	defer c.muNegative.Unlock()
	e, ok := c.negative[apiGroup]
	if !ok {
		return false, false
	}
	if c.now().Sub(e.at) >= negativeRenewInterval {
		delete(c.negative, apiGroup)
		return false, false
	}
	return e.absent, true
}

// clearNegative forgets what a previous listing of this group concluded, because a later one
// answered.
func (c *NamespacedDiscoveryCache) clearNegative(apiGroup string) {
	c.muNegative.Lock()
	defer c.muNegative.Unlock()
	delete(c.negative, apiGroup)
}

// listOnce lists a group, collapsing concurrent calls for the same group into one round trip.
//
// Without this, a burst of authorization requests naming the same unlisted group produced one
// listing each: the rate limit only applies once an attempt has finished, so everything that
// arrives while the first is in flight goes out on its own. The cost of the miss is multiplied by
// the concurrency of the authorization path, which is the whole cluster.
func (c *NamespacedDiscoveryCache) listOnce(apiGroup string) error {
	_, err := c.once(apiGroup, func() error {
		err := c.renewCacheOnceNoRetry(apiGroup)
		if err == nil {
			// The group answers again, so what a previous attempt concluded about it is history.
			// Left in place it would suppress the next refresh for up to an interval after a
			// listing that actually succeeded.
			c.clearNegative(apiGroup)
		}
		return err
	})
	// A caller that waited for somebody else's listing gets no error: whatever that listing
	// concluded is recorded now, and the caller re-reads the caches to see it.
	return err
}

// once runs fn unless an identical call is already in flight, in which case it waits for that one
// and returns false - the caller then reads whatever fn recorded rather than asking again. The
// first result says whether fn ran here, the second is its error.
func (c *NamespacedDiscoveryCache) once(key string, fn func() error) (bool, error) {
	c.muInflight.Lock()
	if c.inflight == nil {
		c.inflight = make(map[string]chan struct{})
	}
	if done, running := c.inflight[key]; running {
		c.muInflight.Unlock()
		<-done
		return false, nil
	}
	done := make(chan struct{})
	c.inflight[key] = done
	c.muInflight.Unlock()

	err := fn()

	c.muInflight.Lock()
	delete(c.inflight, key)
	c.muInflight.Unlock()
	close(done)

	return true, err
}

// renewCacheOnceNoRetry lists a group exactly once. renewCache retries for up to twenty seconds,
// which is right for a cold start and wrong on the authorization path: the API server gives the
// webhook three seconds and then denies, so the retries only pile up work for requests whose
// answer nobody is waiting for any more.
func (c *NamespacedDiscoveryCache) renewCacheOnceNoRetry(apiGroup string) error {
	path := apiV1Path
	if apiGroup != "v1" {
		path = "/apis/" + apiGroup
	}

	req, cancel, err := c.newGetRequest(path)
	if err != nil {
		return fmt.Errorf("renew cache prepare request: %w", err)
	}
	defer cancel()

	return c.renewCacheOnce(apiGroup, req)
}

// forget drops a group that used to be served. Its entry in data would otherwise keep answering
// from a listing of a group that no longer exists.
func (c *NamespacedDiscoveryCache) forget(apiGroup string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, apiGroup)
}

func (c *NamespacedDiscoveryCache) isEntryExpired(e *cacheEntry) bool {
	return c.now().After(e.AddTime.Add(e.TTL))
}

func (c *NamespacedDiscoveryCache) execRequest(req *http.Request, logTag string, result interface{}) error {
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: requesting error: %w", logTag, err)
	}

	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: decoding response error: %w", logTag, err)
	}

	if resp.StatusCode == http.StatusNotFound {
		// Wrapped like every other error here, so a log line says which request 404'd. Callers
		// match it with errors.Is, which the wrapping preserves.
		return fmt.Errorf("%s: %s: %w", logTag, req.URL.Path, ErrNotFound)
	}

	if resp.StatusCode/100 > 2 {
		return fmt.Errorf("%s: kube response error: %d %s", logTag, resp.StatusCode, respBody)
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("%s: failed to unmarshal response: %w", logTag, err)
		}
	}

	return nil
}

// Resource is a single entry of the /apis/.../... endpoint response.
type Resource struct {
	Name       string `json:"name"`
	Namespaced bool   `json:"namespaced"`
}

// Response is a /apis/.../... endpoint response.
type Response struct {
	Resources []Resource `json:"resources"`
}

type PreferredVersion struct {
	// groupVersion specifies the API group and version in the form "group/version"
	GroupVersion string `json:"groupVersion"`
	// version specifies the version in the form of "version". This is to save
	// the clients the trouble of splitting the GroupVersion.
	Version string `json:"version"`
}

type APIGroupResponse struct {
	Versions []APIGroupVersion `json:"versions"`
}

type APIGroupVersion struct {
	Version string `json:"version"`
}

type APIResourceList struct {
	Resources []Resource `json:"resources"`
}

func (l *APIResourceList) Has(resource string) bool {
	for _, res := range l.Resources {
		if res.Name == resource {
			return true
		}
	}

	return false
}
