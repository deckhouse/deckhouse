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
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

const (
	discardMetricsLabelName = "istio.deckhouse.io/discard-metrics"
)

type IstioNamespaceFilterResult struct {
	Name                    string
	DeletionTimestampExists bool
	Revision                string
	DiscardMetrics          bool
}

func applyNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	_, deletionTimestampExists := obj.GetAnnotations()["deletionTimestamp"]

	var namespaceInfo = IstioNamespaceFilterResult{
		Name:                    obj.GetName(),
		DeletionTimestampExists: deletionTimestampExists,
	}

	if discardMetrics, ok := obj.GetLabels()[discardMetricsLabelName]; ok {
		namespaceInfo.DiscardMetrics = discardMetrics == "true"
	}

	return namespaceInfo, nil
}

func applyDiscoveryAppIstioPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var namespaceInfo = IstioNamespaceFilterResult{
		Name: obj.GetNamespace(),
	}
	return namespaceInfo, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("discovery"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "all_namespaces",
			ApiVersion: "v1",
			Kind:       "Namespace",
			FilterFunc: applyNamespaceFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"upmeter"},
					},
				},
			},
		},
		{
			Name:          "namespaces_global_revision",
			ApiVersion:    "v1",
			Kind:          "Namespace",
			FilterFunc:    applyNamespaceFilter,
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"istio-injection": "enabled"}},
		},
		{
			Name:       "namespaces_definite_revision",
			ApiVersion: "v1",
			Kind:       "Namespace",
			FilterFunc: applyNamespaceFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "istio.io/rev",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
		},
		{
			Name:       "istio_pod_global_rev",
			ApiVersion: "v1",
			Kind:       "Pod",
			FilterFunc: applyDiscoveryAppIstioPodFilter,
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "sidecar.istio.io/inject",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"true"},
					},
					{
						Key:      "istio.io/rev",
						Operator: metav1.LabelSelectorOpDoesNotExist,
					},
				},
			},
		},
		{
			Name:       "istio_pod_definite_rev",
			ApiVersion: "v1",
			Kind:       "Pod",
			FilterFunc: applyDiscoveryAppIstioPodFilter,
			NamespaceSelector: &types.NamespaceSelector{
				LabelSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "istio.io/rev",
							Operator: metav1.LabelSelectorOpDoesNotExist,
						},
						{
							Key:      "istio-injection",
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{"enabled"},
						},
					},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "istio.io/rev",
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			},
		},
	},
}, applicationNamespacesDiscovery)

func applicationNamespacesDiscovery(_ context.Context, input *go_hook.HookInput) error {
	var applicationNamespaces = make([]string, 0)
	var applicationNamespacesToMonitor = make([]string, 0)
	var namespacesSnapshots = make([]pkg.Snapshot, 0)
	var namespacesMap = make(map[string]IstioNamespaceFilterResult)

	for nsInfo, err := range sdkobjectpatch.SnapshotIter[IstioNamespaceFilterResult](input.Snapshots.Get("all_namespaces")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'all_namespaces' snapshot: %w", err)
		}

		namespacesMap[nsInfo.Name] = nsInfo
	}

	namespacesSnapshots = append(namespacesSnapshots, input.Snapshots.Get("namespaces_definite_revision")...)
	namespacesSnapshots = append(namespacesSnapshots, input.Snapshots.Get("namespaces_global_revision")...)
	namespacesSnapshots = append(namespacesSnapshots, input.Snapshots.Get("istio_pod_global_rev")...)
	namespacesSnapshots = append(namespacesSnapshots, input.Snapshots.Get("istio_pod_definite_rev")...)
	for nsInfo, err := range sdkobjectpatch.SnapshotIter[IstioNamespaceFilterResult](namespacesSnapshots) {
		if err != nil {
			return fmt.Errorf("failed to iterate over namespace snapshots: %w", err)
		}

		if nsInfo.DeletionTimestampExists {
			continue
		}
		if !lib.Contains(applicationNamespaces, nsInfo.Name) {
			applicationNamespaces = append(applicationNamespaces, nsInfo.Name)
			if !namespacesMap[nsInfo.Name].DiscardMetrics {
				applicationNamespacesToMonitor = append(applicationNamespacesToMonitor, nsInfo.Name)
			}
		}
	}

	sort.Strings(applicationNamespaces)
	sort.Strings(applicationNamespacesToMonitor)

	input.Values.Set("istio.internal.applicationNamespaces", applicationNamespaces)
	input.Values.Set("istio.internal.applicationNamespacesToMonitor", applicationNamespacesToMonitor)

	return nil
}
