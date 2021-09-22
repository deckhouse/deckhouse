/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package cache

import (
	"io/ioutil"
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

func newTestCache() *NamespacedDiscoveryCache {
	server := newTestServer()

	cache := NamespacedDiscoveryCache{}

	cache.client = server.Client()
	cache.kubernetesAPIAddress = server.URL

	now := time.Now()
	cache.now = func() time.Time { return now }

	cache.logger = log.New(ioutil.Discard, "", log.LstdFlags)
	cache.data = make(map[string]*cacheEntry)

	server.Config.ErrorLog = cache.logger

	return &cache
}

func newTestServer() *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		w.Write([]byte(testResponse))
	}))
}

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
