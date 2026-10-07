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

package hooks

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// A project template whose resourcesTemplate holds a Helm text keeps rendering its projects from it,
// but the field is deprecated and goes away in a future release. The alert in
// monitoring/prometheus-rules/project-templates.yaml says what to move where. This hook only exports
// the series the alert is built on and changes nothing in the cluster.
//
// The series are per template, not per project: a template can have thousands of projects, and a
// template without projects counts too, since a new project on it renders from the text.
const (
	helmProjectTemplatesSnapshot = "helm_project_templates"

	// A single group is expired once per run and then fully repopulated, so a template that is
	// deleted or moves to structured fields drops out of the alert.
	helmProjectTemplatesMetricGroup = "d8_multitenancy_manager_helm_project_templates"
	// helmProjectTemplateMetric is 1 for a user template whose resourcesTemplate is not blank.
	helmProjectTemplateMetric = "d8_multitenancy_manager_helm_project_template"

	// virtualTemplateName is the template of the virtual projects deckhouse and default: the
	// controller routes such a project to its virtual handler and never renders a template of that
	// name.
	virtualTemplateName = "virtual"
)

// builtinTemplateNames are the templates the controller writes on every start from its own files
// (images/multitenancy-manager/src/templates), overwriting their spec, labels and annotations. The
// built-ins of the previous release are Helm texts until the new controller rewrites them. That
// release shipped no simple, so a template of that name is a user's until the controller overwrites
// it with the built-in. Either way the template is Deckhouse's once the controller has started.
var builtinTemplateNames = []string{"default", "secure", "secure-with-dedicated-nodes", "simple"}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       helmProjectTemplatesSnapshot,
			ApiVersion: "deckhouse.io/v1alpha2",
			Kind:       "ProjectTemplate",
			FilterFunc: filterHelmProjectTemplate,
		},
	},
}, alertOnHelmProjectTemplates)

// filterHelmProjectTemplate keeps the name of a user template that renders from a Helm text and an
// empty string for any other template. The text itself is not kept, so an edit of it leaves the
// result unchanged and does not run the hook.
//
// A template with the heritage: deckhouse label is Deckhouse's as well, whatever its name: a built-in
// template of an earlier release keeps that label, and the deckhouse module does not let users set it.
func filterHelmProjectTemplate(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	isBuiltin := slices.Contains(builtinTemplateNames, obj.GetName())
	hasDeckhouseHeritage := obj.GetLabels()["heritage"] == "deckhouse"
	isVirtual := obj.GetName() == virtualTemplateName
	if isBuiltin || hasDeckhouseHeritage || isVirtual {
		return "", nil
	}

	resourcesTemplate, _, err := unstructured.NestedString(obj.Object, "spec", "resourcesTemplate")
	if err != nil {
		return nil, fmt.Errorf("read spec.resourcesTemplate of the %q project template: %w", obj.GetName(), err)
	}
	// A blank text counts as none, as in the controller: the template renders from its structured fields.
	if strings.TrimSpace(resourcesTemplate) == "" {
		return "", nil
	}

	return obj.GetName(), nil
}

func alertOnHelmProjectTemplates(_ context.Context, input *go_hook.HookInput) error {
	// The expire goes first: addon-operator applies the operations of a group in order, so an expire
	// after the sets would drop every series.
	input.MetricsCollector.Expire(helmProjectTemplatesMetricGroup)

	for name, err := range sdkobjectpatch.SnapshotIter[string](input.Snapshots.Get(helmProjectTemplatesSnapshot)) {
		if err != nil {
			return fmt.Errorf("iterate over the %q snapshot: %w", helmProjectTemplatesSnapshot, err)
		}
		if name == "" {
			continue
		}
		input.MetricsCollector.Set(helmProjectTemplateMetric, 1, map[string]string{"template": name},
			metrics.WithGroup(helmProjectTemplatesMetricGroup))
	}

	return nil
}
