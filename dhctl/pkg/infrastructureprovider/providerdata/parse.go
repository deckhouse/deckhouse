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

package providerdata

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	libdhctlyaml "github.com/deckhouse/lib-dhctl/pkg/yaml"
	yamlvalidation "github.com/deckhouse/lib-dhctl/pkg/yaml/validation"
)

const (
	nodeGroupKind     = "NodeGroup"
	nodeGroupAPIGroup = "deckhouse.io"

	instanceClassAPIGroup   = "deckhouse.io"
	instanceClassKindSuffix = "InstanceClass"

	CloudProviderCredentialsSecretType = "cloud-provider.deckhouse.io/credentials"

	cloudProviderModuleNamePrefix = "cloud-provider-"
)

func CloudProviderModuleName(providerName string) string {
	return cloudProviderModuleNamePrefix + strings.ToLower(providerName)
}

func IsCloudPermanentNodeGroup(obj map[string]interface{}) bool {
	nodeType, _, _ := unstructured.NestedString(obj, "spec", "nodeType")
	return nodeType == "CloudPermanent"
}

// ParseResourcesYAML extracts CloudPermanent NodeGroups, instance classes and
// credential Secrets from a multi-document YAML string.
func ParseResourcesYAML(resourcesYAML string) (*CloudProviderVars, error) {
	cv := &CloudProviderVars{}
	if strings.TrimSpace(resourcesYAML) == "" {
		return cv, nil
	}

	docs := libdhctlyaml.SplitYAML(resourcesYAML)

	for i, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		index, err := yamlvalidation.ParseIndex(strings.NewReader(doc))
		if err != nil {
			return nil, fmt.Errorf("parse resources document %d index: %w", i, err)
		}

		var obj map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			return nil, fmt.Errorf("unmarshal resources document %d: %w", i, err)
		}

		switch {
		case index.Kind == nodeGroupKind && index.Group() == nodeGroupAPIGroup:
			if IsCloudPermanentNodeGroup(obj) {
				name, _, _ := unstructured.NestedString(obj, "metadata", "name")
				if name != "" {
					if cv.NodeGroups == nil {
						cv.NodeGroups = make(map[string]map[string]interface{})
					}
					cv.NodeGroups[name] = obj
				}
			}

		case strings.HasSuffix(index.Kind, instanceClassKindSuffix) && index.Group() == instanceClassAPIGroup:
			name, _, _ := unstructured.NestedString(obj, "metadata", "name")
			if name != "" {
				if cv.InstanceClasses == nil {
					cv.InstanceClasses = make(map[string]map[string]interface{})
				}
				cv.InstanceClasses[name] = obj
			}

		case index.Kind == "Secret":
			secretType, _, _ := unstructured.NestedString(obj, "type")
			if secretType == CloudProviderCredentialsSecretType {
				name, _, _ := unstructured.NestedString(obj, "metadata", "name")
				if name != "" {
					if cv.Secrets == nil {
						cv.Secrets = make(map[string]map[string]interface{})
					}
					cv.Secrets[name] = obj
				}
			}
		}
	}

	return cv, nil
}
