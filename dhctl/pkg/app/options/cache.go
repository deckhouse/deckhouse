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

package options

// Allowed values for CacheOptions.UseTfCache.
const (
	UseStateCacheAsk = "ask"
	UseStateCacheYes = "yes"
	UseStateCacheNo  = "no"
)

// CacheOptions groups infrastructure (terraform/tofu) state cache settings.
type CacheOptions struct {
	Dir        string
	UseTfCache string
	DropCache  bool

	KubeConfig          string
	KubeConfigContext   string
	KubeConfigInCluster bool
	KubeNamespace       string
	KubeName            string
	KubeLabels          map[string]string

	// ResourceManagementTimeout overrides infrastructure resource-management
	// timeouts (string passed unparsed to the underlying tooling).
	ResourceManagementTimeout string
}

// NewCacheOptions returns CacheOptions with defaults.
func NewCacheOptions() CacheOptions {
	return CacheOptions{
		Dir:        DefaultTmpDir(),
		UseTfCache: UseStateCacheAsk,
		KubeLabels: make(map[string]string),
	}
}
