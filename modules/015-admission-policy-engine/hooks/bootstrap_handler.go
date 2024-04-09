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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	pattern = "*.yaml"
)

type cTemplate struct {
	Name      string
	Processed bool
	Created   bool
}

// Before creating Gatekepeer's constraints, we have to make sure that all necessary ConstraintTemplates and their CRDs are present in the cluster,
// after that we set the `bootstrapped` flag, which in turn permits creating relevant constraints.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/admission-policy-engine/bootstrap",
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 3 * time.Second,
		ExecutionBurst:       1,
	},
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
	templates := input.Snapshots["gatekeeper_templates"]

	if len(templates) != 0 {
		existingTemplates := make(map[string]cTemplate, len(templates))
		bootstrapped = true

		for _, template := range templates {
			t, ok := template.(cTemplate)
			if !ok {
				return fmt.Errorf("Cannot convert ConstraintTemplate")
			}
			existingTemplates[t.Name] = cTemplate{
				Processed: t.Processed,
				Created:   t.Created,
			}
		}

		requiredTemplates, err := getRequiredTemplates()
		if err != nil {
			return err
		}

		for _, name := range requiredTemplates {
			values, ok := existingTemplates[name]
			if !ok {
				// required template isn't found in the cluster
				input.LogEntry.Warnf("admission-policy-engine isn't bootstrapped yet: missing %s ConstraintTemplate", name)
				bootstrapped = false
				break
			}

			if !values.Processed {
				// status.created field of a constraint template isn't found - highly likely the constraint template wasn't processed for some reasons
				input.LogEntry.Warnf("admission-policy-engine isn't bootstrapped yet: ConstraintTemplate %s not processed", name)
				bootstrapped = false
				break
			}
			if !values.Created {
				// status.created field equals false, there might be some errors in processing there
				input.LogEntry.Warnf("admission-policy-engine isn't bootstrapped yet: CRD for ConstraintTemplate %s not created", name)
				bootstrapped = false
				break
			}
		}
	} else {
		input.LogEntry.Warn("admission-policy-engine isn't bootstrapped yet: no required constraint templates found")
	}

	input.Values.Set("admissionPolicyEngine.internal.bootstrapped", bootstrapped)

	input.MetricsCollector.Expire("d8_admission_policy_engine_not_bootstrapped")
	if !bootstrapped {
		input.MetricsCollector.Set("d8_admission_policy_engine_not_bootstrapped", 1, map[string]string{}, metrics.WithGroup("d8_admission_policy_engine_not_bootstrapped"))
	}

	return nil
}

func filterGatekeeperTemplates(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// check if CRD has been successfully created from the ConstraintTemplate
	created, found, err := unstructured.NestedBool(obj.Object, "status", "created")
	if err != nil {
		return nil, err
	}

	return cTemplate{
		Name:      obj.GetName(),
		Processed: found,
		Created:   created,
	}, nil
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
