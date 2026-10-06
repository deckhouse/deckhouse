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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// A user project template named after a built-in template of the next release is overwritten by it
// after the update. The alert in monitoring/prometheus-rules/project-templates.yaml says what that
// does and how to keep the template. This hook only exports the metric the alert is built on and
// changes nothing in the cluster.
const (
	projectTemplatesSnapshot = "project_templates"

	// A single group is expired once per run and then fully repopulated, so a template that is
	// deleted or renamed drops out of the alert.
	projectTemplatesMetricGroup = "d8_multitenancy_manager_project_templates"
	// reservedTemplateNameMetric is 1 for a user template whose name is reserved for a built-in one.
	reservedTemplateNameMetric = "d8_multitenancy_manager_reserved_template_name"
)

// reservedTemplateNames are the names of the built-in templates the next release adds. This release
// has no built-in template with these names, so a template with such a name is a user's. The
// controller of the next release keeps its built-in template in sync with its own copy and overwrites
// the spec, labels and annotations of an existing template with that name.
var reservedTemplateNames = []string{"simple"}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       projectTemplatesSnapshot,
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ProjectTemplate",
			FilterFunc: filterProjectTemplateName,
		},
	},
}, alertOnProjectTemplates)

// filterProjectTemplateName keeps only the name of a template, so a change of its spec leaves the
// result unchanged and does not run the hook.
func filterProjectTemplateName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func alertOnProjectTemplates(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(projectTemplatesMetricGroup)

	metricOpts := metrics.WithGroup(projectTemplatesMetricGroup)
	for name, err := range sdkobjectpatch.SnapshotIter[string](input.Snapshots.Get(projectTemplatesSnapshot)) {
		if err != nil {
			return fmt.Errorf("iterate over the %q snapshot: %w", projectTemplatesSnapshot, err)
		}
		if !slices.Contains(reservedTemplateNames, name) {
			continue
		}
		input.MetricsCollector.Set(reservedTemplateNameMetric, 1, map[string]string{"template": name}, metricOpts)
	}

	return nil
}
