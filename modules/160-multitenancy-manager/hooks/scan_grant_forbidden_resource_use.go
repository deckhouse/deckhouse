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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

// The periodic re-scan lives in its own file on purpose: a Go hook takes its name from the file it
// is registered in (sdk.Registry().Add derives it from the caller frame), and HooksStorage indexes
// hooks by that name, dropping the bindings of the one it replaces. Two RegisterFunc calls in one
// file therefore collapse into a single hook, and only the last registration keeps its bindings.
// Keep one registration per file.

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/160-multitenancy-manager",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "grants",
			Crontab: "*/2 * * * *",
		},
	},
}, dependency.WithExternalDependencies(scanClusterResourceGrantPolicyRulesViolations))

func scanClusterResourceGrantPolicyRulesViolations(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	log := input.Logger
	kube := dc.MustGetK8sClient()

	grantList, err := kube.Dynamic().Resource(schema.GroupVersionResource{
		Group:    "multitenancy.deckhouse.io",
		Version:  "v1alpha1",
		Resource: "clusterresourcegrantpolicies",
	}).List(ctx, v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("fetch grants: %w", err)
	}

	input.MetricsCollector.Expire(grantViolationMetricGroup)

	for _, obj := range grantList.Items {
		g := &grant{}
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, g); err != nil {
			return err
		}
		violations, err := validateGrantNotViolated(ctx, g, kube, log)
		if err != nil {
			return fmt.Errorf("scan grant %s for violations: %w", g.ObjectMeta.Name, err)
		}
		setGrantViolationMetrics(input, g.ObjectMeta.Name, violations)
	}
	return nil
}
