/*
Copyright 2025 Flant JSC

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
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/402-ingress-nginx/hooks/internal"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type IngressControllerWithFinalizer struct {
	Finalizers []string
	Name       string
}

const (
	admissionWebhookName  = "d8-ingress-nginx-admission"
	webhookNamePattern    = "%s.validate.d8-ingress-nginx"
	d8sWebhookNamePattern = "%s.validate.d8-ingress-nginx-deckhouse"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 15},
	Queue:        "/modules/ingress-nginx/handle_finalizers",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "controller",
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "IngressNginxController",
			WaitForSynchronization:       ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			ExecuteHookOnSynchronization: ptr.To(true),
			FilterFunc:                   applyIngressControllerFilter,
		},
		{
			Name:                         "services",
			ApiVersion:                   "v1",
			Kind:                         "Service",
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			WaitForSynchronization:       ptr.To(true),
			NamespaceSelector:            internal.NsSelector(),
			FilterFunc:                   applyServiceFilter,
		},
		{
			Name:                         "daemonsetscruise",
			ApiVersion:                   "apps.kruise.io/v1alpha1",
			Kind:                         "DaemonSet",
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			WaitForSynchronization:       ptr.To(true),
			NamespaceSelector:            internal.NsSelector(),
			FilterFunc:                   applyDaemonSetCruiseFilter,
		},
		{
			Name:                         "valwebhookconfnginx",
			ApiVersion:                   "admissionregistration.k8s.io/v1",
			Kind:                         "validatingwebhookconfigurations",
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			WaitForSynchronization:       ptr.To(true),
			FieldSelector: &types.FieldSelector{
				MatchExpressions: []types.FieldSelectorRequirement{
					{
						Field:    "metadata.name",
						Operator: "Equals",
						Value:    admissionWebhookName,
					},
				},
			},
			FilterFunc: applyIngressControllerWebhookFilter,
		},
	},
}, handleFinalizers)

func applyIngressControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	finalizers := obj.GetFinalizers()
	name := obj.GetName()

	return IngressControllerWithFinalizer{
		Finalizers: finalizers,
		Name:       name,
	}, nil
}

func applyServiceFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var svc corev1.Service
	err := sdk.FromUnstructured(obj, &svc)
	if err != nil {
		return nil, err
	}

	return svc.Name, nil
}

func applyDaemonSetCruiseFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ds appsv1.DaemonSet

	err := sdk.FromUnstructured(obj, &ds)
	if err != nil {
		return nil, err
	}

	return ds.Name, nil
}

func applyIngressControllerWebhookFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var wh admissionregistrationv1.ValidatingWebhookConfiguration
	err := sdk.FromUnstructured(obj, &wh)
	if err != nil {
		return nil, err
	}

	webhooks := make([]string, 0, len(wh.Webhooks))
	for _, wh := range wh.Webhooks {
		webhooks = append(webhooks, wh.Name)
	}

	return webhooks, nil
}

func handleFinalizers(_ context.Context, input *go_hook.HookInput) error {
	const finalizer = "finalizer.ingress-nginx.deckhouse.io"

	serviceNames := set.NewFromSnapshot(input.Snapshots.Get("services"))
	daemonSetNames := set.NewFromSnapshot(input.Snapshots.Get("daemonsetscruise"))
	var webhooks []string
	if len(input.Snapshots.Get("valwebhookconfnginx")) > 0 {
		if err := input.Snapshots.Get("valwebhookconfnginx")[0].UnmarshalTo(&webhooks); err != nil {
			return fmt.Errorf("failed to get validating webhooks from the snapshot: %v", err)
		}
	}

	for controller, err := range sdkobjectpatch.SnapshotIter[IngressControllerWithFinalizer](input.Snapshots.Get("controller")) {
		if err != nil {
			log.Error(fmt.Sprintf("Failed convert shanpshot %s  to IngressControllerWithFinalizer type with error %v", controller.Name, err))
		}

		controllerName := controller.Name

		// Names pattern
		expectedServices := []string{
			controllerName + "-load-balancer",
			controllerName + "-admission",
			fmt.Sprintf("controller-%s-failover", controllerName),
		}
		expectedDaemonSets := []string{
			"controller-" + controllerName,
			fmt.Sprintf("proxy-%s-failover", controllerName),
			fmt.Sprintf("controller-%s-failover", controllerName),
		}

		found := false

		for _, svc := range expectedServices {
			if _, s := serviceNames[svc]; s {
				found = true
				break
			}
		}

		if !found {
			for _, ds := range expectedDaemonSets {
				if _, d := daemonSetNames[ds]; d {
					found = true
					break
				}
			}
		}

		if !found {
		Loop:
			for _, vw := range webhooks {
				switch vw {
				case fmt.Sprintf(webhookNamePattern, controllerName), fmt.Sprintf(d8sWebhookNamePattern, controllerName):
					found = true
					break Loop
				}
			}
		}

		// Set finalizer
		if found {
			finalizers := controller.Finalizers
			if !slices.Contains(finalizers, finalizer) {
				finalizers = append(finalizers, finalizer)
				patch := map[string]interface{}{
					"metadata": map[string]interface{}{
						"finalizers": finalizers,
					},
				}
				input.PatchCollector.PatchWithMerge(patch, "deckhouse.io/v1", "IngressNginxController", "", controllerName)
			}
		} else {
			DeleteFinalizer(input, controllerName, "", "deckhouse.io/v1", "IngressNginxController", finalizer)
		}
	}

	return nil
}

func DeleteFinalizer(input *go_hook.HookInput, crName, crNamespace, crAPIVersion, crKind, finalizerToRemove string) {
	input.PatchCollector.PatchWithMutatingFunc(
		func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
			finalizers := obj.GetFinalizers()
			newFinalizers := make([]string, 0, len(finalizers))
			for _, f := range finalizers {
				if f != finalizerToRemove {
					newFinalizers = append(newFinalizers, f)
				}
			}
			if len(newFinalizers) == len(finalizers) {
				return obj, nil
			}
			objCopy := obj.DeepCopy()
			objCopy.SetFinalizers(newFinalizers)
			return objCopy, nil
		},
		crAPIVersion,
		crKind,
		crNamespace,
		crName,
	)
}
