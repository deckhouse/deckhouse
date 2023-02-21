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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

const extendedMonitoringAnnotationKey = "extended-monitoring.deckhouse.io/enabled"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "namespaces",
			ApiVersion:                   "v1",
			Kind:                         "Namespace",
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   applyNameNamespaceFilter,
		},
		{
			Name:                         "deployments",
			ApiVersion:                   "apps/v1",
			Kind:                         "Deployment",
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   applyNameNamespaceFilter,
		},
		{
			Name:                         "statefulsets",
			ApiVersion:                   "apps/v1",
			Kind:                         "StatefulSet",
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   applyNameNamespaceFilter,
		},
		{
			Name:                         "daemonsets",
			ApiVersion:                   "apps/v1",
			Kind:                         "DaemonSet",
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   applyNameNamespaceFilter,
		},
		{
			Name:                         "cronjobs",
			ApiVersion:                   "batch/v1beta1",
			Kind:                         "CronJob",
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   applyNameNamespaceFilter,
		},
		{
			Name:                         "ingresses",
			ApiVersion:                   "networking.k8s.io/v1",
			Kind:                         "Ingress",
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   applyNameNamespaceFilter,
		},
		{
			Name:                         "nodes",
			ApiVersion:                   "v1",
			Kind:                         "Node",
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   applyNameNamespaceFilter,
		},
	},
}, handleLegacyAnnotatedIngress)

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

func handleLegacyAnnotatedIngress(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("d8_deprecated_legacy_annotation")

	for _, obj := range append(input.Snapshots["namespaces"], input.Snapshots["deployments"], input.Snapshots["statefulsets"],
		input.Snapshots["daemonsets"], input.Snapshots["cronjobs"], input.Snapshots["ingresses"], input.Snapshots["nodes"]) {
		if obj == nil {
			continue
		}

		objMeta := obj.(*ObjectNameNamespaceKind)

		input.MetricsCollector.Set("d8_deprecated_legacy_annotation", 1, map[string]string{"kind": objMeta.Kind, "namespace": objMeta.Namespace, "name": objMeta.Name}, metrics.WithGroup("d8_deprecated_legacy_annotation"))
	}

	return nil
}
