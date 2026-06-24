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

package moduleconfig

import (
	"testing"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

func TestCleanSettingsValidate(t *testing.T) {
	cases := []struct {
		name    string
		in      CleanSettings
		wantErr bool
	}{
		{"air-gap: cache, no upstream", CleanSettings{Cache: CacheSettings{Enabled: true, StorageSize: "20Gi"}}, false},
		{"connected+cache", CleanSettings{Cache: CacheSettings{Enabled: true, StorageSize: "20Gi"}, Upstream: &UpstreamSettings{Host: "r.io"}}, false},
		{"direct: no cache, upstream", CleanSettings{Cache: CacheSettings{Enabled: false}, Upstream: &UpstreamSettings{Host: "r.io"}}, false},
		{"INVALID: no cache, no upstream", CleanSettings{Cache: CacheSettings{Enabled: false}}, true},
		{"INVALID: cache without storageSize", CleanSettings{Cache: CacheSettings{Enabled: true}}, true},
		{"INVALID: upstream without host", CleanSettings{Cache: CacheSettings{Enabled: false}, Upstream: &UpstreamSettings{Scheme: constant.SchemeHTTPS}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.in.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestRegistryModuleConfigIsUnmanaged(t *testing.T) {
	f := false
	tr := true
	if !(RegistryModuleConfig{Enabled: &f}).IsUnmanaged() {
		t.Fatal("enabled:false must be unmanaged")
	}
	if (RegistryModuleConfig{Enabled: &tr}).IsUnmanaged() {
		t.Fatal("enabled:true must not be unmanaged")
	}
	if (RegistryModuleConfig{Enabled: nil}).IsUnmanaged() {
		t.Fatal("nil enabled defaults to managed")
	}
}
