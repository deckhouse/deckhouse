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

// Described exercises examples-description: when any example carries a
// description the whole list switches to the {x-description, x-example}
// wrapper form.
//
// +kubebuilder:object:root=true
type Described struct {
	Spec DescribedSpec `json:"spec"`
}

type DescribedSpec struct {
	// Settings: every example is described, so each renders as a full wrapper.
	//
	// +crd-enricher:deckhouse:documentation:examples={field: value}
	// +crd-enricher:deckhouse:documentation:examples-description=my super example
	// +crd-enricher:deckhouse:documentation:examples={field: value2}
	// +crd-enricher:deckhouse:documentation:examples-description=my super example two
	Settings map[string]string `json:"settings"`

	// Mixed: only the second example is described, so both switch to the wrapper
	// form and the first one omits x-description.
	//
	// +crd-enricher:deckhouse:documentation:examples=5m
	// +crd-enricher:deckhouse:documentation:examples=1h
	// +crd-enricher:deckhouse:documentation:examples-description=one hour
	Mixed string `json:"mixed"`
}
