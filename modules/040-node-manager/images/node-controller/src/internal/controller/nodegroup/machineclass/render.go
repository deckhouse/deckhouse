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
	"sort"
	"text/template"
)

func genFuncMap() template.FuncMap {
	f := FuncMap()
	f["include"] = renderInclude
	return f
}

func renderInclude(name string, data interface{}) (string, error) {
	switch name {
	case "helm_lib_module_labels":
		return renderModuleLabels(data)
	default:
		return "", fmt.Errorf("include %q: machine-class renderer only ports helm_lib_module_labels", name)
	}
}

func renderModuleLabels(data interface{}) (string, error) {
	args, ok := data.([]interface{})
	if !ok || (len(args) != 1 && len(args) != 2) {
		return "", fmt.Errorf("helm_lib_module_labels: supports only (list .) and (list . (dict ...)) forms")
	}
	ctx, ok := args[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("helm_lib_module_labels: context is not a map[string]interface{}")
	}
	chart, _ := ctx["Chart"].(map[string]interface{})
	name, _ := chart["Name"].(string)
	out := "labels:\n  heritage: deckhouse\n  module: " + name

	if len(args) == 2 {
		extra, ok := args[1].(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("helm_lib_module_labels: additional labels is not a map[string]interface{}")
		}
		keys := make([]string, 0, len(extra))
		for k := range extra {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out += fmt.Sprintf("\n  %s: %q", k, fmt.Sprintf("%v", extra[k]))
		}
	}
	return out, nil
}

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
