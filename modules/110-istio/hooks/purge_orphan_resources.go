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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

const (
	istioSystemNs                = "d8-istio"
	istioComponentsLabelSelector = "install.operator.istio.io/owning-resource-namespace=d8-istio"
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
	istioClusterCRDs = []schema.GroupVersionResource{
		{Group: "networking.istio.io", Version: "v1alpha3", Resource: "envoyfilters"},
		{Group: "networking.istio.io", Version: "v1alpha3", Resource: "gateways"},
		{Group: "security.istio.io", Version: "v1beta1", Resource: "peerauthentications"},
		{Group: "security.istio.io", Version: "v1beta1", Resource: "requestauthentications"},
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
	// Create a rest.Config object
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	// Create the dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	ns, _ := k8sClient.CoreV1().Namespaces().Get(context.TODO(), istioSystemNs, metav1.GetOptions{})
	if ns != nil {
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
			input.Logger.Infof("Finalizers from IstioOperator/%s in namespace %s removed", iop.GetName(), istioSystemNs)
			_, iopDeletionTimestampExists := iop.GetAnnotations()["deletionTimestamp"]
			if !iopDeletionTimestampExists {
				err := k8sClient.Dynamic().Resource(iopGVR).Namespace(istioSystemNs).Delete(context.TODO(), iop.GetName(), metav1.DeleteOptions{})
				if err != nil {
					return err
				}
				input.Logger.Infof("IstioOperator/%s deleted from namespace %s", iop.GetName(), istioSystemNs)
			}
		}
		// delete NS
		_, nsDeletionTimestampExists := ns.GetAnnotations()["deletionTimestamp"]
		if !nsDeletionTimestampExists {
			err := k8sClient.CoreV1().Namespaces().Delete(context.TODO(), ns.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			input.Logger.Infof("Namespace %s deleted", ns.GetName())
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
		input.Logger.Infof("ClusterRole/%s deleted", icr.GetName())
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
		input.Logger.Infof("ClusterRoleBinding/%s deleted", icrb.GetName())
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
		input.Logger.Infof("MutatingWebhookConfiguration/%s deleted", imwc.GetName())
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
		input.Logger.Infof("ValidatingWebhookConfiguration/%s deleted", ivwc.GetName())
	}

	// delete cluster-wide Custom Resources
	for _, icwr := range istioClusterCRDs {
		crList, err := dynamicClient.Resource(icwr).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			input.Logger.Warnf("Failed to list %s: %v", icwr.Resource, err)
			continue
		}
		for _, cr := range crList.Items {
			err := dynamicClient.Resource(icwr).Delete(context.TODO(), cr.GetName(), metav1.DeleteOptions{})
			if err != nil {
				input.Logger.Warnf("Failed to delete %s/%s: %v", icwr.Resource, cr.GetName(), err)
			} else {
				input.Logger.Infof("%s/%s deleted", icwr.Resource, cr.GetName())
			}
		}
	}

	return nil
}
