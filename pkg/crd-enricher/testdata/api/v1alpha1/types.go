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

// Package v1alpha1 is a minimal API package used only by the enricher tests. It
// declares a single CRD root type so that Run has a Go root to match against the
// CRD YAML fixture; the markers exercise the explicit-vs-generated examples
// distinction.
package v1alpha1

// Foo is the CRD root type backing the test fixture.
//
// +kubebuilder:object:root=true
type Foo struct {
	Spec FooSpec `json:"spec"`
}

// FooSpec carries the spec fields of Foo. The "name" field has no example marker
// (so a generated example is the only way it gets one), while "channel" carries
// an explicit example marker that must survive regardless of the flag.
type FooSpec struct {
	Name string `json:"name"`

	// +crd-enricher:deckhouse:documentation:examples=stable
	Channel string `json:"channel"`

	Registry Registry `json:"registry"`
}

// Registry carries an object example whose keys are authored out of alphabetical
// order ("repo" before "dockerCfg") to exercise order preservation: the example
// must render in the authored order even though the schema properties are
// sorted.
//
// +crd-enricher:deckhouse:documentation:examples={repo: registry.example.io/x, dockerCfg: <credentials>}
type Registry struct {
	Repo      string `json:"repo"`
	DockerCfg string `json:"dockerCfg"`
}
