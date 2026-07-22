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

package v1alpha1

// Ordered exercises key-order preservation of example objects: the authored key
// order must survive even though the schema properties are sorted.
//
// +kubebuilder:object:root=true
type Ordered struct {
	Spec OrderedSpec `json:"spec"`
}

type OrderedSpec struct {
	Registry OrderedRegistry `json:"registry"`

	// Ports: object example with keys authored out of alphabetical order.
	//
	// +crd-enricher:deckhouse:documentation:examples={zebra: 1, apple: 2, mango: 3}
	Ports map[string]int32 `json:"ports"`
}

// OrderedRegistry carries an object example whose keys are authored out of
// alphabetical order ("repo" before "dockerCfg").
//
// +crd-enricher:deckhouse:documentation:examples={repo: registry.example.io/x, dockerCfg: <credentials>}
type OrderedRegistry struct {
	Repo      string `json:"repo"`
	DockerCfg string `json:"dockerCfg"`
}
