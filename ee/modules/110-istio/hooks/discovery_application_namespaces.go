/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
)

type IstioNamespaceFilterResult struct {
	Name                    string
	DeletionTimestampExists bool
	RevisionRaw             string // for dataplane_metadata_exporter.go
	Revision                string
	AutoUpgradeLabelExists  bool // for dataplane_metadata_exporter.go
	DiscardMetrics          bool
}

func applyNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	_, deletionTimestampExists := obj.GetAnnotations()["deletionTimestamp"]

	var namespaceInfo = IstioNamespaceFilterResult{
		Name:                    obj.GetName(),
		DeletionTimestampExists: deletionTimestampExists,
	}

	if revision, ok := obj.GetLabels()[autoUpgradeLabelName]; ok {
		namespaceInfo.AutoUpgradeLabelExists = revision == "true"
	}

	if revision, ok := obj.GetLabels()["istio.io/rev"]; ok {
		namespaceInfo.RevisionRaw = revision
	} else {
		namespaceInfo.RevisionRaw = "global"
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
	Queue: internal.Queue("discovery"),
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

func applicationNamespacesDiscovery(input *go_hook.HookInput) error {
	var applicationNamespaces = make([]string, 0)
	var applicationNamespacesToMonitor = make([]string, 0)
	var namespacesSnapshots = make([]go_hook.FilterResult, 0)
	var namespacesMap = make(map[string]IstioNamespaceFilterResult)

	for _, ns := range input.Snapshots["all_namespaces"] {
		nsInfo := ns.(IstioNamespaceFilterResult)
		namespacesMap[nsInfo.Name] = nsInfo
	}

	namespacesSnapshots = append(namespacesSnapshots, input.Snapshots["namespaces_definite_revision"]...)
	namespacesSnapshots = append(namespacesSnapshots, input.Snapshots["namespaces_global_revision"]...)
	namespacesSnapshots = append(namespacesSnapshots, input.Snapshots["istio_pod_global_rev"]...)
	namespacesSnapshots = append(namespacesSnapshots, input.Snapshots["istio_pod_definite_rev"]...)
	for _, ns := range namespacesSnapshots {
		nsInfo := ns.(IstioNamespaceFilterResult)
		if nsInfo.DeletionTimestampExists {
			continue
		}
		if !internal.Contains(applicationNamespaces, nsInfo.Name) {
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
