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
	"regexp"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/modules/110-istio/hooks/lib/istio_versions"
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
	istioRevisionGVR = schema.GroupVersionResource{
		Group:    "sailoperator.io",
		Version:  "v1",
		Resource: "istiorevisions",
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

func isOperatorSupportedRevision(versionMap istio_versions.IstioVersionsMap, revision string) bool {
	if revision == "" {
		return true
	}
	version := versionMap.GetVersionByRevision(revision)
	if version == "" {
		return true
	}
	return versionMap.DoesVersionSupportOperator(version)
}

func isOperatorResourceByName(versionMap istio_versions.IstioVersionsMap, name string) bool {
	revision := regexp.MustCompile(`v\d+x\d+`).FindString(name)
	return isOperatorSupportedRevision(versionMap, revision)
}

func purgeOrphanResources(_ context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	patch, err := json.Marshal(deleteFinalizersPatch)
	if err != nil {
		return err
	}
	k8sClient, err := dc.GetK8sClient()
	if err != nil {
		return err
	}
	versionMap := istio_versions.VersionMapJSONToVersionMap(input.Values.Get("istio.internal.versionMap").String())

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

			if fed.GetDeletionTimestamp() == nil {
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

			if mc.GetDeletionTimestamp() == nil {
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
				revision, _, _ := unstructured.NestedString(iop.Object, "spec", "revision")
				if !isOperatorSupportedRevision(versionMap, revision) {
					continue
				}
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

				if iop.GetDeletionTimestamp() == nil {
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

		// Delete the istio-ca-root-cert ConfigMap in namespaces
		namespaces, err := k8sClient.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			input.Logger.Warn("Failed to list namespaces", log.Err(err))
		} else {
			for _, namespace := range namespaces.Items {
				if namespace.Name == istioSystemNs {
					continue
				}

				err := k8sClient.CoreV1().ConfigMaps(namespace.Name).Delete(context.TODO(), istioRootCertConfigMapName, metav1.DeleteOptions{})
				if err != nil && !k8serrors.IsNotFound(err) {
					input.Logger.Warn("Failed to delete ConfigMap",
						slog.String("name", istioRootCertConfigMapName),
						slog.String("namespace", namespace.Name),
						log.Err(err))
					continue
				}
				input.Logger.Info("ConfigMap deleted",
					slog.String("name", istioRootCertConfigMapName),
					slog.String("namespace", namespace.Name))
			}
		}

		// delete NS
		if ns.GetDeletionTimestamp() == nil {
			err := k8sClient.CoreV1().Namespaces().Delete(context.TODO(), ns.GetName(), metav1.DeleteOptions{})
			if err != nil && !k8serrors.IsNotFound(err) {
				input.Logger.Warn("Failed to delete namespace",
					slog.String("name", ns.GetName()),
					log.Err(err))
			} else if err == nil {
				input.Logger.Info("Namespace deleted", slog.String("name", ns.GetName()))
			}
		}
	}

	// Clean up cluster-wide Istio resources of d8-istio, even if the namespace is already gone.
	// Istio CRs have no finalizers, deleting them is enough.
	istios, err := k8sClient.Dynamic().Resource(istioGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		input.Logger.Warn("Failed to list Istio resources", log.Err(err))
	} else {
		for _, istio := range istios.Items {
			istioInfo, err := parseIstio(&istio)
			if err != nil {
				input.Logger.Warn("Failed to parse Istio", slog.String("name", istio.GetName()), log.Err(err))
				continue
			}
			if istioInfo.Namespace != istioSystemNs {
				continue
			}
			if !isOperatorSupportedRevision(versionMap, istioInfo.Revision) {
				continue
			}
			if istio.GetDeletionTimestamp() != nil {
				continue
			}

			err = k8sClient.Dynamic().Resource(istioGVR).Delete(context.TODO(), istio.GetName(), metav1.DeleteOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}
				input.Logger.Warn("Failed to delete Istio", slog.String("name", istio.GetName()), log.Err(err))
				continue
			}
			input.Logger.Info("Istio deleted", slog.String("name", istio.GetName()))
		}
	}

	// Clean up cluster-wide IstioRevision resources. The operator is gone with the module,
	// so nobody else would remove their finalizers. The istiod resources they own are removed by the GC.
	// The operator may still be running, so:
	// - handle them after the Istio CRs, or the operator recreates them;
	// - delete them before removing finalizers, as finalizers can't be re-added once deletion started.
	istioRevisions, err := k8sClient.Dynamic().Resource(istioRevisionGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		input.Logger.Warn("Failed to list IstioRevision resources", log.Err(err))
	} else {
		for _, istioRevision := range istioRevisions.Items {
			istioRevisionInfo, err := parseIstioRevision(&istioRevision)
			if err != nil {
				input.Logger.Warn("Failed to parse IstioRevision", slog.String("name", istioRevision.GetName()), log.Err(err))
				continue
			}
			// IstioRevisions are cluster-scoped, only touch the ones deploying the control-plane into d8-istio.
			if istioRevisionInfo.Namespace != istioSystemNs {
				continue
			}
			if !isOperatorSupportedRevision(versionMap, istioRevisionInfo.Revision) {
				continue
			}

			if istioRevision.GetDeletionTimestamp() == nil {
				err := k8sClient.Dynamic().Resource(istioRevisionGVR).Delete(context.TODO(), istioRevision.GetName(), metav1.DeleteOptions{})
				if err != nil {
					if k8serrors.IsNotFound(err) {
						continue
					}
					input.Logger.Warn("Failed to delete IstioRevision", slog.String("name", istioRevision.GetName()), log.Err(err))
					continue
				}
				input.Logger.Info("IstioRevision deleted", slog.String("name", istioRevision.GetName()))
			}

			_, err = k8sClient.Dynamic().Resource(istioRevisionGVR).Patch(context.TODO(), istioRevision.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
			if err != nil {
				if k8serrors.IsNotFound(err) {
					continue
				}
				input.Logger.Warn("Failed to remove finalizers from IstioRevision", slog.String("name", istioRevision.GetName()), log.Err(err))
				continue
			}
			input.Logger.Info("Finalizers from IstioRevision removed", slog.String("name", istioRevision.GetName()))
		}
	}

	// delete ClusterRole
	icrs, err := k8sClient.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		input.Logger.Warn("Failed to list ClusterRoles", log.Err(err))
	} else {
		for _, icr := range icrs.Items {
			if !isOperatorResourceByName(versionMap, icr.GetName()) {
				continue
			}
			err := k8sClient.RbacV1().ClusterRoles().Delete(context.TODO(), icr.GetName(), metav1.DeleteOptions{})
			if err != nil {
				input.Logger.Warn("Failed to delete ClusterRole",
					slog.String("name", icr.GetName()),
					log.Err(err))
				continue
			}
			input.Logger.Info("ClusterRole deleted", slog.String("name", icr.GetName()))
		}
	}

	// delete ClusterRoleBinding
	icrbs, err := k8sClient.RbacV1().ClusterRoleBindings().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		input.Logger.Warn("Failed to list ClusterRoleBindings", log.Err(err))
	} else {
		for _, icrb := range icrbs.Items {
			if !isOperatorResourceByName(versionMap, icrb.GetName()) {
				continue
			}
			err := k8sClient.RbacV1().ClusterRoleBindings().Delete(context.TODO(), icrb.GetName(), metav1.DeleteOptions{})
			if err != nil {
				input.Logger.Warn("Failed to delete ClusterRoleBinding",
					slog.String("name", icrb.GetName()),
					log.Err(err))
				continue
			}
			input.Logger.Info("ClusterRoleBinding deleted", slog.String("name", icrb.GetName()))
		}
	}

	// delete MutatingWebhookConfiguration
	imwcs, err := k8sClient.AdmissionregistrationV1().MutatingWebhookConfigurations().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		input.Logger.Warn("Failed to list MutatingWebhookConfigurations", log.Err(err))
	} else {
		for _, imwc := range imwcs.Items {
			if !isOperatorResourceByName(versionMap, imwc.GetName()) {
				continue
			}
			err := k8sClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.TODO(), imwc.GetName(), metav1.DeleteOptions{})
			if err != nil {
				input.Logger.Warn("Failed to delete MutatingWebhookConfiguration",
					slog.String("name", imwc.GetName()),
					log.Err(err))
				continue
			}
			input.Logger.Info("MutatingWebhookConfiguration deleted", slog.String("name", imwc.GetName()))
		}
	}

	// delete ValidatingWebhookConfiguration
	ivwcs, err := k8sClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(context.TODO(), metav1.ListOptions{LabelSelector: istioComponentsLabelSelector})
	if err != nil {
		input.Logger.Warn("Failed to list ValidatingWebhookConfigurations", log.Err(err))
	} else {
		for _, ivwc := range ivwcs.Items {
			if !isOperatorResourceByName(versionMap, ivwc.GetName()) {
				continue
			}
			err := k8sClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(context.TODO(), ivwc.GetName(), metav1.DeleteOptions{})
			if err != nil {
				input.Logger.Warn("Failed to delete ValidatingWebhookConfiguration",
					slog.String("name", ivwc.GetName()),
					log.Err(err))
				continue
			}
			input.Logger.Info("ValidatingWebhookConfiguration deleted", slog.String("name", ivwc.GetName()))
		}
	}

	return nil
}
