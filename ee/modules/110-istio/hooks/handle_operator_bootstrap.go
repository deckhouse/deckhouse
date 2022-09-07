/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const globalRevisionIstiodIsReadyPath = "istio.internal.globalRevisionIstiodIsReady"

type istiodPod struct {
	Name     string
	Revision string
	Phase    v1.PodPhase
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("operator-bootstrap"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "istiod_pods",
			ApiVersion:        "v1",
			Kind:              "Pod",
			NamespaceSelector: internal.NsSelector(),
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "istio.io/rev",
						Operator: "Exists",
					},
					{
						Key:      "app",
						Operator: "In",
						Values:   []string{"istiod"},
					},
				},
			},
			FilterFunc: applyPodFilter,
		},
	},
}, operatorBootstrapHook)

func applyPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod v1.Pod
	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, err
	}
	return istiodPod{
		Name:     pod.Name,
		Revision: pod.Labels["istio.io/rev"],
		Phase:    pod.Status.Phase,
	}, nil
}

func operatorBootstrapHook(input *go_hook.HookInput) error {
	var istiodGlobalRevisionIsReady bool
	if !input.Values.Get("istio.internal.globalRevision").Exists() {
		return nil
	}
	globalRevision := input.Values.Get("istio.internal.globalRevision").String()
	for _, podRaw := range input.Snapshots["istiod_pods"] {
		pod := podRaw.(istiodPod)
		if pod.Revision == globalRevision && pod.Phase == v1.PodRunning {
			istiodGlobalRevisionIsReady = true
		}
	}
	input.Values.Set(globalRevisionIstiodIsReadyPath, istiodGlobalRevisionIsReady)
	return nil
}
