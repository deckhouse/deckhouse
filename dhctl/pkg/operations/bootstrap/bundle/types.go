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

package bundle

type (
	// StopRegistry gracefully shuts down the bundle registry server.
	StopRegistry func()
	// StopTunnel closes the SSH reverse tunnel to the bundle registry.
	StopTunnel func()
)

// RegistryConfigProvider abstracts registry configuration, allowing callers to query
// whether the cluster uses a local (bundle-based) registry.
type RegistryConfigProvider interface {
	IsLocal() (bool, error)
}
