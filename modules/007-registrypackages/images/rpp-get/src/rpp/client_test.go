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

package rpp

import "testing"

func TestNewFetcherSelectsBackend(t *testing.T) {
	direct := newFetcher(Config{RegistryDirect: true, RegistryRepo: "registry.local/repo", RegistryAuth: "x"})
	if _, ok := direct.(*directClient); !ok {
		t.Fatalf("RegistryDirect=true: got %T, want *directClient", direct)
	}

	proxy := newFetcher(Config{Endpoints: []string{"1.2.3.4:4219"}})
	if _, ok := proxy.(*httpClient); !ok {
		t.Fatalf("RegistryDirect=false: got %T, want *httpClient", proxy)
	}
}

func TestUpdateAuthIsNoOpInDirectMode(t *testing.T) {
	c := NewClient(Config{RegistryDirect: true, RegistryRepo: "registry.local/repo", RegistryAuth: "x"}, nil, nil)
	before := c.fetcher

	c.UpdateAuth([]string{"5.6.7.8:4219"}, "tok")

	if c.fetcher != before {
		t.Error("UpdateAuth replaced the fetcher in direct mode, want unchanged")
	}
	if _, ok := c.fetcher.(*directClient); !ok {
		t.Fatalf("fetcher = %T, want *directClient", c.fetcher)
	}
}
