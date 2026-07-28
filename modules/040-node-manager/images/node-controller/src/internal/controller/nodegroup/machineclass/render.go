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
	"strings"
	"text/template"

	"sigs.k8s.io/yaml"
)

// moduleName is what helm_lib_module_labels took from .Chart.Name. node-controller is not helm,
// and none of the templates it renders read .Chart, so the name is stated here once instead of
// being round-tripped through a synthetic chart context.
const moduleName = "node-manager"

const moduleLabelsPartial = "helm_lib_module_labels"

// machineClassFuncMap is stateless, so it is built once instead of on every render.
var machineClassFuncMap = func() template.FuncMap {
	f := FuncMap()
	f["include"] = renderInclude
	return f
}()

func renderInclude(name string, data interface{}) (string, error) {
	if name != moduleLabelsPartial {
		return "", fmt.Errorf("include %q: machine-class renderer only ports %s", name, moduleLabelsPartial)
	}
	return renderModuleLabels(data)
}

// renderModuleLabels ports helm_lib_module_labels: it returns the `labels:` block the provider
// templates pipe through nindent. The labels are load-bearing — the machine templates receive
// `node-group` this way, and pruneStaleCAPI/deleteInfraMachineTemplates select on it.
//
// The block is produced by the YAML marshaller rather than assembled from strings, so quoting
// and escaping cannot be got wrong: keys come out canonically ordered and a value like "true"
// is quoted for us. That differs from helm byte-for-byte (helm emitted heritage/module first and
// quoted every extra), which is harmless: the result is parsed into an object before it is
// applied, and no checksum reads the labels.
func renderModuleLabels(data interface{}) (string, error) {
	labels, err := moduleLabels(data)
	if err != nil {
		return "", fmt.Errorf("%s: %w", moduleLabelsPartial, err)
	}

	out, err := yaml.Marshal(map[string]map[string]string{"labels": labels})
	if err != nil {
		return "", fmt.Errorf("%s: marshal labels: %w", moduleLabelsPartial, err)
	}
	return strings.TrimSuffix(string(out), "\n"), nil
}

// moduleLabels validates the (list .) / (list . (dict ...)) call shapes and flattens them into
// the label set. The template context itself carries nothing we need, so it is only checked for
// shape.
func moduleLabels(data interface{}) (map[string]string, error) {
	args, ok := data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected a list argument, got %T", data)
	}
	if len(args) != 1 && len(args) != 2 {
		return nil, fmt.Errorf("supports only the (list .) and (list . (dict ...)) forms, got %d arguments", len(args))
	}

	labels := map[string]string{
		"heritage": "deckhouse",
		"module":   moduleName,
	}
	if len(args) == 1 {
		return labels, nil
	}

	extra, ok := args[1].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("additional labels must be a dict, got %T", args[1])
	}
	for k, v := range extra {
		value, err := labelValue(v)
		if err != nil {
			return nil, fmt.Errorf("label %q: %w", k, err)
		}
		labels[k] = value
	}
	return labels, nil
}

// labelValue renders a label value the way sprig's quote did — as a string — but refuses
// anything that is not a scalar instead of writing Go's "map[a:b]" rendering into the object.
func labelValue(v interface{}) (string, error) {
	switch value := v.(type) {
	case string:
		return value, nil
	case bool, int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", value), nil
	default:
		return "", fmt.Errorf("expected a scalar, got %T", v)
	}
}

func RenderMachineClass(templateContent []byte, ctx map[string]interface{}) ([]byte, error) {
	t, err := template.New("machine-class.yaml").Funcs(machineClassFuncMap).Parse(string(templateContent))
	if err != nil {
		return nil, fmt.Errorf("parse machine-class template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return nil, fmt.Errorf("render machine-class template: %w", err)
	}
	return buf.Bytes(), nil
}
