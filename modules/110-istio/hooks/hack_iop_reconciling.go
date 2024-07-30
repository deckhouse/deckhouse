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

	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib/crd"
)

const validatingErrorStr = `failed calling webhook`

type IstioOperatorCrdSnapshot struct {
	Revision  string
	NeedPunch bool
}

type IstioOperatorPodSnapshot struct {
	Name              string
	Revision          string
	CreationTimestamp time.Time
	Phase             v1.PodPhase
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: lib.Queue("iop-reconciling"),
	Schedule: []go_hook.ScheduleConfig{
		{Name: "cron", Crontab: "*/5 * * * *"},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "istio_operators",
			ApiVersion:        "install.istio.io/v1alpha1",
			Kind:              "IstioOperator",
			NamespaceSelector: lib.NsSelector(),
			FilterFunc:        applyIopFilter,
		},
		{
			Name:              "istio_operator_pods",
			ApiVersion:        "v1",
			Kind:              "Pod",
			NamespaceSelector: lib.NsSelector(),
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

	result.Name = pod.Name
	result.Revision = pod.Labels["revision"]
	result.CreationTimestamp = pod.CreationTimestamp.Time
	result.Phase = pod.Status.Phase

	return result, nil
}

func hackIopReconcilingHook(input *go_hook.HookInput) error {
	operatorPodMap := make(map[string]string)

	for _, operatorPodRaw := range input.Snapshots["istio_operator_pods"] {
		operatorPod := operatorPodRaw.(IstioOperatorPodSnapshot)
		if time.Now().After(operatorPod.CreationTimestamp.Add(time.Minute*5)) && operatorPod.Phase == v1.PodRunning {
			operatorPodMap[operatorPod.Revision] = operatorPod.Name
		}
	}

	for _, iopRaw := range input.Snapshots["istio_operators"] {
		iop := iopRaw.(IstioOperatorCrdSnapshot)
		if iop.NeedPunch {
			input.LogEntry.Infof("iop with rev %s needs to punch.", iop.Revision)
			if podName, ok := operatorPodMap[iop.Revision]; ok {
				input.LogEntry.Infof("Pod %s is allowed to punch.", podName)
				input.PatchCollector.Delete("v1", "Pod", "d8-istio", podName, object_patch.InBackground())
				input.LogEntry.Infof("Pod %s deleted.", podName)
			}
		}
	}

	return nil
}
