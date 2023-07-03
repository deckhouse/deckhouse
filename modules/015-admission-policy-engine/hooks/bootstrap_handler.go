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
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
			Name:       "gatekeeper_deployment",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-admission-policy-engine"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"gatekeeper-controller-manager"},
			},
			FilterFunc: filterGatekeeperDeployment,
		},
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
	var existingTemplates = make(map[string]struct{})
	snap := input.Snapshots["gatekeeper_deployment"]
	if len(snap) == 0 {
		input.Values.Set("admissionPolicyEngine.internal.bootstrapped", false)
		return nil
	}

	flag, ok := input.Values.GetOk("admissionPolicyEngine.internal.bootstrapped")
	if ok {
		if flag.Bool() {
			// to prevent flapping
			return nil
		}
	}

	// check if deployment is ready
	bootstrapped := snap[0].(bool)

	if bootstrapped {
		requiredTemplates, err := getRequiredTemplates()
		if err != nil {
			return err
		}
		for _, snapshot := range input.Snapshots["gatekeeper_templates"] {
			name := snapshot.(string)
			existingTemplates[name] = struct{}{}
		}

		for _, name := range requiredTemplates {
			if _, exists := existingTemplates[name]; !exists {
				input.LogEntry.Warnf("admission-policy-engine isn't bootstrapped yet: missing %s constraint template", name)
				bootstrapped = false
				break
			}
		}
	}

	input.Values.Set("admissionPolicyEngine.internal.bootstrapped", bootstrapped)

	return nil
}

func filterGatekeeperDeployment(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var dep v1.Deployment

	err := sdk.FromUnstructured(obj, &dep)
	if err != nil {
		return nil, err
	}

	return dep.Status.ReadyReplicas > 0, nil
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
