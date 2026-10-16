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
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	pattern = "*.yaml"

	// webhookConfigurationName is the configuration the module renders once it is bootstrapped.
	webhookConfigurationName = "d8-admission-policy-engine-config"
)

type cTemplate struct {
	Name      string
	Processed bool
	Created   bool
}

// The hook decides what of the module may be rendered:
//   - `bootstrapped` permits the webhook configurations and Ratify. It is set once every required ConstraintTemplate
//     is processed and has its CRD, and it stays set while the webhook configuration exists or the flag is already set,
//     so a template added by a release or a status flap of Gatekeeper never removes the webhooks of a working cluster;
//   - `readyConstraintKinds` lists the kinds whose constraints may be rendered: the template of the module exists and
//     its CRD is established. Constraints of other kinds wait for their CRD.
//
// The kinds do not depend on `status.created` of a template. Gatekeeper derives it from the status objects of its pods,
// which are deleted together with the pods, so the field turns false whenever every Gatekeeper pod is gone, and the
// constraints of a working cluster would be removed. The field also adds no guarantee over the CRD: both appear once
// a single replica has compiled the template.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/admission-policy-engine/bootstrap",
	Settings: &go_hook.HookConfigSettings{
		ExecutionMinInterval: 3 * time.Second,
		ExecutionBurst:       1,
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			// The configuration exists only after the module has been bootstrapped once. While it exists,
			// the flag stays set: a template added by a release or a status flap of Gatekeeper must not
			// remove every webhook and every constraint of the cluster. The configuration also survives
			// a restart of Deckhouse, which resets the previous value of the flag.
			Name:       "webhook_configuration",
			ApiVersion: "admissionregistration.k8s.io/v1",
			Kind:       "ValidatingWebhookConfiguration",
			NameSelector: &types.NameSelector{
				MatchNames: []string{webhookConfigurationName},
			},
			FilterFunc: filterName,
		},
		{
			// Gatekeeper creates a CRD per ConstraintTemplate. A constraint can be applied once its CRD
			// is established; the templates render only the constraints of these kinds.
			Name:       "constraint_crds",
			ApiVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"gatekeeper.sh/constraint": "yes"},
			},
			FilterFunc: filterConstraintKind,
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

func handleGatekeeperBootstrap(_ context.Context, input *go_hook.HookInput) error {
	templates, err := sdkobjectpatch.UnmarshalToStruct[cTemplate](input.Snapshots, "gatekeeper_templates")
	if err != nil {
		return fmt.Errorf("failed to unmarshal gatekeeper_templates: %w", err)
	}

	var bootstrapped bool
	if len(templates) != 0 {
		existingTemplates := make(map[string]cTemplate, len(templates))
		bootstrapped = true

		for _, t := range templates {
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
				input.Logger.Warn("admission-policy-engine isn't bootstrapped yet: missing ConstraintTemplate", slog.String("name", name))
				bootstrapped = false
				break
			}

			if !values.Processed {
				// status.created field of a constraint template isn't found - highly likely the constraint template wasn't processed for some reasons
				input.Logger.Warn("admission-policy-engine isn't bootstrapped yet: ConstraintTemplate not processed", slog.String("name", name))
				bootstrapped = false
				break
			}
			if !values.Created {
				// status.created field equals false, there might be some errors in processing there
				input.Logger.Warn("admission-policy-engine isn't bootstrapped yet: CRD for ConstraintTemplate not created", slog.String("name", name))
				bootstrapped = false
				break
			}
		}
	} else {
		input.Logger.Warn("admission-policy-engine isn't bootstrapped yet: no required constraint templates found")
	}

	crds, err := sdkobjectpatch.UnmarshalToStruct[constraintCRD](input.Snapshots, "constraint_crds")
	if err != nil {
		return fmt.Errorf("failed to unmarshal constraint_crds: %w", err)
	}
	// The template is named after its kind in lower case. A CRD without a template of the module is a leftover.
	moduleTemplates := make(map[string]bool, len(templates))
	for _, t := range templates {
		moduleTemplates[t.Name] = true
	}
	readyKinds := make([]string, 0, len(crds))
	for _, crd := range crds {
		if crd.Kind != "" && crd.Established && moduleTemplates[strings.ToLower(crd.Kind)] {
			readyKinds = append(readyKinds, crd.Kind)
		}
	}
	sort.Strings(readyKinds)
	input.Values.Set("admissionPolicyEngine.internal.readyConstraintKinds", readyKinds)

	// The metric reports every template that is not ready, also when the flag is kept below.
	allTemplatesReady := bootstrapped
	configurations, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "webhook_configuration")
	if err != nil {
		return fmt.Errorf("failed to unmarshal webhook_configuration: %w", err)
	}
	webhooksRendered := false
	for _, name := range configurations {
		if name == webhookConfigurationName {
			webhooksRendered = true
		}
	}
	// The previous value keeps the flag also when the webhook configuration is deleted by hand or by a failed release.
	wasBootstrapped := input.Values.Get("admissionPolicyEngine.internal.bootstrapped").Bool()
	if !bootstrapped && (webhooksRendered || wasBootstrapped) {
		input.Logger.Warn("admission-policy-engine keeps its webhooks: not every ConstraintTemplate is ready yet, constraints of the missing ones wait for their CRDs")
		bootstrapped = true
	}

	input.Values.Set("admissionPolicyEngine.internal.bootstrapped", bootstrapped)
	input.MetricsCollector.Expire("d8_admission_policy_engine_not_bootstrapped")
	if !allTemplatesReady {
		input.MetricsCollector.Set("d8_admission_policy_engine_not_bootstrapped", 1, nil, metrics.WithGroup("d8_admission_policy_engine_not_bootstrapped"))
	}

	return nil
}

// filterName returns the name of an object that is not being deleted: a configuration on its way out
// does not prove the module is bootstrapped.
func filterName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if obj.GetDeletionTimestamp() != nil {
		return "", nil
	}
	return obj.GetName(), nil
}

type constraintCRD struct {
	Kind        string
	Established bool
}

func filterConstraintKind(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	kind, _, err := unstructured.NestedString(obj.Object, "spec", "names", "kind")
	if err != nil {
		return nil, err
	}
	conditions, _, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}
	established := false
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if ok && cond["type"] == "Established" && cond["status"] == "True" {
			established = true
		}
	}
	return constraintCRD{Kind: kind, Established: established}, nil
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
