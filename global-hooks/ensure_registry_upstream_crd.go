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

package hooks

import "github.com/deckhouse/deckhouse/go_lib/hooks/ensure_crds"

const registryUpstreamCRDPath = "/deckhouse/modules/038-registry/crds/registryupstream.yaml"

// RegistryUpstream is installed whatever the state of the registry module. With the
// module disabled or in Unmanaged mode there is no registry agent on the nodes, and
// nodelet applies these objects on Deckhouse Engine nodes itself — a cluster in that
// state still needs the API to create them. The module ships the same file in its
// crds/, so enabling it installs identical content and the two never compete.
var _ = ensure_crds.RegisterEnsureCRDsHook(registryUpstreamCRDPath)
