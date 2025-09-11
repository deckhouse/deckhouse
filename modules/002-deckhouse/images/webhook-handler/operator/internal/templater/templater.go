// Copyright 2025 Flant JSC
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

package templater

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	deckhouseiov1alpha1 "deckhouse.io/webhook/api/v1alpha1"
	"sigs.k8s.io/yaml"
)

var defaultFuncMap = template.FuncMap{
	"toYaml":   toYAML,
	"indent":   indent,
	"list":     list,
	"split":    strings.Split,
	"join":     strings.Join,
	"getGroup": getGroup,
	// "slice":    slice,
}

func RenderValidationTemplate(tpl string, vh *deckhouseiov1alpha1.ValidationWebhook) (*bytes.Buffer, error) {
	tplt, err := template.New("validation").Funcs(defaultFuncMap).Parse(tpl)
	if err != nil {
		return nil, fmt.Errorf("template parse: %w", err)
	}

	var buf bytes.Buffer

	err = tplt.Execute(&buf, vh)
	if err != nil {
		return nil, fmt.Errorf("template execute: %w", err)
	}

	return &buf, nil
}

func RenderConversionTemplate(tpl string, cwh *deckhouseiov1alpha1.ConversionWebhook) (*bytes.Buffer, error) {
	tplt, err := template.New("conversion").Funcs(defaultFuncMap).Parse(tpl)
	if err != nil {
		return nil, fmt.Errorf("template parse: %w", err)
	}

	var buf bytes.Buffer

	err = tplt.Execute(&buf, cwh)
	if err != nil {
		return nil, fmt.Errorf("template execute: %w", err)
	}

	return &buf, nil
}

// toYAML takes an interface, marshals it to yaml, and returns a string. It will
// always return a string, even on marshal error (empty string).
func toYAML(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}

	data, err = yaml.JSONToYAML(data)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}

	return strings.TrimSuffix(string(data), "\n")
}

func indent(spaces int, s string) string {
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = pad + line
	}
	return strings.Join(lines, "\n")
}

func list(objs ...any) []any {
	return objs
}

// get CRD group from CRD name
func getGroup(name string) string {
	words := strings.Split(name, ".")
	if len(words) >= 1 {
		words = words[1:]
	}
	return strings.Join(words, ".")
}

// func slice(s string, i, j int) string {
// 	return s[i:j]
// }
