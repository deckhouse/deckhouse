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

type ClientConfig struct {
	Repository string
	Scheme     string
	CA         string
	Auth       string
	SignCheck  bool

	// FlattenLayers selects which OCI image bytes are returned by
	// Client.GetPackage:
	//
	//   - false (default, legacy): only the LAST layer of the image is
	//     returned as a gzipped tar. This is what the historical /package
	//     and rpp-get callers expect.
	//   - true: all layers are merged via go-containerregistry's
	//     mutate.Extract and the resulting filesystem is returned as a
	//     single gzipped tar. Required when the file you want may live in
	//     any layer (e.g. icon extraction from arbitrarily-built package
	//     images).
	//
	// Switching this on changes the bytes (and therefore the layer digest)
	// returned for a given manifest digest, so callers that cache by
	// manifest digest must not flip the flag between requests for the same
	// digest.
	FlattenLayers bool
}

// PackagesConfig describes connection parameters resolved for a packages
// repository (e.g. PackageRepository CR). It carries the same registry
// connection fields as ClientConfig but is returned by ClientConfigGetter
// keyed by a packages-repository name rather than by a registry repository.
//
// SignCheck is intentionally omitted: it is a runtime-level concern of the
// proxy, not a property of the upstream registry credentials.
type PackagesConfig struct {
	Repository string
	Scheme     string
	CA         string
	Auth       string
}

// ToClientConfig converts a PackagesConfig into a ClientConfig usable by
// registry.Client methods, copying the requested runtime flags.
func (c *PackagesConfig) ToClientConfig(signCheck, flattenLayers bool) *ClientConfig {
	if c == nil {
		return nil
	}
	return &ClientConfig{
		Repository:    c.Repository,
		Scheme:        c.Scheme,
		CA:            c.CA,
		Auth:          c.Auth,
		SignCheck:     signCheck,
		FlattenLayers: flattenLayers,
	}
}
