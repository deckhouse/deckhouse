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
	istioGVR = schema.GroupVersionResource{
		Group:    "sailoperator.io",
		Version:  "v1",
		Resource: "istios",
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

func purgeOrphanResources(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
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
				input.Logger.Warn("Failed to remove finalizers from IstioFederation", slog.String("name", fed.GetName()), log.Err(err))
				continue
			}
			input.Logger.Info("Finalizers from IstioFederation removed", slog.String("name", fed.GetName()))

			_, fedDeletionTimestampExists := fed.GetAnnotations()["deletionTimestamp"]
			if !fedDeletionTimestampExists {
				err := k8sClient.Dynamic().Resource(istioFederationGVR).Delete(context.TODO(), fed.GetName(), metav1.DeleteOptions{})
				if err != nil {
					input.Logger.Warn("Failed to delete IstioFederation", slog.String("name", fed.GetName()), log.Err(err))
					continue
				}
				input.Logger.Info("IstioFederation deleted", slog.String("name", fed.GetName()))
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
				input.Logger.Warn("Failed to remove finalizers from IstioMulticluster", slog.String("name", mc.GetName()), log.Err(err))
				continue
			}
			input.Logger.Info("Finalizers from IstioMulticluster removed", slog.String("name", mc.GetName()))

			_, mcDeletionTimestampExists := mc.GetAnnotations()["deletionTimestamp"]
			if !mcDeletionTimestampExists {
				err := k8sClient.Dynamic().Resource(istioMulticlusterGVR).Delete(context.TODO(), mc.GetName(), metav1.DeleteOptions{})
				if err != nil {
					input.Logger.Warn("Failed to delete IstioMulticluster", slog.String("name", mc.GetName()), log.Err(err))
					continue
				}
				input.Logger.Info("IstioMulticluster deleted", slog.String("name", mc.GetName()))
			}
		}
	}

	ns, _ := k8sClient.CoreV1().Namespaces().Get(context.TODO(), istioSystemNs, metav1.GetOptions{})
	if ns != nil {
		// remove finalizers and delete iop in ns d8-istio
		iops, err := k8sClient.Dynamic().Resource(iopGVR).Namespace(istioSystemNs).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			input.Logger.Warn("Failed to list IstioOperator resources", log.Err(err))
		} else {
			for _, iop := range iops.Items {
				_, err = k8sClient.Dynamic().Resource(iopGVR).Namespace(istioSystemNs).Patch(context.TODO(), iop.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
				if err != nil {
					input.Logger.Warn("Failed to remove finalizers from IstioOperator",
						slog.String("name", iop.GetName()),
						slog.String("namespace", istioSystemNs),
						log.Err(err))
					continue
				}
				input.Logger.Info("Finalizers from IstioOperator removed",
					slog.String("name", iop.GetName()),
					slog.String("namespace", istioSystemNs))

				_, iopDeletionTimestampExists := iop.GetAnnotations()["deletionTimestamp"]
				if !iopDeletionTimestampExists {
					err := k8sClient.Dynamic().Resource(iopGVR).Namespace(istioSystemNs).Delete(context.TODO(), iop.GetName(), metav1.DeleteOptions{})
					if err != nil {
						input.Logger.Warn("Failed to delete IstioOperator",
							slog.String("name", iop.GetName()),
							slog.String("namespace", istioSystemNs),
							log.Err(err))
						continue
					}
					input.Logger.Info("IstioOperator deleted",
						slog.String("name", iop.GetName()),
						slog.String("namespace", istioSystemNs))
				}
			}
		}

		// remove finalizers and delete istios in ns d8-istio
		istios, err := k8sClient.Dynamic().Resource(istioGVR).Namespace(istioSystemNs).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			input.Logger.Warnf("Failed to list Istio resources: %v", err)
		} else {
			for _, istio := range istios.Items {
				_, err = k8sClient.Dynamic().Resource(istioGVR).Namespace(istioSystemNs).Patch(context.TODO(), istio.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
				if err != nil {
					input.Logger.Warnf("Failed to remove finalizers from Istio/%s in namespace %s: %v", istio.GetName(), istioSystemNs, err)
					continue
				}
				input.Logger.Infof("Finalizers from Istio/%s in namespace %s removed", istio.GetName(), istioSystemNs)
				_, istioDeletionTimestampExists := istio.GetAnnotations()["deletionTimestamp"]
				if !istioDeletionTimestampExists {
					err := k8sClient.Dynamic().Resource(istioGVR).Namespace(istioSystemNs).Delete(context.TODO(), istio.GetName(), metav1.DeleteOptions{})
					if err != nil {
						input.Logger.Warnf("Failed to delete Istio/%s from namespace %s: %v", istio.GetName(), istioSystemNs, err)
						continue
					}
					input.Logger.Infof("Istio/%s deleted from namespace %s", istio.GetName(), istioSystemNs)
				}
			}
		}

		// Delete the istio-ca-root-cert ConfigMap in namespaces
		namespaces, err := k8sClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			input.Logger.Warnf("Failed to list namespaces: %v", err)
		} else {
			for _, namespace := range namespaces.Items {
				if namespace.Name == istioSystemNs {
					continue
				}

				err := k8sClient.CoreV1().ConfigMaps(namespace.Name).Delete(context.TODO(), istioRootCertConfigMapName, metav1.DeleteOptions{})
				if err != nil && !k8serrors.IsNotFound(err) {
					input.Logger.Warnf("Failed to delete ConfigMap/%s in namespace %s: %v", istioRootCertConfigMapName, namespace.Name, err)
					continue
				}
				input.Logger.Infof("ConfigMap/%s deleted from namespace %s", istioRootCertConfigMapName, namespace.Name)
			}
		}

		// delete NS
		_, nsDeletionTimestampExists := ns.GetAnnotations()["deletionTimestamp"]
		if !nsDeletionTimestampExists {
			err := k8sClient.CoreV1().Namespaces().Delete(context.TODO(), ns.GetName(), metav1.DeleteOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				input.Logger.Warnf("Failed to delete namespace %s: %v", ns.GetName(), err)
			} else if err == nil {
				input.Logger.Infof("Namespace %s deleted", ns.GetName())
			}
		}
	}

	// delete ClusterRole
	icrs, err := k8sClient.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		input.Logger.Warnf("Failed to list ClusterRoles: %v", err)
	} else {
		for _, icr := range icrs.Items {
			err := k8sClient.RbacV1().ClusterRoles().Delete(context.TODO(), icr.GetName(), metav1.DeleteOptions{})
			if err != nil {
				input.Logger.Warnf("Failed to delete ClusterRole/%s: %v", icr.GetName(), err)
				continue
			}
			input.Logger.Infof("ClusterRole/%s deleted", icr.GetName())
		}
	}

	// delete ClusterRoleBinding
	icrbs, err := k8sClient.RbacV1().ClusterRoleBindings().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		input.Logger.Warnf("Failed to list ClusterRoleBindings: %v", err)
	} else {
		for _, icrb := range icrbs.Items {
			err := k8sClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), icrb.GetName(), metav1.DeleteOptions{})
			if err != nil {
				input.Logger.Warnf("Failed to delete ClusterRoleBinding/%s: %v", icrb.GetName(), err)
				continue
			}
			input.Logger.Infof("ClusterRoleBinding/%s deleted", icrb.GetName())
		}
	}

	// delete MutatingWebhookConfiguration
	imwcs, err := k8sClient.AdmissionregistrationV1().MutatingWebhookConfigurations().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		input.Logger.Warnf("Failed to list MutatingWebhookConfigurations: %v", err)
	} else {
		for _, imwc := range imwcs.Items {
			err := k8sClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.TODO(), imwc.GetName(), metav1.DeleteOptions{})
			if err != nil {
				input.Logger.Warnf("Failed to delete MutatingWebhookConfiguration/%s: %v", imwc.GetName(), err)
				continue
			}
			input.Logger.Infof("MutatingWebhookConfiguration/%s deleted", imwc.GetName())
		}
	}

	// delete ValidatingWebhookConfiguration
	ivwcs, err := k8sClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		input.Logger.Warnf("Failed to list ValidatingWebhookConfigurations: %v", err)
	} else {
		for _, ivwc := range ivwcs.Items {
			err := k8sClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(context.TODO(), ivwc.GetName(), metav1.DeleteOptions{})
			if err != nil {
				input.Logger.Warnf("Failed to delete ValidatingWebhookConfiguration/%s: %v", ivwc.GetName(), err)
				continue
			}
			input.Logger.Infof("ValidatingWebhookConfiguration/%s deleted", ivwc.GetName())
		}
	}

	return nil
}
