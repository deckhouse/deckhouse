/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
)

type NamespaceInfo struct {
	Name     string
	Revision string // for revisions_monitoring.go
}

func applyNamespaceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var namespaceInfo = NamespaceInfo{
		Name: obj.GetName(),
	}

	if revision, ok := obj.GetLabels()["istio.io/rev"]; ok {
		namespaceInfo.Revision = revision
	} else {
		namespaceInfo.Revision = "global"
	}

	return namespaceInfo, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("discovery"),
	Kubernetes: []go_hook.KubernetesConfig{
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
						Operator: "Exists",
					},
				},
			},
		},
	},
}, applicationNamespacesDiscovery)

func applicationNamespacesDiscovery(input *go_hook.HookInput) error {
	var applicationNamespaces = make([]string, 0)

	for _, ns := range append(input.Snapshots["namespaces_definite_revision"], input.Snapshots["namespaces_global_revision"]...) {
		nsInfo := ns.(NamespaceInfo)
		if !internal.Contains(applicationNamespaces, nsInfo.Name) {
			applicationNamespaces = append(applicationNamespaces, nsInfo.Name)
		}
	}

	sort.Strings(applicationNamespaces)

	input.Values.Set("istio.internal.applicationNamespaces", applicationNamespaces)

	return nil
}
