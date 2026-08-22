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

// LegacyRoot exercises the pre-kubebuilder root marker. controller-gen renders a
// CRD for a type that only declares deepcopy-gen should give it a
// DeepCopyObject method, so a package written that way -- and several of the
// storage modules are -- gets a manifest without ever spelling
// kubebuilder:object:root. Every crd-enricher marker on such a type used to be
// dropped, because the type never entered the root set the enricher walks.
//
// +k8s:deepcopy-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +crd-enricher:crd:preserveUnknownFields=false
type LegacyRoot struct {
	Spec LegacyRootSpec `json:"spec"`
}

type LegacyRootSpec struct {
	// Name carries a field marker as well, so the fixture shows both halves
	// reaching the schema: the CRD-level setting above and this one.
	//
	// +crd-enricher:deckhouse:documentation:examples=first
	Name string `json:"name"`
}
