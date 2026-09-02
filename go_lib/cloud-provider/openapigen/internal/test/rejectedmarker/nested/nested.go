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

// Package nested holds a deliberately broken marker in a package that the CRD root
// only reaches transitively. It exists to prove the generator refuses to emit a
// schema instead of dropping the constraint. Do not fix the marker below.
package nested

// Spec is referenced from the rejectedmarker root type.
type Spec struct {
	// MinLength on an integer is rejected by controller-tools: without the generator's
	// own check the constraint would silently vanish from the produced schema.
	// +kubebuilder:validation:MinLength=3
	Replicas int `json:"replicas"`
}
