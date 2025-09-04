/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/namespace-configurator/namespaces_discovery",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "namespaces",
			ApiVersion: "v1",
			Kind:       "Namespace",
			// Ignore upmeter probe fake namespaces, because upmeter deletes them immediately.
			// Ignore deckhouse and multitenancy-manager namespaces, because they are managed by Deckhouse.
			// They do not require any labels.
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values: []string{
							"upmeter", "deckhouse", "multitenancy-manager",
						},
					},
				},
			},
			FilterFunc: applyNamespaceFilter,
		},
	},
}, handleNamespaceConfiguration)

type Namespace struct {
	Name        string            `json:"name"`
	Annotations map[string]string `json:"annotations"`
	Labels      map[string]string `json:"labels"`
}

func applyNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return Namespace{
		Name:        obj.GetName(),
		Annotations: obj.GetAnnotations(),
		Labels:      obj.GetLabels(),
	}, nil
}

func handleNamespaceConfiguration(ctx context.Context, input *go_hook.HookInput) error {
	namespaces, err := sdkobjectpatch.UnmarshalToStruct[Namespace](input.Snapshots, "namespaces")
	if err != nil {
		return fmt.Errorf("failed to unmarshal namespaces snapshot: %w", err)
	}
	if len(namespaces) == 0 {
		input.Logger.Debug("Namespaces not found. Skip")
		return nil
	}

	configurations := input.Values.Get("namespaceConfigurator.configurations").Array()

	for _, configuration := range configurations {
		var configItem namespaceConfigurationItem

		err = configItem.Load(configuration)
		if err != nil {
			return err
		}
		err = configItem.Apply(ctx, input)
		if err != nil {
			return err
		}
	}

	return nil
}

type namespaceConfigurationItem struct {
	IncludeNames    []string
	ExcludeNames    []string
	Annotations     map[string]interface{}
	Labels          map[string]interface{}
	IncludePatterns []*regexp.Regexp
	ExcludePatterns []*regexp.Regexp
}

func (configItem *namespaceConfigurationItem) Load(result gjson.Result) error {
	configItem.Annotations = make(map[string]interface{})
	for k, v := range result.Get("annotations").Map() {
		if v.Type != gjson.Null {
			configItem.Annotations[k] = v.String()
		} else {
			configItem.Annotations[k] = nil
		}
	}
	configItem.Labels = make(map[string]interface{})
	for k, v := range result.Get("labels").Map() {
		if v.Type != gjson.Null {
			configItem.Labels[k] = v.String()
		} else {
			configItem.Labels[k] = nil
		}
	}
	for _, includeName := range result.Get("includeNames").Array() {
		configItem.IncludeNames = append(configItem.IncludeNames, includeName.String())
	}
	for _, excludeName := range result.Get("excludeNames").Array() {
		configItem.ExcludeNames = append(configItem.ExcludeNames, excludeName.String())
	}
	configItem.IncludePatterns = make([]*regexp.Regexp, len(configItem.IncludeNames))
	for i, s := range configItem.IncludeNames {
		pattern, err := regexp.Compile(s)
		if err != nil {
			return err
		}
		configItem.IncludePatterns[i] = pattern
	}
	configItem.ExcludePatterns = make([]*regexp.Regexp, len(configItem.ExcludeNames))
	for i, s := range configItem.ExcludeNames {
		pattern, err := regexp.Compile(s)
		if err != nil {
			return err
		}
		configItem.ExcludePatterns[i] = pattern
	}
	return nil
}

func (configItem *namespaceConfigurationItem) Apply(_ context.Context, input *go_hook.HookInput) error {
	namespaces, err := sdkobjectpatch.UnmarshalToStruct[Namespace](input.Snapshots, "namespaces")
	if err != nil {
		return fmt.Errorf("failed to unmarshal namespaces snapshot: %w", err)
	}

	for _, ns := range namespaces {
		input.Logger.Debug("Processing namespace:", ns.Name)

		mergePatch := makePatch(input, ns, configItem)
		if mergePatch != nil {
			input.PatchCollector.PatchWithMerge(mergePatch, "v1", "Namespace", "", ns.Name)
		}
	}
	return nil
}

func makePatch(input *go_hook.HookInput, ns Namespace, configItem *namespaceConfigurationItem) interface{} {
	var newAnnotations = make(map[string]interface{})
	var newLabels = make(map[string]interface{})
	var mergePatch interface{}
	var matched = false

	input.Logger.Debug("Matching exclude patterns for namespace", slog.String("namespace", ns.Name))
	for _, r := range configItem.ExcludePatterns {
		if r.MatchString(ns.Name) {
			input.Logger.Debug("Skip configuring excluded namespace", slog.String("namespace", ns.Name))
			return mergePatch
		}
	}

	input.Logger.Debug("Matching include patterns for namespace", slog.String("namespace", ns.Name))
	for _, r := range configItem.IncludePatterns {
		if r.MatchString(ns.Name) {
			matched = true
		}
	}
	if !matched {
		input.Logger.Debug("Skip configuring not matched namespace", slog.String("namespace", ns.Name))
		return mergePatch
	}

ALOOP:
	for ck, cv := range configItem.Annotations {
		found := false
		for nk, nv := range ns.Annotations {
			if ck == nk {
				found = true
				if cv != nil && cv.(string) == nv {
					input.Logger.Debug("Annotation already set for namespace", slog.String("key", ck), slog.Any("value", cv), slog.String("namespace", ns.Name))
					continue ALOOP
				}
			}
		}
		if cv == nil && !found {
			input.Logger.Debug("Annotation already unset for namespace", slog.String("key", ck), slog.String("namespace", ns.Name))
			continue ALOOP
		}
		input.Logger.Debug("Setting annotation for namespace", slog.String("key", ck), slog.Any("value", cv), slog.String("namespace", ns.Name))
		newAnnotations[ck] = cv
	}

LLOOP:
	for ck, cv := range configItem.Labels {
		found := false
		for nk, nv := range ns.Labels {
			if ck == nk {
				found = true
				if cv != nil && cv.(string) == nv {
					input.Logger.Debug("Label already set for namespace", slog.String("key", ck), slog.Any("value", cv), slog.String("namespace", ns.Name))
					continue LLOOP
				}
			}
		}
		if cv == nil && !found {
			input.Logger.Debug("Label already unset for namespace", slog.String("key", ck), slog.String("namespace", ns.Name))
			continue LLOOP
		}
		input.Logger.Debug("Setting label for namespace", slog.String("key", ck), slog.Any("value", cv), slog.String("namespace", ns.Name))
		newLabels[ck] = cv
	}

	if len(newAnnotations) != 0 || len(newLabels) != 0 {
		var newMetadata = make(map[string]interface{})
		if len(newAnnotations) != 0 {
			newMetadata["annotations"] = newAnnotations
		}
		if len(newLabels) != 0 {
			newMetadata["labels"] = newLabels
		}
		mergePatch = map[string]interface{}{
			"metadata": newMetadata,
		}
	}

	return mergePatch
}
