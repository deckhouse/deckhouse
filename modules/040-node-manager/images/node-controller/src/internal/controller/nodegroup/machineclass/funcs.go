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
	"encoding/json"
	"maps"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"
)

// FuncMap must stay byte-for-byte compatible with the Helm/dhctl engine that
// renders the provider machine-class.checksum templates, otherwise the computed
// checksum diverges from the get_crds machineclass_checksum hooks and triggers a
// mass node rollout.
//
// NOTE: Sync the content of this function among these files!
//                 dhctl/pkg/template/funcs.go
//                 helm-mod/pkg/engine/funcs.go
//                 modules/040-node-manager/images/bashible-apiserver/pkg/template/funcs.go
//  (you are here) modules/040-node-manager/images/node-controller/src/internal/controller/nodegroup/machineclass/funcs.go
//
// include/tpl/required/lookup are placeholders: the checksum templates do not use
// them, but they are kept so the FuncMap matches the shared definition verbatim.
func FuncMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	extra := template.FuncMap{
		"toToml":        toTOML,
		"toYaml":        toYAML,
		"fromYaml":      fromYAML,
		"fromYamlArray": fromYAMLArray,
		"toJson":        toJSON,
		"fromJson":      fromJSON,
		"fromJsonArray": fromJSONArray,

		"include":  func(string, any) string { return "not implemented" },
		"tpl":      func(string, any) any { return "not implemented" },
		"required": func(string, any) (any, error) { return "not implemented", nil },
		"lookup": func(string, string, string, string) (map[string]any, error) {
			return map[string]any{}, nil
		},
	}

	maps.Copy(f, extra)

	return f
}

func toYAML(v any) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

func fromYAML(str string) map[string]any {
	m := map[string]any{}
	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

func fromYAMLArray(str string) []any {
	a := []any{}
	if err := yaml.Unmarshal([]byte(str), &a); err != nil {
		a = []any{err.Error()}
	}
	return a
}

func toTOML(v any) string {
	b := bytes.NewBuffer(nil)
	e := toml.NewEncoder(b)
	err := e.Encode(v)
	if err != nil {
		return err.Error()
	}
	return b.String()
}

func toJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}

func fromJSON(str string) map[string]any {
	m := make(map[string]any)
	if err := json.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

func fromJSONArray(str string) []any {
	a := []any{}
	if err := json.Unmarshal([]byte(str), &a); err != nil {
		a = []any{err.Error()}
	}
	return a
}
