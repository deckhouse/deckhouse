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

package registry

// The bootstrap layout as the node agent reads it.
//
// A hand-written copy of RegistryNodeSpec rather than the type itself, and for a mechanical reason:
// those types live in go_lib/registry/apis, which needs apimachinery v0.34, while dhctl is pinned to
// client-go v0.33 — importing them builds a dhctl whose own Kubernetes client does not compile
// ("undefined: metav1.InitialEventsListBlueprintAnnotationKey", measured). The same reason the
// `modeManaged` literal beside this file is a literal.
//
// The source of truth is RegistryNodeSpec in
// go_lib/registry/apis/deckhouse.io/v1alpha1/registrynode_types.go. Only what the first master needs
// is here: no additional routes, since those come from custom resources a node in this state cannot
// read anyway, and no storage backend, since there is no store during an installation.
//
// What breaks if the two drift apart is visible rather than silent: the agent rejects a layout it
// cannot parse and says so in its log, and the golden JSON in the test beside this file is what a
// renamed field would have to be changed through.
type bootstrapLayout struct {
	// Cache sends the agent to the storage when true, straight to the upstream when false.
	Cache bool `json:"cache,omitempty"`

	// Backends serve the primary image set, in priority order.
	Backends []layoutBackend `json:"backends,omitempty"`
}

type layoutBackend struct {
	// Name identifies the backend's role: "Upstream" or "Storage".
	Name string `json:"name"`

	layoutEndpoint `json:",inline"`
}

type layoutEndpoint struct {
	// Scheme is "HTTPS" or "HTTP", the spelling the API uses; empty means HTTPS.
	Scheme string `json:"scheme,omitempty"`
	Host   string `json:"host"`
	Path   string `json:"path,omitempty"`

	// CA verifies the endpoint, empty meaning the system trust store.
	CA string `json:"ca,omitempty"`

	Auth *layoutAuth `json:"auth,omitempty"`
}

type layoutAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// backendUpstream is the role name of the primary upstream, as v1alpha1.BackendUpstream spells it.
const backendUpstream = "Upstream"
