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

package attach

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

// metadataFieldsToDrop are the metadata fields the cluster sets itself: Commander keeps the
// resource in its resource group and the agent applies it again, so they would only make the
// applied object differ from the desired one.
var metadataFieldsToDrop = []string{
	"creationTimestamp",
	"deletionGracePeriodSeconds",
	"deletionTimestamp",
	"finalizers",
	"generateName",
	"generation",
	"managedFields",
	"ownerReferences",
	"resourceVersion",
	"selfLink",
	"uid",
}

// annotationPrefixesToDrop are the annotations written by the tools that applied the object.
var annotationPrefixesToDrop = []string{
	"kubectl.kubernetes.io/",
	"meta.helm.sh/",
}

// providerResourcesYAML renders the provider configuration of a cluster in the new format as a
// multi-document YAML: the provider ModuleConfig, the credentials Secrets, the instance classes and
// the CloudPermanent NodeGroups, in this order, sorted by name inside a kind. A cluster in the new
// format has no provider configuration document (which a legacy scan returns), so scan returns this
// set instead: Commander puts it into a resource group of the attached cluster.
// It returns an empty string for a cluster that has no such set: a static one, one with a legacy
// provider configuration, or one whose provider variables were not read.
func providerResourcesYAML(metaConfig *config.MetaConfig) (string, error) {
	if metaConfig.ClusterType != config.CloudClusterType || metaConfig.HasLegacyProviderConfig() {
		return "", nil
	}

	vars := metaConfig.CloudProviderVars
	if vars == nil {
		return "", nil
	}

	var objects []map[string]any
	if len(vars.Settings) > 0 {
		objects = append(objects, vars.Settings)
	}
	objects = append(objects, objectsByName(vars.Secrets)...)
	objects = append(objects, objectsByName(vars.InstanceClasses)...)
	objects = append(objects, objectsByName(vars.NodeGroups)...)

	documents := make([]string, 0, len(objects))
	for _, object := range objects {
		cleaned, err := cleanResource(object)
		if err != nil {
			return "", err
		}

		document, err := yaml.Marshal(cleaned)
		if err != nil {
			return "", fmt.Errorf("encode provider resource: %w", err)
		}
		documents = append(documents, string(document))
	}

	return strings.Join(documents, "---\n"), nil
}

func objectsByName(objects map[string]map[string]any) []map[string]any {
	names := slices.Sorted(maps.Keys(objects))

	result := make([]map[string]any, 0, len(names))
	for _, name := range names {
		result = append(result, objects[name])
	}

	return result
}

// cleanResource returns a copy of the object without the status and the metadata the cluster sets.
func cleanResource(object map[string]any) (map[string]any, error) {
	data, err := json.Marshal(object)
	if err != nil {
		return nil, fmt.Errorf("copy provider resource: %w", err)
	}

	var cleaned map[string]any
	if err := json.Unmarshal(data, &cleaned); err != nil {
		return nil, fmt.Errorf("copy provider resource: %w", err)
	}

	delete(cleaned, "status")

	metadata, ok := cleaned["metadata"].(map[string]any)
	if !ok {
		return cleaned, nil
	}

	for _, field := range metadataFieldsToDrop {
		delete(metadata, field)
	}

	if annotations, ok := metadata["annotations"].(map[string]any); ok {
		for key := range annotations {
			if slices.ContainsFunc(annotationPrefixesToDrop, func(prefix string) bool { return strings.HasPrefix(key, prefix) }) {
				delete(annotations, key)
			}
		}
		if len(annotations) == 0 {
			delete(metadata, "annotations")
		}
	}

	return cleaned, nil
}
