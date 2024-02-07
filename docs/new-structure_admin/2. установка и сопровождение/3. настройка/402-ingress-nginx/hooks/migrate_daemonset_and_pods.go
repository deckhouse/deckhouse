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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
)

// TODO: Remove this migration hook after release 1.46

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/ingress-nginx/migrate_daemonset",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "ds",
			ApiVersion:                   "apps/v1",
			Kind:                         "DaemonSet",
			WaitForSynchronization:       pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			NamespaceSelector:            internal.NsSelector(),
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"controller", "proxy-failover"},
					},
				},
			},
			FilterFunc: applyDaemonSetNameFilter,
		},
		{
			Name:                         "pods",
			ApiVersion:                   "v1",
			Kind:                         "Pod",
			WaitForSynchronization:       pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			NamespaceSelector:            internal.NsSelector(),
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"controller"},
					},
					{
						Key:      "ingress.deckhouse.io/block-deleting",
						Operator: metav1.LabelSelectorOpDoesNotExist,
					},
					{
						Key:      "lifecycle.apps.kruise.io/state",
						Operator: metav1.LabelSelectorOpDoesNotExist,
					},
				},
			},
			FilterFunc: applyIngressPodFilter,
		},
		{
			Name:                         "controller",
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "IngressNginxController",
			ExecuteHookOnSynchronization: pointer.Bool(false),
			ExecuteHookOnEvents:          pointer.Bool(false),
			FilterFunc:                   inletHostWithFailoverFilter,
		},
	},
}, migrateDaemonSet)

func applyDaemonSetNameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func inletHostWithFailoverFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	name := obj.GetName()
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from ingress controller %s: %v", name, err)
	}
	if !ok {
		return nil, fmt.Errorf("ingress controller %s has no spec field", name)
	}

	inlet, _, err := unstructured.NestedString(spec, "inlet")
	if err != nil {
		return nil, fmt.Errorf("cannot get inlet for ingress controller %s: %v", name, err)
	}

	// we have to add label "ingress.deckhouse.io/block-deleting": "true" only for HostWithFailover pods
	// other inlets will work out of the box
	if inlet != "HostWithFailover" {
		return nil, nil
	}

	return name, nil
}

func migrateDaemonSet(input *go_hook.HookInput) (err error) {
	dss := input.Snapshots["ds"]

	for _, ds := range dss {
		dsName := ds.(string)
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]string{
					"helm.sh/resource-policy": "keep",
				},
			},
		}
		input.PatchCollector.MergePatch(patch, "apps/v1", "DaemonSet", internal.Namespace, dsName, object_patch.IgnoreMissingObject())
		input.PatchCollector.Delete("apps/v1", "DaemonSet", internal.Namespace, dsName, object_patch.NonCascading())
	}

	for _, sn := range input.Snapshots["controller"] {
		if sn == nil {
			continue
		}

		controllerName := sn.(string)

		for _, sn := range input.Snapshots["pods"] {
			pod := sn.(ingressControllerPod)
			if pod.ControllerName != controllerName {
				continue
			}

			input.PatchCollector.MergePatch(blockDeletingLabel, "v1", "Pod", internal.Namespace, pod.Name)
		}
	}

	return nil
}

var (
	blockDeletingLabel = map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]string{
				"ingress.deckhouse.io/block-deleting": "true",
			},
		},
	}
)
