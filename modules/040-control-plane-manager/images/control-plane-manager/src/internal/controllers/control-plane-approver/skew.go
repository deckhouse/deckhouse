/*
Copyright 2026 Flant JSC

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

package controlplaneapprover

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

// isSkewSensitive reports whether the component reaches kube-apiservers through
// kubernetes-api-proxy and so must not be newer than any of them.
func isSkewSensitive(c controlplanev1alpha1.OperationComponent) bool {
	return c == controlplanev1alpha1.OperationComponentKubeControllerManager ||
		c == controlplanev1alpha1.OperationComponentKubeScheduler
}

// holdBackVersionSkew drops not-yet-approved kube-controller-manager/kube-scheduler operations
// whose target Kubernetes version is not yet run by every kube-apiserver: every existing
// kube-apiserver mirror pod must report it, and every ready master must have such a pod.
// The apiserver stage gate already holds them while any kube-apiserver operation is in flight;
// this covers the window before the kube-apiserver operations of a version change are created.
// It reports whether anything was held back, so the caller can retry later.
func (r *reconciler) holdBackVersionSkew(
	ctx context.Context,
	ops []controlplanev1alpha1.ControlPlaneOperation,
	masters []string,
) ([]controlplanev1alpha1.ControlPlaneOperation, bool, error) {
	if !slices.ContainsFunc(ops, isPendingSkewSensitive) {
		return ops, false, nil
	}
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: constants.KubeSystemNamespace, Name: constants.ControlPlaneManagerConfigSecretName}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}
		// Keep approving etcd/kube-apiserver; only the version-sensitive operations wait.
		logger.Error(err, "holding back kube-controller-manager/kube-scheduler operations: cannot read their target Kubernetes version")
		return slices.DeleteFunc(slices.Clone(ops), isPendingSkewSensitive), true, nil
	}

	pods := &corev1.PodList{}
	if err := r.client.List(ctx, pods,
		client.InNamespace(constants.KubeSystemNamespace),
		client.MatchingLabels{constants.StaticPodComponentLabelKey: controlplanev1alpha1.OperationComponentKubeAPIServer.PodComponentName()},
	); err != nil {
		return nil, false, err
	}
	versions := apiServerVersions(pods.Items)

	kept := make([]controlplanev1alpha1.ControlPlaneOperation, 0, len(ops))
	heldBack := false
	for _, op := range ops {
		if !isPendingSkewSensitive(op) {
			kept = append(kept, op)
			continue
		}

		key := op.Spec.Component.SecretKey()
		desired := manifestKubernetesVersion(secret.Data[key])
		switch {
		case desired == "":
			logger.Error(nil, "holding back operation: component template has no kubernetes-version annotation",
				"operation", op.Name, "secret", constants.ControlPlaneManagerConfigSecretName, "key", key,
				"annotation", constants.KubernetesVersionAnnotationKey)
			heldBack = true
		case !allRunVersion(versions, masters, desired):
			logger.Info("holding back operation until every kube-apiserver runs its Kubernetes version",
				"operation", op.Name, "desiredVersion", desired, "kubeAPIServerVersions", versions, "readyMasters", masters)
			heldBack = true
		default:
			kept = append(kept, op)
		}
	}
	return kept, heldBack, nil
}

func isPendingSkewSensitive(op controlplanev1alpha1.ControlPlaneOperation) bool {
	return !op.Spec.Approved && isSkewSensitive(op.Spec.Component)
}

// allRunVersion requires every kube-apiserver (including ones on NotReady masters) to run version
// and every ready master to have a kube-apiserver.
func allRunVersion(versionByNode map[string]string, masters []string, version string) bool {
	for _, v := range versionByNode {
		if v != version {
			return false
		}
	}
	for _, node := range masters {
		if _, ok := versionByNode[node]; !ok {
			return false
		}
	}
	return true
}

// apiServerVersions maps node name to the Kubernetes version of its kube-apiserver mirror pod.
func apiServerVersions(pods []corev1.Pod) map[string]string {
	versions := make(map[string]string, len(pods))
	for _, pod := range pods {
		versions[pod.Spec.NodeName] = pod.Annotations[constants.KubernetesVersionAnnotationKey]
	}
	return versions
}

// manifestKubernetesVersion returns the kubernetes-version annotation of a static pod manifest
// template from d8-control-plane-manager-config, or "" if it cannot be read.
func manifestKubernetesVersion(manifest []byte) string {
	var obj metav1.PartialObjectMetadata
	if err := yaml.Unmarshal(manifest, &obj); err != nil {
		return ""
	}
	return obj.Annotations[constants.KubernetesVersionAnnotationKey]
}
