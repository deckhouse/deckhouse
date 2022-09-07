/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal/crd"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"time"
)

type IstioOperatorCrdSnapshot struct {
	Revision  string
	NeedPunch bool
}

type IstioOperatorPodSnapshot struct {
	Name           string
	Revision       string
	AllowedToPunch bool
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: internal.Queue("operator-bootstrap"),
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "istio_operators",
			ApiVersion:        "install.istio.io/v1alpha1",
			Kind:              "IstioOperator",
			NamespaceSelector: internal.NsSelector(),
			FilterFunc:        applyIopFilter,
		},
		{
			Name:              "istio_operator_pods",
			ApiVersion:        "v1",
			Kind:              "Pod",
			NamespaceSelector: internal.NsSelector(),
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "revision",
						Operator: "Exists",
					},
					{
						Key:      "app",
						Operator: "In",
						Values:   []string{"operator"},
					},
				},
			},
			FilterFunc: applyPodFilter,
		},
	},
}, operatorBootstrapHook)

func applyIopFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var iop crd.IstioOperator
	var result IstioOperatorCrdSnapshot
	err := sdk.FromUnstructured(obj, &iop)
	if err != nil {
		return nil, err
	}
	result.Revision = iop.Spec.Revision
	if iop.Status.Status == "ERROR" {
		result.NeedPunch = true
	}
	return result, nil
}

func applyPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod v1.Pod
	var result IstioOperatorPodSnapshot
	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, err
	}

	if pod.CreationTimestamp.After(time.Now().Add(time.Minute * 5)) {
		result.AllowedToPunch = true
	}
	result.Name = pod.Name
	result.Revision = pod.Labels["revision"]
	return result, nil
}

func operatorBootstrapHook(input *go_hook.HookInput) error {
	operatorPodMap := make(map[string]string)

	for _, operatorPodRaw := range input.Snapshots["istio_operator_pods"] {
		operatorPod := operatorPodRaw.(IstioOperatorPodSnapshot)
		if operatorPod.AllowedToPunch {
			operatorPodMap[operatorPod.Revision] = operatorPod.Name
		}
	}

	for _, iopRaw := range input.Snapshots["istio_operators"] {
		iop := iopRaw.(IstioOperatorCrdSnapshot)
		if iop.NeedPunch {
			if podName, ok := operatorPodMap[iop.Revision]; ok {
				input.PatchCollector.Delete("v1", "Pod", "d8-istio", podName, object_patch.InBackground())
			}
		}
	}

	return nil
}
