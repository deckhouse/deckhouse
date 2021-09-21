/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cache

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	renewTokenPeriod = 30 * time.Second

	defaultTTL = 1 * time.Hour

	saPath    = "/var/run/secrets/kubernetes.io/serviceaccount/"
	caPath    = saPath + "ca.crt"
	tokenPath = saPath + "token"

	kubernetesAPIAddress = "https://kubernetes.default"
)

type Cache interface {
	Get(string, string) (bool, error)
}

var _ Cache = (*NamespacedDiscoveryCache)(nil)

type cacheEntry struct {
	TTL     time.Duration
	AddTime time.Time

	Data map[string]bool
}

func newCacheEntry(addTime time.Time) *cacheEntry {
	return &cacheEntry{
		AddTime: addTime,
		Data:    make(map[string]bool),
		TTL:     defaultTTL,
	}
}

type NamespacedDiscoveryCache struct {
	logger *log.Logger

	client *http.Client

	mu   sync.RWMutex
	data map[string]*cacheEntry

	now func() time.Time

	kubernetesAPIAddress string
}

func NewNamespacedDiscoveryCache(logger *log.Logger) *NamespacedDiscoveryCache {
	c := &NamespacedDiscoveryCache{
		logger: logger,
		data:   make(map[string]*cacheEntry),
		now:    time.Now,

		kubernetesAPIAddress: kubernetesAPIAddress,
	}
	c.initClient()
	return c
}

func (c *NamespacedDiscoveryCache) initClient() {
	tlsConfig := &tls.Config{}

	contentCA, err := ioutil.ReadFile(caPath)
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
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("renew cache requesting: %w", err)
	}

	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("renew cache decoding response: %w", err)
	}

	if resp.StatusCode/100 > 2 {
		return fmt.Errorf("kube response error: %s", respBody)
	}

	var groupedResp Response
	err = json.Unmarshal(respBody, &groupedResp)
	if err != nil {
		return fmt.Errorf("renew cache decoding response: %w", err)
	}

	cache := newCacheEntry(c.now())
	for _, resource := range groupedResp.Resources {
		cache.Data[resource.Name] = resource.Namespaced
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[apiGroup] = cache
	return nil
}

func (c *NamespacedDiscoveryCache) renewCache(apiGroup string) error {
	path := "/api/v1"
	if apiGroup != "v1" {
		path = "/apis/" + apiGroup
	}

	return Retry(func() (bool, error) {
		req, err := http.NewRequest(http.MethodGet, c.kubernetesAPIAddress+path, nil)
		if err != nil {
			return false, fmt.Errorf("renew cache prepare request: %w", err)
		}

		if err := c.renewCacheOnce(apiGroup, req); err != nil {
			return true, err
		}

		return false, nil
	})
}

func (c *NamespacedDiscoveryCache) getFromCache(apiGroup string) (*cacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[apiGroup]
	return entry, ok
}

func (c *NamespacedDiscoveryCache) Get(apiGroup, resource string) (bool, error) {
	namespacedInfo, ok := c.getFromCache(apiGroup)

	switch {
	case !ok:
		// there is no cache, renew
		if err := c.renewCache(apiGroup); err != nil {
			return false, err
		}

		namespacedInfo, _ = c.getFromCache(apiGroup)
	case c.now().After(namespacedInfo.AddTime.Add(namespacedInfo.TTL)):
		// cache is expired
		if err := c.renewCache(apiGroup); err != nil {
			// if there is an error, we could just use stale cache
			c.logger.Println(err)
		} else {
			namespacedInfo, _ = c.getFromCache(apiGroup)
		}
	}

	// if cache for api group exists but there is no resource, we should update the cache entry for the whole group
	namespaced, ok := namespacedInfo.Data[resource]
	if !ok {
		if err := c.renewCache(apiGroup); err != nil {
			return false, err
		}

		namespacedInfo, _ = c.getFromCache(apiGroup)

		namespaced, ok = namespacedInfo.Data[resource]
		if !ok {
			return false, fmt.Errorf("resource %s/%s is not found in cluster", apiGroup, resource)
		}
	}

	return namespaced, nil
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
