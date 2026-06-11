// Copyright 2026 Flant JSC
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

package dhctlproviderprotocol

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

// ParseResourcesYAML parses a multi-document YAML string that may contain
// NodeGroups, *InstanceClass objects, and credential Secrets, and returns a
// populated CloudProviderVars. Provider resources arrive pre-parsed in
// input.Vars — this helper is for parsing standalone resource files.
func ParseResourcesYAML(resourcesYAML string) (*CloudProviderVars, error) {
	cv := &CloudProviderVars{}
	if strings.TrimSpace(resourcesYAML) == "" {
		return cv, nil
	}

	for i, doc := range splitYAMLDocs(resourcesYAML) {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var obj map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			return nil, fmt.Errorf("unmarshal resources document %d: %w", i, err)
		}

		kind, _ := obj["kind"].(string)
		apiVersion, _ := obj["apiVersion"].(string)
		group := strings.SplitN(apiVersion, "/", 2)[0]
		name := nestedStringFromObj(obj, "metadata", "name")

		switch {
		case kind == "NodeGroup" && group == "deckhouse.io":
			nodeType := nestedStringFromObj(obj, "spec", "nodeType")
			if nodeType == "CloudPermanent" && name != "" {
				if cv.NodeGroups == nil {
					cv.NodeGroups = make(map[string]map[string]interface{})
				}
				cv.NodeGroups[name] = obj
			}

		case strings.HasSuffix(kind, "InstanceClass") && group == "deckhouse.io":
			if name != "" {
				if cv.InstanceClasses == nil {
					cv.InstanceClasses = make(map[string]map[string]interface{})
				}
				cv.InstanceClasses[name] = obj
			}

		case kind == "Secret":
			secretType, _ := obj["type"].(string)
			if secretType == CredentialsSecretType && name != "" {
				if cv.Secrets == nil {
					cv.Secrets = make(map[string]map[string]interface{})
				}
				cv.Secrets[name] = obj
			}
		}
	}

	return cv, nil
}

func splitYAMLDocs(data string) []string {
	return strings.Split(data, "\n---")
}

func nestedStringFromObj(obj map[string]interface{}, keys ...string) string {
	cur := obj
	for i, k := range keys {
		if i == len(keys)-1 {
			s, _ := cur[k].(string)
			return s
		}
		next, _ := cur[k].(map[string]interface{})
		if next == nil {
			return ""
		}
		cur = next
	}
	return ""
}
