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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

const (
	discardMetricsLabelName = "istio.deckhouse.io/discard-metrics"
)

type IstioNamespaceFilterResult struct {
	Name                    string
	DeletionTimestampExists bool
	DiscardMetrics          bool
	NamespaceInjection      bool // the namespace labels enable injection for all its pods
	PodInjectionAllowed     bool // the namespace has neither `istio-injection` nor `istio.io/rev` label
}

type IstioPodFilterResult struct {
	Namespace  string
	Injectable bool // false for pods with an empty `istio.io/rev` label, which no webhook matches
}

func applyNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	labels := obj.GetLabels()

	var namespaceInfo = IstioNamespaceFilterResult{
		Name:                    obj.GetName(),
		DeletionTimestampExists: obj.GetDeletionTimestamp() != nil,
	}

	if discardMetrics, ok := labels[discardMetricsLabelName]; ok {
		namespaceInfo.DiscardMetrics = discardMetrics == "true"
	}

	injection, injectionLabelExists := labels["istio-injection"]
	revision, revisionLabelExists := labels["istio.io/rev"]

	switch {
	case injectionLabelExists && revisionLabelExists:
	case injectionLabelExists:
		namespaceInfo.NamespaceInjection = injection == "enabled"
	case revisionLabelExists:
		namespaceInfo.NamespaceInjection = revision != ""
	default:
		namespaceInfo.PodInjectionAllowed = true
	}

	return namespaceInfo, nil
}

func applyDiscoveryAppIstioPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	revision, revisionLabelExists := obj.GetLabels()["istio.io/rev"]

	var podInfo = IstioPodFilterResult{
		Namespace:  obj.GetNamespace(),
		Injectable: !revisionLabelExists || revision != "",
	}
	return podInfo, nil
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
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "istio.io/rev",
						Operator: metav1.LabelSelectorOpExists,
					},
					{
						Key:      "sidecar.istio.io/inject",
						Operator: metav1.LabelSelectorOpNotIn,
						Values:   []string{"false"},
					},
				},
			},
		},
	},
}, applicationNamespacesDiscovery)

func applicationNamespacesDiscovery(_ context.Context, input *go_hook.HookInput) error {
	var applicationNamespaces = make([]string, 0)
	var applicationNamespacesToMonitor = make([]string, 0)
	var namespacesMap = make(map[string]IstioNamespaceFilterResult)
	var applicationNamespacesSet = make(map[string]struct{})

	for nsInfo, err := range sdkobjectpatch.SnapshotIter[IstioNamespaceFilterResult](input.Snapshots.Get("all_namespaces")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'all_namespaces' snapshot: %w", err)
		}

		namespacesMap[nsInfo.Name] = nsInfo
		if nsInfo.NamespaceInjection {
			applicationNamespacesSet[nsInfo.Name] = struct{}{}
		}
	}

	podsSnapshots := append(input.Snapshots.Get("istio_pod_global_rev"), input.Snapshots.Get("istio_pod_definite_rev")...)
	for podInfo, err := range sdkobjectpatch.SnapshotIter[IstioPodFilterResult](podsSnapshots) {
		if err != nil {
			return fmt.Errorf("failed to iterate over pod snapshots: %w", err)
		}

		// pod labels are taken into account only in namespaces without injection labels
		if podInfo.Injectable && namespacesMap[podInfo.Namespace].PodInjectionAllowed {
			applicationNamespacesSet[podInfo.Namespace] = struct{}{}
		}
	}

	for name := range applicationNamespacesSet {
		nsInfo := namespacesMap[name]
		if nsInfo.DeletionTimestampExists {
			continue
		}
		applicationNamespaces = append(applicationNamespaces, name)
		if !nsInfo.DiscardMetrics {
			applicationNamespacesToMonitor = append(applicationNamespacesToMonitor, name)
		}
	}

	sort.Strings(applicationNamespaces)
	sort.Strings(applicationNamespacesToMonitor)

	input.Values.Set("istio.internal.applicationNamespaces", applicationNamespaces)
	input.Values.Set("istio.internal.applicationNamespacesToMonitor", applicationNamespacesToMonitor)

	return nil
}
