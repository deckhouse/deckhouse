/*
Copyright 2022 Flant JSC

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
	"io/fs"
	"os"
	"path/filepath"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

const (
	pattern = "*.yaml"
)

// Before creating Gatekepeer's constraints, we have to have running gatekeeper-controller-manager deployment for handling ConstraintTemplates and all required CRDs (constraint templates) for them
// so, based on ready deployment replicas and constraints templates in place we set the `bootstrapped` flag and create constraints only when true

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "gatekeeper_templates",
			ApiVersion: "templates.gatekeeper.sh/v1",
			Kind:       "ConstraintTemplate",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"heritage": "deckhouse",
					"module":   "admission-policy-engine",
				},
			},
			FilterFunc: filterGatekeeperTemplates,
		},
	},
}, handleGatekeeperBootstrap)

func handleGatekeeperBootstrap(input *go_hook.HookInput) error {
	var bootstrapped bool
	var existingTemplates = set.NewFromSnapshot(input.Snapshots["gatekeeper_templates"])

	if existingTemplates.Size() != 0 {
		bootstrapped = true
		requiredTemplates, err := getRequiredTemplates()
		if err != nil {
			return err
		}

		for _, name := range requiredTemplates {
			if !existingTemplates.Has(name) {
				input.LogEntry.Warnf("admission-policy-engine isn't bootstrapped yet: missing %s constraint template", name)
				bootstrapped = false
				break
			}
		}
	}

	input.Values.Set("admissionPolicyEngine.internal.bootstrapped", bootstrapped)

	return nil
}

func filterGatekeeperTemplates(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func getRequiredTemplates() ([]string, error) {
	type ctemplate struct {
		Kind               string `yaml:"kind"`
		APIVersion         string `yaml:"apiVersion"`
		*metav1.ObjectMeta `yaml:"metadata"`
	}

	root := "/deckhouse/modules/015-admission-policy-engine/charts/constraint-templates/templates/"
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		root = os.Getenv("D8_TEST_CHART_PATH")
	}

	var files []string
	var requiredTemplates = make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if matched, err := filepath.Match(pattern, filepath.Base(path)); err != nil {
			return err
		} else if matched {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		template := &ctemplate{}
		yamlFile, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		err = yaml.Unmarshal(yamlFile, template)
		if err != nil {
			return nil, err
		}
		if template.ObjectMeta != nil && template.Kind == "ConstraintTemplate" && template.APIVersion == "templates.gatekeeper.sh/v1" {
			requiredTemplates = append(requiredTemplates, template.ObjectMeta.Name)
		}
	}

	return requiredTemplates, nil
}
