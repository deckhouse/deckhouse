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

// Unset exercises unset:<key>: controller-gen writes a description for every
// node it can reach, the ones it renders from a vendored type included, and a
// manifest that is not supposed to carry that text has no way to drop it with
// raw: alone.
//
// +kubebuilder:object:root=true
type Unset struct {
	Spec UnsetSpec `json:"spec"`
}

type UnsetSpec struct {
	// Conditions: the item description comes from the shared condition type and
	// is dropped, while the descriptions this API does want are set as usual, so
	// the two markers are shown working on the same node.
	//
	// +crd-enricher:unset:items.description
	// +crd-enricher:raw:items.properties.type.description=Condition type. `Ready` is `True` once the resource is in place.
	Conditions []Condition `json:"conditions"`

	// Retries keeps its own description: unset addresses one node, not a subtree.
	Retries int `json:"retries"`
}

// Condition stands in for a vendored condition type, whose godoc controller-gen
// renders into items.description.
type Condition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}
