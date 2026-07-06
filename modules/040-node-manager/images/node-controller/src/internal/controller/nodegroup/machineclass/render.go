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
	"bytes"
	"fmt"
	"text/template"
)

// genFuncMap is the FuncMap for rendering the provider machine-class.yaml
// templates (the get_crds node_group_machine_class define). It extends the shared
// checksum FuncMap with a real include for the single partial those templates
// reference — helm_lib_module_labels. It is deliberately kept separate from
// FuncMap so the synced checksum FuncMap (dhctl/helm-mod/bashible) stays
// byte-identical; only the machine-class render path gets a working include.
func genFuncMap() template.FuncMap {
	f := FuncMap()
	f["include"] = renderInclude
	return f
}

// renderInclude implements the Helm include for the only partial every provider
// machine-class.yaml references: helm_lib_module_labels. Any other name errors so
// a template that starts using a not-yet-ported partial fails loudly instead of
// silently diverging the rendered MachineClass.
func renderInclude(name string, data interface{}) (string, error) {
	switch name {
	case "helm_lib_module_labels":
		return renderModuleLabels(data)
	default:
		return "", fmt.Errorf("include %q: machine-class renderer only ports helm_lib_module_labels", name)
	}
}

// renderModuleLabels reproduces deckhouse_lib_helm _module_labels.tpl for the
// (list .) call form — one element, no additional labels — which is the only form
// the provider machine-class.yaml templates use. The result carries no
// surrounding newline; the template's `| nindent 2` supplies indentation. This
// must stay byte-identical to the define or the MachineClass label block diverges.
func renderModuleLabels(data interface{}) (string, error) {
	args, ok := data.([]interface{})
	if !ok || len(args) != 1 {
		return "", fmt.Errorf("helm_lib_module_labels: machine-class renderer supports only the (list .) one-element form")
	}
	ctx, ok := args[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("helm_lib_module_labels: context is not a map[string]interface{}")
	}
	chart, _ := ctx["Chart"].(map[string]interface{})
	name, _ := chart["Name"].(string)
	return "labels:\n  heritage: deckhouse\n  module: " + name, nil
}

// RenderMachineClass renders a provider's machine-class.yaml (node_group_machine_class)
// into the MachineClass manifest bytes. ctx must carry the same shape the Helm tpl
// call builds: Chart{Name}, Values (the full tree, incl. nodeManager.internal and
// global.discovery.clusterUUID), nodeGroup (the blob element) and zoneName.
func RenderMachineClass(templateContent []byte, ctx map[string]interface{}) ([]byte, error) {
	t, err := template.New("machine-class.yaml").Funcs(genFuncMap()).Parse(string(templateContent))
	if err != nil {
		return nil, fmt.Errorf("parse machine-class template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return nil, fmt.Errorf("render machine-class template: %w", err)
	}
	return buf.Bytes(), nil
}
