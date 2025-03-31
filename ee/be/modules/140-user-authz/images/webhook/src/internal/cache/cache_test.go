/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCacheRenew(t *testing.T) {
	cache := newTestCache()

	err := cache.renewCache("test")
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

func TestCacheCoreResources(t *testing.T) {
	cache := newTestCoreResourcesCache()

	coreResources := cache.getCoreResourcesFromCache()
	if len(coreResources) != 0 {
		t.Fatalf("cache is not empty")
	}

	coreResources, err := cache.GetCoreResources()
	if err != nil {
		t.Fatal(err)
	}

	var apiResourceList APIResourceList
	err = json.Unmarshal([]byte(coreResourcesResponse), &apiResourceList)
	if err != nil {
		t.Fatal(err)
	}

	expectedCoreResources := make(CoreResourcesDict, len(apiResourceList.Resources))
	for _, resource := range apiResourceList.Resources {
		expectedCoreResources[getResourceNameBeforeSlash(resource.Name)] = struct{}{}
	}

	if fmt.Sprint(expectedCoreResources) != fmt.Sprint(coreResources) {
		t.Fatal("received list of core resources doesn't match the expected one")
	}

	coreResources = cache.getCoreResourcesFromCache()
	if len(coreResources) == 0 {
		t.Fatalf("cache wasn't populated")
	}

	now := cache.now()
	// change client here to not be able to update the cache
	cache.now = func() time.Time { return now.Add(time.Hour * 3) }

	coreResources = cache.getCoreResourcesFromCache()
	if len(coreResources) != 0 {
		t.Fatalf("cache is not expired")
	}

	coreResources, err = cache.GetCoreResources()
	if err != nil {
		t.Fatal(err)
	}

	if fmt.Sprint(expectedCoreResources) != fmt.Sprint(coreResources) {
		t.Fatal("received list of core resources doesn't match the expected one after expiration")
	}
}

func TestCacheGetIfNoResource(t *testing.T) {
	cache := newTestCache()

	err := cache.renewCache("test")
	if err != nil {
		t.Fatal(err)
	}

	delete(cache.data["test"].Data, "nodes")

	namespaced, err := cache.Get("test", "nodes")
	if err != nil {
		t.Fatal(err)
	}

	if namespaced != false {
		t.Fatalf("nodes namespaced: %v != %v", namespaced, false)
	}
}

func TestCacheStale(t *testing.T) {
	cache := newTestCache()

	err := cache.renewCache("test")
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

	expectedErr := "check API: kube response error: 500 ERROR: exceeded retry limit"

	err := cache.Check()
	if err.Error() != expectedErr {
		t.Fatalf("%q received, expected %q", err.Error(), expectedErr)
	}

	server = newTestServer()
	cache.client = server.Client()
	cache.kubernetesAPIAddress = server.URL

	err = cache.Check()
	if err != nil {
		t.Fatalf("%q received, expected nil", err.Error())
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
	cache.coreResources = new(coreResourcesCache)

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
