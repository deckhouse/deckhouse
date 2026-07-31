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

package machinetemplate

import (
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"sigs.k8s.io/yaml"
)

// nondeterministicFuncs are the sprig functions removed from the sandbox. A machine template must
// be a pure function of its context: node-controller renders it once, at generation creation, and
// the rendered object is then frozen for the life of that generation. A template that reads the
// clock, /dev/urandom, DNS or the process environment would put a value into the cluster that
// nothing can reproduce or explain — and a "did anything change" question would have no answer.
//
// Removal happens at parse time (text/template reports `function "now" not defined`), so a
// template using one of these never reaches a cluster.
var nondeterministicFuncs = []string{
	// clock
	"now", "date", "dateInZone", "date_in_zone", "dateModify", "date_modify",
	"mustDateModify", "htmlDate", "htmlDateInZone", "ago", "toDate", "mustToDate",
	"unixEpoch", "duration", "durationRound", "dateAgo",
	// randomness
	"randAlphaNum", "randAlpha", "randAscii", "randNumeric", "randBytes", "shuffle", "uuidv4",
	// crypto and secrets
	"genPrivateKey", "derivePassword", "buildCustomCert", "genCA", "genCAWithKey",
	"genSelfSignedCert", "genSelfSignedCertWithKey", "genSignedCert", "genSignedCertWithKey",
	"encryptAES", "decryptAES", "bcrypt", "htpasswd",
	// host environment
	"env", "expandenv", "getHostByName",
}

// sandboxFuncMap is stateless, so it is built once.
var sandboxFuncMap = buildSandboxFuncMap()

func buildSandboxFuncMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	for _, name := range nondeterministicFuncs {
		delete(f, name)
	}

	// The YAML/JSON helpers helm adds on top of sprig. They are re-implemented here rather than
	// imported from the legacy machineclass renderer: that package emulates a helm values tree and
	// goes away with the last MCM provider, taking its versions with it. They also differ on
	// purpose — these return an error instead of hiding it in an "Error" key of the result.
	f["toYaml"] = toYAML
	f["fromYaml"] = fromYAML
	f["toJson"] = toJSON
	f["fromJson"] = fromJSON

	// required mirrors helm's: it passes the value through and errors only when it is absent or
	// empty. A template uses it (or sprig's fail) to say what is missing in the provider's own
	// words; node-controller adds the NodeGroup, InstanceClass and zone to the message itself.
	f["required"] = func(message string, value any) (any, error) {
		if value == nil {
			return nil, errors.New(message)
		}
		if s, ok := value.(string); ok && s == "" {
			return nil, errors.New(message)
		}
		return value, nil
	}

	return f
}

// SandboxFuncNames returns the sorted names of every function available to a template. It exists
// for the golden test that pins the sandbox surface: a sprig upgrade that adds a function must be
// reviewed (is it deterministic?) instead of silently widening the contract.
func SandboxFuncNames() []string {
	return slices.Sorted(maps.Keys(sandboxFuncMap))
}

func toYAML(v any) (string, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(data), "\n"), nil
}

func fromYAML(str string) (map[string]any, error) {
	m := map[string]any{}
	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func toJSON(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func fromJSON(str string) (map[string]any, error) {
	m := map[string]any{}
	if err := json.Unmarshal([]byte(str), &m); err != nil {
		return nil, err
	}
	return m, nil
}
