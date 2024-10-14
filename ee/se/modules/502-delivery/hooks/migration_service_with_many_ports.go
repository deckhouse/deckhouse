/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
)

type serviceInfo struct {
	Name      string
	Namespace string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "service_helm_fix",
			ApiVersion: "v1",
			Kind:       "Service",
			FilterFunc: applyServiceFilterHelmFix,
			NameSelector: &types.NameSelector{
				MatchNames: []string{"argocd-repo-server", "argocd-server"},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "migration.deckhouse.io/fix-services-broken-by-helm",
						Operator: v1.LabelSelectorOpNotIn,
						Values:   []string{"done"},
					},
				},
			},
			NamespaceSelector: lib.NsSelector(),
		},
	},
}, patchServiceWithManyPorts)

func applyServiceFilterHelmFix(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return serviceInfo{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}, nil
}

func patchServiceWithManyPorts(input *go_hook.HookInput) error {
	serviceSnapshots := input.Snapshots["service_helm_fix"]
	for _, serviceSnapshot := range serviceSnapshots {
		serviceInfoObj := serviceSnapshot.(serviceInfo)
		input.PatchCollector.Delete(
			"v1",
			"Service",
			serviceInfoObj.Name,
			serviceInfoObj.Namespace,
			object_patch.InForeground(),
		)
	}
	return nil
}
