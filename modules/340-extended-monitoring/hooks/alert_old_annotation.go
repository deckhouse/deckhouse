/*
Copyright 2023 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const extendedMonitoringAnnotationKey = "extended-monitoring.flant.com/enabled"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "namespaces",
			ApiVersion: "v1",
			Kind:       "Namespace",
			FilterFunc: applyNameNamespaceFilter,
		},
		{
			Name:       "deployments",
			ApiVersion: "apps/v1",
			Kind:       "Deployment",
			FilterFunc: applyNameNamespaceFilter,
		},
		{
			Name:       "statefulsets",
			ApiVersion: "apps/v1",
			Kind:       "StatefulSet",
			FilterFunc: applyNameNamespaceFilter,
		},
		{
			Name:       "daemonsets",
			ApiVersion: "apps/v1",
			Kind:       "DaemonSet",
			FilterFunc: applyNameNamespaceFilter,
		},
		{
			Name:       "cronjobs",
			ApiVersion: "batch/v1",
			Kind:       "CronJob",
			FilterFunc: applyNameNamespaceFilter,
		},
		{
			Name:       "ingresses",
			ApiVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
			FilterFunc: applyNameNamespaceFilter,
		},
		{
			Name:       "nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			FilterFunc: applyNameNamespaceFilter,
		},
	},
}, handleLegacyAnnotatedResource)

type ObjectNameNamespaceKind struct {
	Name      string
	Namespace string
	Kind      string
}

func applyNameNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	if _, ok := obj.GetAnnotations()[extendedMonitoringAnnotationKey]; !ok {
		return nil, nil
	}

	return &ObjectNameNamespaceKind{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      obj.GetKind(),
	}, nil
}

func handleLegacyAnnotatedResource(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("d8_deprecated_legacy_annotation")

	if err := iterateOverSnapshotsAndSetMetric(input.MetricsCollector, input.Snapshots.Get("nodes")); err != nil {
		return fmt.Errorf("failed to iterate over 'nodes' snapshots: %w", err)
	}
	if err := iterateOverSnapshotsAndSetMetric(input.MetricsCollector, input.Snapshots.Get("namespaces")); err != nil {
		return fmt.Errorf("failed to iterate over 'namespaces' snapshots: %w", err)
	}
	if err := iterateOverSnapshotsAndSetMetric(input.MetricsCollector, input.Snapshots.Get("deployments")); err != nil {
		return fmt.Errorf("failed to iterate over 'deployments' snapshots: %w", err)
	}
	if err := iterateOverSnapshotsAndSetMetric(input.MetricsCollector, input.Snapshots.Get("statefulsets")); err != nil {
		return fmt.Errorf("failed to iterate over 'statefulsets' snapshots: %w", err)
	}
	if err := iterateOverSnapshotsAndSetMetric(input.MetricsCollector, input.Snapshots.Get("daemonsets")); err != nil {
		return fmt.Errorf("failed to iterate over 'daemonsets' snapshots: %w", err)
	}
	if err := iterateOverSnapshotsAndSetMetric(input.MetricsCollector, input.Snapshots.Get("cronjobs")); err != nil {
		return fmt.Errorf("failed to iterate over 'cronjobs' snapshots: %w", err)
	}
	if err := iterateOverSnapshotsAndSetMetric(input.MetricsCollector, input.Snapshots.Get("ingresses")); err != nil {
		return fmt.Errorf("failed to iterate over 'ingresses' snapshots: %w", err)
	}

	return nil
}

func iterateOverSnapshotsAndSetMetric(collector sdkpkg.MetricsCollector, filterResults []sdkpkg.Snapshot) error {
	for obj, err := range sdkobjectpatch.SnapshotIter[ObjectNameNamespaceKind](filterResults) {
		if err != nil {
			return fmt.Errorf("failed to iterate over snapshots: %w", err)
		}

		collector.Set("d8_deprecated_legacy_annotation", 1, map[string]string{"kind": obj.Kind, "namespace": obj.Namespace, "name": obj.Name}, metrics.WithGroup("d8_deprecated_legacy_annotation"))
	}

	return nil
}
