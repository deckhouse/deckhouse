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

package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
	sigsyaml "sigs.k8s.io/yaml"
)

// Covers the API type's own marshalling, not the on-disk userdata (built from
// a spec-only type in nodebootstrap/render.go). An unreported status must not
// marshal empty lists: without omitempty they came out as "extensions: null".
func TestNodeConfigStatusMarshalsNothingItDoesNotHave(t *testing.T) {
	config := &NodeConfig{Spec: NodeSpec{NodeName: "master-0"}}

	document, err := sigsyaml.Marshal(config)
	require.NoError(t, err)

	require.NotContains(t, string(document), "extensions: null")
	require.NotContains(t, string(document), "units: null")
}
