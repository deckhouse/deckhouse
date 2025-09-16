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
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

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

func hackIopReconcilingHook(_ context.Context, input *go_hook.HookInput) error {
	operatorPodMap := make(map[string]string)

	for operatorPod, err := range sdkobjectpatch.SnapshotIter[IstioOperatorPodSnapshot](input.Snapshots.Get("istio_operator_pods")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'istio_operator_pods' snapshot: %w", err)
		}

		if time.Now().After(operatorPod.CreationTimestamp.Add(time.Minute*5)) && operatorPod.Phase == v1.PodRunning {
			operatorPodMap[operatorPod.Revision] = operatorPod.Name
		}
	}

	for iop, err := range sdkobjectpatch.SnapshotIter[IstioOperatorCrdSnapshot](input.Snapshots.Get("istio_operators")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'istio_operators' snapshot: %w", err)
		}

		if iop.NeedPunch {
			input.Logger.Info("iop with rev needs to punch.", slog.String("rev", iop.Revision))
			if podName, ok := operatorPodMap[iop.Revision]; ok {
				input.Logger.Info("Pod is allowed to punch.", slog.String("name", podName))
				input.PatchCollector.DeleteInBackground("v1", "Pod", "d8-istio", podName)
				input.Logger.Info("Pod deleted.", slog.String("name", podName))
			}
		}
	}

	return nil
}
