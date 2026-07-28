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

// Named exercises examples-name: a name (like a description) switches the whole
// x-doc-examples list to the wrapper form.
//
// +kubebuilder:object:root=true
type Named struct {
	Spec NamedSpec `json:"spec"`
}

type NamedSpec struct {
	// Full: every example has both a name and a description, so each renders as a
	// {x-doc-name, x-doc-description, x-doc-example} wrapper.
	//
	// +crd-enricher:deckhouse:documentation:examples={field: a}
	// +crd-enricher:deckhouse:documentation:examples-name=First
	// +crd-enricher:deckhouse:documentation:examples-description=the first example
	// +crd-enricher:deckhouse:documentation:examples={field: b}
	// +crd-enricher:deckhouse:documentation:examples-name=Second
	// +crd-enricher:deckhouse:documentation:examples-description=the second example
	Full map[string]string `json:"full"`

	// NameOnly: only names, no descriptions, so each renders as
	// {x-doc-name, x-doc-example} — the x-doc-description key is omitted.
	//
	// +crd-enricher:deckhouse:documentation:examples=1h
	// +crd-enricher:deckhouse:documentation:examples-name=one hour
	// +crd-enricher:deckhouse:documentation:examples=1d
	// +crd-enricher:deckhouse:documentation:examples-name=one day
	NameOnly string `json:"nameOnly"`

	// Trigger: only the first example has a name; the second has nothing. A
	// single name still switches the whole list to the wrapper form, so the
	// second renders as {x-doc-example} only.
	//
	// +crd-enricher:deckhouse:documentation:examples=x
	// +crd-enricher:deckhouse:documentation:examples-name=the x
	// +crd-enricher:deckhouse:documentation:examples=y
	Trigger string `json:"trigger"`
}
