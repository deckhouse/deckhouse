/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
)

const isGlobalRevisionIstiodReadyPath = "istio.internal.isGlobalRevisionIstiodReady"

type istiodPod struct {
	Name     string
	Revision string
	Phase    v1.PodPhase
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
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
			FilterFunc: applyIstiodPodFilter,
		},
	},
}, discoveryIstiodHealthHook)

func applyIstiodPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
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

func discoveryIstiodHealthHook(input *go_hook.HookInput) error {
	var isGlobalRevisionIstiodReady bool
	if !input.Values.Get("istio.internal.globalRevision").Exists() {
		return nil
	}
	globalRevision := input.Values.Get("istio.internal.globalRevision").String()
	for _, podRaw := range input.Snapshots["istiod_pods"] {
		pod := podRaw.(istiodPod)
		if pod.Revision == globalRevision && pod.Phase == v1.PodRunning {
			isGlobalRevisionIstiodReady = true
		}
	}
	input.Values.Set(isGlobalRevisionIstiodReadyPath, isGlobalRevisionIstiodReady)
	if !isGlobalRevisionIstiodReady {
		// There is a problem deleting the webhook configuration from helm. It must be deleted in the first place.
		input.PatchCollector.Delete("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "", "d8-istio-validator-global", object_patch.InForeground())
	}
	return nil
}
