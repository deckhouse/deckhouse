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

package machineclass

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderModuleLabels_ListOneForm(t *testing.T) {
	out, err := renderModuleLabels([]interface{}{
		map[string]interface{}{"Chart": map[string]interface{}{"Name": "node-manager"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "labels:\n  heritage: deckhouse\n  module: node-manager", out)
}

func TestRenderInclude_RejectsUnportedPartial(t *testing.T) {
	_, err := renderInclude("helm_lib_something_else", []interface{}{})
	require.Error(t, err)
}

func TestRenderMachineClass_IncludeNindentByteParity(t *testing.T) {
	tmpl := []byte("metadata:\n" +
		"  name: worker-abcd1234\n" +
		"  namespace: d8-cloud-instance-manager\n" +
		"  {{- include \"helm_lib_module_labels\" (list .) | nindent 2 }}\n" +
		"spec: {}\n")

	out, err := RenderMachineClass(tmpl, map[string]interface{}{
		"Chart": map[string]interface{}{"Name": "node-manager"},
	})
	require.NoError(t, err)

	assert.Equal(t, "metadata:\n"+
		"  name: worker-abcd1234\n"+
		"  namespace: d8-cloud-instance-manager\n"+
		"  labels:\n"+
		"    heritage: deckhouse\n"+
		"    module: node-manager\n"+
		"spec: {}\n", string(out))
}
