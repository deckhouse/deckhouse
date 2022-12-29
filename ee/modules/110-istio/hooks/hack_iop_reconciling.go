/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

// The operator creates validating webhooks and istio resources, which are validated by this webhook.
// Sometimes it happens that a validating webhook resource is created, but the validating webhook service
// (istiod) itself, for some reason, has not started yet and can't handle requests.
// In this case, the operator cannot deploy the resources because of a timeout from the validating webhook and doesn't retry.
// This hook checks the operator status and if it is error due to a resource validation timeout, it deletes the operator pod.
// After deleting the operator pod it will be recreated and will try to create all the necessary resources again.

package hooks

import (
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal"
	"github.com/deckhouse/deckhouse/ee/modules/110-istio/hooks/internal/crd"
)

const validatingErrorStr = `failed calling webhook "validation.istio.io"`

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
	Queue: internal.Queue("iop-reconciling"),
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "*/5 * * * *"},
	},
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
						Operator: metav1.LabelSelectorOpExists,
					},
					{
						Key:      "app",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"operator"},
					},
				},
			},
			FilterFunc: applyIstioOperatorPodFilter,
		},
	},
}, hackIopReconcilingHook)

func applyIopFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var iop crd.IstioOperator
	var result IstioOperatorCrdSnapshot
	err := sdk.FromUnstructured(obj, &iop)

	if err != nil {
		return nil, err
	}

	result.Revision = iop.Spec.Revision
	if iop.Status.ComponentStatus.Pilot.Status == "ERROR" &&
		strings.Contains(iop.Status.ComponentStatus.Pilot.Error, validatingErrorStr) {
		result.NeedPunch = true
	}
	return result, nil
}

func applyIstioOperatorPodFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod v1.Pod
	var result IstioOperatorPodSnapshot
	err := sdk.FromUnstructured(obj, &pod)
	if err != nil {
		return nil, err
	}

	if pod.CreationTimestamp.After(time.Now().Add(time.Minute*5)) && pod.Status.Phase == v1.PodRunning {
		result.AllowedToPunch = true
	}
	result.Name = pod.Name
	result.Revision = pod.Labels["revision"]
	return result, nil
}

func hackIopReconcilingHook(input *go_hook.HookInput) error {
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
