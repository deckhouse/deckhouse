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
	// cluster-wide until the next listing. Installing a CRD already requires far more privilege
	// than that yields, and the platform tolerates comparable windows elsewhere (the API server
	// caches this webhook's answers for 30 seconds), so the trade is deliberate: a bounded,
	// privilege-gated staleness window in exchange for closing an unbounded amplifier that any
	// tenant can drive.
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

	// muUnserved guards unserved, which remembers the API groups the API server answered 404 for.
	// It is kept apart from data, and bounded, because its keys are attacker-chosen.
	muUnserved sync.Mutex
	unserved   map[string]time.Time

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
		unserved:          make(map[string]time.Time),
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

func (c *NamespacedDiscoveryCache) renewCache(apiGroup string) error {
	path := apiV1Path
	if apiGroup != "v1" {
		path = "/apis/" + apiGroup
	}

	return Retry(func() (bool, error) {
		req, cancel, err := c.newGetRequest(path)
		if err != nil {
			return false, fmt.Errorf("renew cache prepare request: %w", err)
		}
		defer cancel()

		if err := c.renewCacheOnce(apiGroup, req); err != nil {
			return true, err
		}

		return false, nil
	})
}

func (c *NamespacedDiscoveryCache) getAvailableAPIGroupVerionsInDescendingOrder(apiGroup string) ([]string, error) {
	path := "/apis/" + apiGroup

	availableVersions := make([]string, 0)

	err := Retry(func() (bool, error) {
		req, cancel, err := c.newGetRequest(path)
		if err != nil {
			return false, fmt.Errorf("build request for available apigroup versions: %w", err)
		}
		defer cancel()

		var apiGroupVersions APIGroupResponse
		if err = c.execRequest(req, "request available apigroup versions", &apiGroupVersions); err != nil {
			return true, fmt.Errorf("request available apigroup verions: %w", err)
		}

		for _, v := range apiGroupVersions.Versions {
			availableVersions = append(availableVersions, v.Version)
		}

		return false, nil
	})

	slices.SortFunc(availableVersions, func(v1, v2 string) int {
		return -(k8sversion.CompareKubeAwareVersionStrings(v1, v2))
	})

	return availableVersions, err
}

func (c *NamespacedDiscoveryCache) requestPreferredVersion(group, resource string) (string, error) {
	preferredVersion := ""

	availableVersions, err := c.getAvailableAPIGroupVerionsInDescendingOrder(group)
	if err != nil {
		return "", fmt.Errorf("get available apigroup versions: %w", err)
	}

	for _, version := range availableVersions {
		path := fmt.Sprintf("/apis/%s/%s", group, version)
		if err := Retry(func() (bool, error) {
			req, cancel, err := c.newGetRequest(path)
			if err != nil {
				return false, fmt.Errorf("request %s %s/%s version build error: %w", resource, group, version, err)
			}
			defer cancel()

			var apiResourceList APIResourceList
			if err = c.execRequest(req, "request list of resources", &apiResourceList); err != nil {
				return true, fmt.Errorf("request list of resources: %w", err)
			}

			if apiResourceList.Has(resource) {
				preferredVersion = version
			}

			return false, nil
		}); err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return "", fmt.Errorf("get preferred version: %w", err)
		}

		if preferredVersion != "" {
			break
		}
	}

	if preferredVersion == "" {
		return "", fmt.Errorf("failed to discover preferred version for %s.%s", resource, group)
	}

	return preferredVersion, nil
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

