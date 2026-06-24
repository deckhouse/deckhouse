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

package proxy

import "testing"

func TestRewriteRepoPath(t *testing.T) {
	cases := []struct {
		name                string
		path, alias, remote string
		want                string
	}{
		{"alias to remote", "/v2/system/deckhouse/module/manifests/v1", "system/deckhouse", "deckhouse/ee", "/v2/deckhouse/ee/module/manifests/v1"},
		{"alias exact", "/v2/system/deckhouse/manifests/v1", "system/deckhouse", "deckhouse/ee", "/v2/deckhouse/ee/manifests/v1"},
		{"empty alias prepends remote", "/v2/library/nginx/manifests/v1", "", "mirror/dockerhub", "/v2/mirror/dockerhub/library/nginx/manifests/v1"},
		{"empty remote strips alias", "/v2/system/deckhouse/x/blobs/sha", "system/deckhouse", "", "/v2/x/blobs/sha"},
		{"no v2 prefix untouched", "/healthz", "system/deckhouse", "deckhouse/ee", "/healthz"},
		{"v2 root untouched", "/v2/", "system/deckhouse", "deckhouse/ee", "/v2/"},
		{"alias not matched untouched", "/v2/other/repo/manifests/v1", "system/deckhouse", "deckhouse/ee", "/v2/other/repo/manifests/v1"},
		{"empty alias empty remote untouched", "/v2/library/nginx/manifests/v1", "", "", "/v2/library/nginx/manifests/v1"},
		{"slashes trimmed", "/v2/system/deckhouse/m/manifests/v1", "/system/deckhouse/", "/deckhouse/ee/", "/v2/deckhouse/ee/m/manifests/v1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rewriteRepoPath(c.path, c.alias, c.remote); got != c.want {
				t.Fatalf("rewriteRepoPath(%q,%q,%q) = %q, want %q", c.path, c.alias, c.remote, got, c.want)
			}
		})
	}
}
