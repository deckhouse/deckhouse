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

// These labels are not decorative — the machine templates receive `node-group` through this very
// include, and pruneStaleCAPI/deleteInfraMachineTemplates select on it. The block comes out of the
// YAML marshaller, so keys are canonically ordered and quoting is the marshaller's decision: a
// value that would otherwise parse as a bool or a number is quoted back into a string. helm
// emitted heritage/module first and quoted every extra; the parsed result is the same, and no
// checksum reads the labels.
func TestRenderModuleLabels(t *testing.T) {
	cases := []struct {
		name string
		args []interface{}
		want string
	}{
		{
			name: "list . form",
			args: []interface{}{map[string]interface{}{}},
			want: "labels:\n  heritage: deckhouse\n  module: node-manager",
		},
		{
			name: "additional labels are merged in",
			args: []interface{}{
				map[string]interface{}{},
				map[string]interface{}{"node-group": "worker", "app": "capi-controller-manager"},
			},
			want: "labels:\n  app: capi-controller-manager\n  heritage: deckhouse" +
				"\n  module: node-manager\n  node-group: worker",
		},
		{
			name: "values that are not strings stay strings in YAML",
			args: []interface{}{
				map[string]interface{}{},
				map[string]interface{}{"enabled": true, "count": 3},
			},
			want: "labels:\n  count: \"3\"\n  enabled: \"true\"" +
				"\n  heritage: deckhouse\n  module: node-manager",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := renderModuleLabels(tc.args)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRenderModuleLabels_Rejects(t *testing.T) {
	cases := []struct {
		name string
		args interface{}
	}{
		{name: "not a list", args: map[string]interface{}{}},
		{name: "no arguments", args: []interface{}{}},
		{name: "three arguments", args: []interface{}{map[string]interface{}{}, map[string]interface{}{}, "extra"}},
		{name: "additional labels not a dict", args: []interface{}{map[string]interface{}{}, "worker"}},
		{
			name: "label value is not a scalar",
			args: []interface{}{
				map[string]interface{}{},
				map[string]interface{}{"nested": map[string]interface{}{"a": "b"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := renderModuleLabels(tc.args)
			require.Error(t, err, "a malformed call must fail the render, not emit broken YAML")
		})
	}
}

// Only helm_lib_module_labels is reachable from the four templates node-controller renders
// (machine-class.yaml, machine-template.yaml and the two .checksum files). Anything else must
// stop the render instead of quietly writing a placeholder into a MachineClass field.
func TestUnportedHelmFunctionsFail(t *testing.T) {
	cases := []struct {
		name string
		tmpl string
	}{
		{name: "tpl", tmpl: `{{ tpl "{{ .x }}" . }}`},
		{name: "lookup", tmpl: `{{ lookup "v1" "Secret" "ns" "name" }}`},
		{name: "include of an unported partial", tmpl: `{{ include "helm_lib_something" . }}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RenderMachineClass([]byte(tc.tmpl), map[string]interface{}{})
			require.Error(t, err)
			assert.NotContains(t, string(out), "not implemented")
		})
	}
}

// nindent is what the provider templates pipe the include through, so the byte layout of the
// returned block matters: a stray leading or trailing newline would shift the metadata block.
func TestRenderMachineClass_IncludeNindentByteParity(t *testing.T) {
	cases := []struct {
		name string
		call string
		want string
	}{
		{
			name: "list . form",
			call: `{{- include "helm_lib_module_labels" (list .) | nindent 2 }}`,
			want: "  labels:\n    heritage: deckhouse\n    module: node-manager\n",
		},
		{
			name: "with additional labels",
			call: `{{- include "helm_lib_module_labels" (list . (dict "node-group" .nodeGroup.name)) | nindent 2 }}`,
			want: "  labels:\n    heritage: deckhouse\n    module: node-manager\n    node-group: worker\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpl := "metadata:\n  name: worker-abcd1234\n  namespace: d8-cloud-instance-manager\n  " +
				tc.call + "\nspec: {}\n"

			out, err := RenderMachineClass([]byte(tmpl), map[string]interface{}{
				"nodeGroup": map[string]interface{}{"name": "worker"},
			})
			require.NoError(t, err)

			assert.Equal(t, "metadata:\n  name: worker-abcd1234\n  namespace: d8-cloud-instance-manager\n"+
				tc.want+"spec: {}\n", string(out))
		})
	}
}