func (c *NamespacedDiscoveryCache) GetPreferredVersion(group, resource string) (string, error) {
	version := c.preferredVersionFromCache(group, resource)
	if version != "" {
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

	// A group the API server said it does not serve, recently enough to still believe it. Answered
	// from the bounded negative cache rather than from another round trip.
	if !ok && c.unservedRecently(apiGroup) {
		return false, fmt.Errorf("api group %s is not served: %w", apiGroup, ErrResourceAbsent)
	}

	switch {
	case !ok:
		// The group has never been listed. One attempt, not the retry loop: this is the
		// authorization path, the caller chose the group, and the API server gives the whole
		// webhook three seconds before it gives up and denies - retries past that only pile up
		// work for an answer nobody is waiting for.
		if err := c.renewCacheOnceNoRetry(apiGroup); err != nil {
			// A group the API server 404s has to be remembered as empty, or the rate limit below
			// never applies to it: the caller picks the group out of the request path, so an
			// unknown group is the cheapest way to ask for a listing, and without an entry every
			// request for one would produce a fresh round trip. An empty entry answers the same
			// way - the resource is not in it - while the interval bounds the re-listing.
			if errors.Is(err, ErrNotFound) {
				c.noteUnserved(apiGroup)
				return false, fmt.Errorf("api group %s is not served: %w", apiGroup, ErrResourceAbsent)
			}
			return false, err
		}

		namespacedInfo, _ = c.getFromCache(apiGroup)
	case c.isEntryExpired(namespacedInfo.cacheEntry):
		// cache is expired
		if err := c.renewCache(apiGroup); err != nil {
			// if there is an error, we could just use stale cache
			c.logger.Println(err)
		} else {
			namespacedInfo, _ = c.getFromCache(apiGroup)
		}
	}

	// The group is listed but does not carry the resource. It may have been installed since the
	// listing, so list the group again - but not more often than negativeRenewInterval, and
	// without the retry loop. This is the authorization path and the caller chooses the resource
	// name, so both the rate and the duration of the work have to be bounded.
	namespaced, ok := namespacedInfo.Data[resource]
	if !ok {
		if c.now().Sub(namespacedInfo.AddTime) >= negativeRenewInterval {
			if err := c.renewCacheOnceNoRetry(apiGroup); err != nil {
				// The group stopped being served since it was listed. That is an answer, not a
				// failure to ask, and it is remembered for the same reason as above.
				if errors.Is(err, ErrNotFound) {
					c.noteUnserved(apiGroup)
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
			} else {
				namespacedInfo, _ = c.getFromCache(apiGroup)
				namespaced, ok = namespacedInfo.Data[resource]
			}
		}
		if !ok {
			// A listing of the group succeeded and does not carry the resource, so this is an
			// answer, not a failure to ask.
			return false, fmt.Errorf("resource %s/%s is not found in cluster: %w", apiGroup, resource, ErrResourceAbsent)
		}
	}

	return namespaced, nil
}

// noteUnserved remembers that the API server 404s this group, so the lookups that follow are
// answered without another round trip until negativeRenewInterval has passed.
//
// The map is capped. When it is full, entries are dropped until there is room again - oldest first
// among those the scan sees, which is enough: every entry carries the same answer, so evicting the
// wrong one costs at most one extra round trip.
func (c *NamespacedDiscoveryCache) noteUnserved(apiGroup string) {
	now := c.now()

	c.muUnserved.Lock()
	defer c.muUnserved.Unlock()

	if c.unserved == nil {
		c.unserved = make(map[string]time.Time)
	}

	if len(c.unserved) >= maxUnservedGroups {
		// Drop everything that has expired anyway, and if that was not enough, the oldest of a
		// bounded sample. Go's map iteration order is random, so the sample is a fair one.
		for group, at := range c.unserved {
			if now.Sub(at) >= negativeRenewInterval {
				delete(c.unserved, group)
			}
		}
		for len(c.unserved) >= maxUnservedGroups {
			oldest, oldestAt, seen := "", now, 0
			for group, at := range c.unserved {
				if oldest == "" || at.Before(oldestAt) {
					oldest, oldestAt = group, at
				}
				if seen++; seen >= 16 {
					break
				}
			}
			delete(c.unserved, oldest)
		}
	}

	c.unserved[apiGroup] = now
}

// unservedRecently reports whether this group was answered 404 within the interval.
func (c *NamespacedDiscoveryCache) unservedRecently(apiGroup string) bool {
	c.muUnserved.Lock()
	defer c.muUnserved.Unlock()
	at, ok := c.unserved[apiGroup]
	if !ok {
		return false
	}
	if c.now().Sub(at) >= negativeRenewInterval {
		delete(c.unserved, apiGroup)
		return false
	}
	return true
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
		return ErrNotFound
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
