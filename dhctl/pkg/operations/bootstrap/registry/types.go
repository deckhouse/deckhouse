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

type (
	// Stop gracefully shuts down the bundle registry server.
	Stop func()

	// StopTunnel closes the SSH reverse tunnel to the bundle registry.
	StopTunnel func()

	// BundlePathProvider returns the path to the directory with tar or chunk.tar
	// image bundles. Returns an error if the path is not set or invalid.
	BundlePathProvider func() (string, error)
)

// ConfigProvider abstracts registry configuration, allowing callers to query
// whether the cluster uses a local (bundle-based) registry.
type ConfigProvider interface {
	IsLocal() (bool, error)
}
