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
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	istioSystemNs                = "d8-istio"
	istioComponentsLabelSelector = "install.operator.istio.io/owning-resource-namespace=d8-istio"
	istioRootCertConfigMapName   = "istio-ca-root-cert"
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
	istioFederationGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "istiofederations",
	}
	istioMulticlusterGVR = schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1alpha1",
		Resource: "istiomulticlusters",
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

	// Clean up cluster-wide IstioFederation resources
	federations, err := k8sClient.Dynamic().Resource(istioFederationGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		input.Logger.Warn("Failed to list IstioFederation resources", log.Err(err))
	} else {
		for _, fed := range federations.Items {
			_, err = k8sClient.Dynamic().Resource(istioFederationGVR).Patch(context.TODO(), fed.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
			if err != nil {
				input.Logger.Warn("Failed to remove finalizers from IstioFederation", slog.String("finalizer_name", fed.GetName()), log.Err(err))
				continue
			}
			input.Logger.Info("Finalizers from IstioFederation removed", slog.String("finalizer_name", fed.GetName()))

			_, fedDeletionTimestampExists := fed.GetAnnotations()["deletionTimestamp"]
			if !fedDeletionTimestampExists {
				err := k8sClient.Dynamic().Resource(istioFederationGVR).Delete(context.TODO(), fed.GetName(), metav1.DeleteOptions{})
				if err != nil {
					input.Logger.Warn("Failed to delete IstioFederation", slog.String("finalizer_name", fed.GetName()), log.Err(err))
					continue
				}
				input.Logger.Info("IstioFederation deleted", slog.String("finalizer_name", fed.GetName()))
			}
		}
	}

	// Clean up cluster-wide IstioMulticluster resources
	multiclusters, err := k8sClient.Dynamic().Resource(istioMulticlusterGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		input.Logger.Warn("Failed to list IstioMulticluster resources", log.Err(err))
	} else {
		for _, mc := range multiclusters.Items {
			_, err = k8sClient.Dynamic().Resource(istioMulticlusterGVR).Patch(context.TODO(), mc.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
			if err != nil {
				input.Logger.Warn("Failed to remove finalizers from IstioMulticluster", slog.String("finalizer_name", mc.GetName()), log.Err(err))
				continue
			}
			input.Logger.Info("Finalizers from IstioMulticluster removed", slog.String("finalizer_name", mc.GetName()))

			_, mcDeletionTimestampExists := mc.GetAnnotations()["deletionTimestamp"]
			if !mcDeletionTimestampExists {
				err := k8sClient.Dynamic().Resource(istioMulticlusterGVR).Delete(context.TODO(), mc.GetName(), metav1.DeleteOptions{})
				if err != nil {
					input.Logger.Warn("Failed to delete IstioMulticluster", slog.String("finalizer_name", mc.GetName()), log.Err(err))
					continue
				}
				input.Logger.Info("IstioMulticluster deleted", slog.String("finalizer_name", mc.GetName()))
			}
		}
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
			input.Logger.Info("Finalizers from IstioOperator in namespace removed", slog.String("finalizer_name", iop.GetName()), slog.String("namespace", istioSystemNs))
			_, iopDeletionTimestampExists := iop.GetAnnotations()["deletionTimestamp"]
			if !iopDeletionTimestampExists {
				err := k8sClient.Dynamic().Resource(iopGVR).Namespace(istioSystemNs).Delete(context.TODO(), iop.GetName(), metav1.DeleteOptions{})
				if err != nil {
					return err
				}
				input.Logger.Info("IstioOperator deleted from namespace", slog.String("finalizer_name", iop.GetName()), slog.String("namespace", istioSystemNs))
			}
		}

		// Delete the istio-ca-root-cert ConfigMap in namespaces
		namespaces, err := k8sClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, namespace := range namespaces.Items {
			if namespace.Name == istioSystemNs {
				continue
			}

			err := k8sClient.CoreV1().ConfigMaps(namespace.Name).Delete(context.TODO(), istioRootCertConfigMapName, metav1.DeleteOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				input.Logger.Warn("Failed to delete ConfigMap in namespace", slog.String("configmap", istioRootCertConfigMapName), slog.String("namespace", namespace.Name), log.Err(err))
				continue
			}
			input.Logger.Info("ConfigMap deleted from namespace", slog.String("configmap", istioRootCertConfigMapName), slog.String("namespace", namespace.Name))
		}

		// delete NS
		_, nsDeletionTimestampExists := ns.GetAnnotations()["deletionTimestamp"]
		if !nsDeletionTimestampExists {
			err := k8sClient.CoreV1().Namespaces().Delete(context.TODO(), ns.GetName(), metav1.DeleteOptions{})
			if err != nil {
				return err
			}
			input.Logger.Info("Namespace deleted", slog.String("namespace", ns.GetName()))
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
		input.Logger.Info("ClusterRole deleted", slog.String("name", icr.GetName()))
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
		input.Logger.Info("ClusterRoleBinding deleted", slog.String("name", icrb.GetName()))
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
		input.Logger.Info("MutatingWebhookConfiguration deleted", slog.String("name", imwc.GetName()))
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
		input.Logger.Info("ValidatingWebhookConfiguration deleted", slog.String("name", ivwc.GetName()))
	}

	return nil
}
