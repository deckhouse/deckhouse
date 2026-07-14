// Copyright 2024 Flant JSC
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

type ClientConfigGetter interface {
	// Get returns the registry client configuration for the given registry
	// repository (e.g. registry.DefaultRepository or a ModuleSource repo).
	Get(repository string) (*ClientConfig, error)

	// GetPackagesConfig returns the registry connection parameters for a
	// PackageRepository CR addressed by name. It is the dependency-inverted
	// replacement for the in-proxy k8sClient lookup that used to read
	// PackageRepository directly. Implementations are expected to translate the
	// CR's spec (registry repo, scheme, CA, dockerCfg) into a PackagesConfig.
	GetPackagesConfig(packageRepositoryName string) (*PackagesConfig, error)
}
