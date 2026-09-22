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

// Quoting exercises the one way a marker value stops being what its author
// wrote: the value is YAML, so prose containing a colon and a space parses as a
// mapping instead of a string.
//
// +kubebuilder:object:root=true
type Quoting struct {
	Spec QuotingSpec `json:"spec"`
}

type QuotingSpec struct {
	// Phase: the override below is written unquoted, so it never reaches the
	// schema as text.
	//
	// +crd-enricher:raw:description=Phase of the resource (`reason: Deleting` while it is torn down).
	Phase string `json:"phase"`
}
