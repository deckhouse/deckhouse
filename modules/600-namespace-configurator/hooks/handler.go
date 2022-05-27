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
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/namespace-configurator/namespaces_discovery",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "namespaces",
			ApiVersion: "v1",
			Kind:       "Namespace",
			// Ignore upmeter probe fake namespaces, because upmeter deletes them immediately.
			// They do not require any labels.
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values: []string{
							"upmeter",
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

func handleNamespaceConfiguration(input *go_hook.HookInput) error {

	snap := input.Snapshots["namespaces"]
	if len(snap) == 0 {
		input.LogEntry.Debugln("Namespaces not found. Skip")
		return nil
	}

	configurations := input.Values.Get("namespaceConfigurator.configurations").Array()
	var configItem namespaceConfigurationItem
	var err error

	for _, configuration := range configurations {
		err = configItem.Load(configuration)
		if err != nil {
			return err
		}
		err = configItem.Apply(input)
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

func (configItem *namespaceConfigurationItem) Apply(input *go_hook.HookInput) error {
	for _, s := range input.Snapshots["namespaces"] {
		ns := s.(Namespace)
		input.LogEntry.Debugln("Processing namespace:", ns.Name)

		mergePatch := makePatch(input, &ns, configItem)
		if mergePatch != nil {
			input.PatchCollector.MergePatch(mergePatch, "v1", "Namespace", "", ns.Name)
		}
	}
	return nil
}

func makePatch(input *go_hook.HookInput, ns *Namespace, configItem *namespaceConfigurationItem) interface{} {
	var newAnnotations = make(map[string]interface{})
	var newLabels = make(map[string]interface{})
	var mergePatch interface{}
	var matched = false

	input.LogEntry.Debugf("Matching exclude patterns for namespace: %s\n", ns.Name)
	for _, r := range configItem.ExcludePatterns {
		if r.MatchString(ns.Name) {
			input.LogEntry.Debugf("Skip configuring excluded namespace: %s\n", ns.Name)
			return mergePatch
		}
	}

	input.LogEntry.Debugf("Matching include patterns for namespace: %s\n", ns.Name)
	for _, r := range configItem.IncludePatterns {
		if r.MatchString(ns.Name) {
			matched = true
		}
	}
	if !matched {
		input.LogEntry.Debugf("Skip configuring not matched namespace: %s\n", ns.Name)
		return mergePatch
	}

ALOOP:
	for ck, cv := range configItem.Annotations {
		found := false
		for nk, nv := range ns.Annotations {
			if ck == nk {
				found = true
				if cv != nil && cv.(string) == nv {
					input.LogEntry.Debugf("Annotation %s=%s already set for namespace: %s\n", ck, cv, ns.Name)
					continue ALOOP
				}
			}
		}
		if cv == nil && !found {
			input.LogEntry.Debugf("Annotation %s already unset for namespace: %s\n", ck, ns.Name)
			continue ALOOP
		}
		input.LogEntry.Debugf("Setting annotation %s=%s for namespace: %s\n", ck, cv, ns.Name)
		newAnnotations[ck] = cv
	}

LLOOP:
	for ck, cv := range configItem.Labels {
		found := false
		for nk, nv := range ns.Labels {
			if ck == nk {
				found = true
				if cv != nil && cv.(string) == nv {
					input.LogEntry.Debugf("Label %s=%s already set for namespace: %s\n", ck, cv, ns.Name)
					continue LLOOP
				}
			}
		}
		if cv == nil && !found {
			input.LogEntry.Debugf("Label %s already unset for namespace: %s\n", ck, ns.Name)
			continue LLOOP
		}
		input.LogEntry.Debugf("Setting label %s=%s for namespace: %s\n", ck, cv, ns.Name)
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
