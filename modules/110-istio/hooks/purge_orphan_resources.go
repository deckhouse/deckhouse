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

package hooks

import (
	"context"
	"encoding/json"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	istioSystemNs                = "d8-istio"
	istioComponentsLabelSelector = "install.operator.istio.io/owning-resource-namespace=d8-istio"
	iopCrdName                   = "istiooperators.install.istio.io"
)

var (
	deleteFinalizersPatch = map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers": nil,
		},
	}
	iopGVR = schema.GroupVersionResource{
		Group:    "install.istio.io",
		Version:  "v1alpha1",
		Resource: "istiooperators",
	}
	crdGVR = schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterDeleteHelm: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(purgeOrphanResources))

func purgeOrphanResources(input *go_hook.HookInput, dc dependency.Container) error {
	patch, err := json.Marshal(deleteFinalizersPatch)
	if err != nil {
		return err
	}
	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}

	ns, _ := k8sClient.CoreV1().Namespaces().Get(context.TODO(), istioSystemNs, metav1.GetOptions{})
	if ns != nil {
		// iopcrd, _ := k8sClient.Dynamic().Resource(crdGVR).Get(context.TODO(), iopCrdName, metav1.GetOptions{})
		// if iopcrd != nil {
		// remove finalizers and delete iop in ns d8-istio
		iops, err := k8sClient.Dynamic().Resource(iopGVR).Namespace(istioSystemNs).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, iop := range iops.Items {
			_, err = k8sClient.Dynamic().Resource(iopGVR).Namespace(istioSystemNs).Patch(context.TODO(), iop.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
			if err != nil {
				return err
			}
			input.LogEntry.Infof("Remove finalizers from IstioOperator/%s in namespace %s", iop.GetName(), istioSystemNs)
			err := k8sClient.Dynamic().Resource(iopGVR).Namespace(istioSystemNs).Delete(context.TODO(), iop.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			input.LogEntry.Infof("Delete IstioOperator/%s in namespace %s", iop.GetName(), istioSystemNs)
		}
		// }
		// delete NS
		_, nsDeletionTimestampExists := ns.GetAnnotations()["deletionTimestamp"]
		if !nsDeletionTimestampExists {
			err := k8sClient.CoreV1().Namespaces().Delete(context.TODO(), ns.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			input.LogEntry.Infof("Delete namespace %s", ns.GetName())
		}
	}

	// delete ClusterRole
	icrs, err := k8sClient.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		return err
	}
	for _, icr := range icrs.Items {
		err := k8sClient.RbacV1().ClusterRoles().Delete(context.TODO(), icr.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		input.LogEntry.Infof("Delete ClusterRole/%s", icr.GetName())
	}

	// delete ClusterRoleBinding
	icrbs, err := k8sClient.RbacV1().ClusterRoleBindings().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		return err
	}
	for _, icrb := range icrbs.Items {
		err := k8sClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), icrb.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		input.LogEntry.Infof("Delete ClusterRoleBinding/%s", icrb.GetName())
	}

	// delete MutatingWebhookConfiguration
	imwcs, err := k8sClient.AdmissionregistrationV1().MutatingWebhookConfigurations().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		return err
	}
	for _, imwc := range imwcs.Items {
		err := k8sClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.TODO(), imwc.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		input.LogEntry.Infof("Delete MutatingWebhookConfiguration/%s", imwc.GetName())
	}

	// delete ValidatingWebhookConfiguration
	ivwcs, err := k8sClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		return err
	}
	for _, ivwc := range ivwcs.Items {
		err := k8sClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(context.TODO(), ivwc.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		input.LogEntry.Infof("Delete ValidatingWebhookConfiguration/%s", ivwc.GetName())
	}

	return nil
}
