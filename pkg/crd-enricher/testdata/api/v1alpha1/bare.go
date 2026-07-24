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

// Bare exercises examples without any description: they must stay a plain list
// of values, both for scalar and object examples.
//
// +kubebuilder:object:root=true
type Bare struct {
	Spec BareSpec `json:"spec"`
}

type BareSpec struct {
	// +crd-enricher:deckhouse:documentation:examples=stable
	Channel string `json:"channel"`

	Config BareConfig `json:"config"`
}

// BareConfig carries an object example without a description, so it must render
// as a bare object (its own keys) rather than a wrapper.
//
// +crd-enricher:deckhouse:documentation:examples={image: nginx, tag: latest}
type BareConfig struct {
	Image string `json:"image"`
	Tag   string `json:"tag"`
}
