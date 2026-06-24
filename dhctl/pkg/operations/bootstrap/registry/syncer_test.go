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

package registry

import "testing"

// TestBootstrapCacheIdentifiers pins the names DeleteBootstrapCache tears down to
// what SetupBootstrapCache (bootstrap_cache.go) creates.
func TestBootstrapCacheIdentifiers(t *testing.T) {
	if bootstrapCacheNamespace != "d8-system" {
		t.Fatalf("bootstrapCacheNamespace = %q, want d8-system", bootstrapCacheNamespace)
	}
	wantPods := map[string]bool{
		"registry-bootstrap-cache":      true,
		"registry-bootstrap-cache-fill": true,
	}
	if len(bootstrapCachePods) != len(wantPods) {
		t.Fatalf("bootstrapCachePods = %v, want keys %v", bootstrapCachePods, wantPods)
	}
	for _, p := range bootstrapCachePods {
		if !wantPods[p] {
			t.Fatalf("unexpected bootstrap cache pod %q", p)
		}
	}

	wantSecrets := map[string]bool{
		"registry-bootstrap-cache-pki":    true,
		"registry-bootstrap-cache-config": true,
		"registry-bootstrap-cache-fill":   true,
	}
	if len(bootstrapCacheSecrets) != len(wantSecrets) {
		t.Fatalf("bootstrapCacheSecrets = %v, want keys %v", bootstrapCacheSecrets, wantSecrets)
	}
	for _, s := range bootstrapCacheSecrets {
		if !wantSecrets[s] {
			t.Fatalf("unexpected bootstrap cache secret %q", s)
		}
	}
}
