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

package derived_status

import (
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// specFrom builds the typed spec from the unstructured shape the API server stores, which is how
// the fixtures are written: a NodeGroup as it exists in a cluster, not as a Go literal. It is the
// same conversion the controllers' typed Get performs, so a fixture that cannot decode is a
// fixture describing a NodeGroup the API server would not have accepted.
func specFrom(raw map[string]interface{}) v1.NodeGroupSpec {
	spec := v1.NodeGroupSpec{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &spec); err != nil {
		panic("fixture spec does not decode into v1.NodeGroupSpec: " + err.Error())
	}
	return spec
}
