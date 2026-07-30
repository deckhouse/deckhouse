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
	"errors"
	"fmt"
	"maps"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"
)

func FuncMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	// Non-deterministic functions must not be reachable from provider templates: the
	// checksum names the immutable MachineClass/MachineTemplate, so a template that
	// renders differently on every pass would silently roll every node in the group
	// each resync. Deleting them makes such a template fail at Parse instead.
	for _, name := range []string{
		"now", "ago", // clock
		"randAlphaNum", "randAlpha", "randNumeric", "randAscii", "randInt", "randBytes", "shuffle", // math/rand
		"uuidv4", "genPrivateKey", "genCA", "genCAWithKey", "genSelfSignedCert", "genSelfSignedCertWithKey",
		"genSignedCert", "genSignedCertWithKey", "bcrypt", "htpasswd", // crypto/rand
		"getHostByName", // network
	} {
		delete(f, name)
	}

	extra := template.FuncMap{
		"toToml":        toTOML,
		"toYaml":        toYAML,
		"fromYaml":      fromYAML,
		"fromYamlArray": fromYAMLArray,
		"toJson":        toJSON,
		"fromJson":      fromJSON,
		"fromJsonArray": fromJSONArray,

		// Not ported. A template reaching one of these would otherwise get the placeholder
		// string rendered into a MachineClass field and applied to the cluster; failing the
		// render is the only safe answer. RenderMachineClass replaces "include" with the one
		// partial it does port (helm_lib_module_labels).
		"include": func(name string, _ any) (string, error) {
			return "", fmt.Errorf("include %q is not available in the machine-class renderer", name)
		},
		"tpl": func(string, any) (any, error) {
			return nil, errors.New("tpl is not available in the machine-class renderer")
		},
		// required mirrors helm: passes the value through, erroring only when
		// absent. Templates pipe cloudProvider values through it then read fields.
		"required": func(warn string, val any) (any, error) {
			if val == nil {
				return val, errors.New(warn)
			}
			if s, ok := val.(string); ok && s == "" {
				return val, errors.New(warn)
			}
			return val, nil
		},
		"lookup": func(string, string, string, string) (map[string]any, error) {
			return nil, errors.New("lookup is not available in the machine-class renderer")
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
