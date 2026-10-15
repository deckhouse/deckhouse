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
	"maps"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

// holdBackUnpulledImages drops operations whose template images are not yet held on their node
// and reports whether any were dropped.
//
// kubelet pulls a static pod image only after stopping the old pod, so the component is down for
// the whole pull. A running container keeps its image on the node: an image holder of the
// control-plane-manager Pod, or the static pod of the component when the template keeps its image
// (e.g. a certificate renewal). The holders change in the same Helm release as the templates and
// may still run the old images.
//
// A dropped operation keeps its slot from SelectApprovable, so operations behind it don't overtake it.
func (r *reconciler) holdBackUnpulledImages(
	ctx context.Context,
	ops []controlplanev1alpha1.ControlPlaneOperation,
) ([]controlplanev1alpha1.ControlPlaneOperation, bool, error) {
	if !slices.ContainsFunc(ops, writesTemplate) {
		return ops, false, nil
	}
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: constants.KubeSystemNamespace, Name: constants.ControlPlaneManagerConfigSecretName}, secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, false, err
		}
		logger.Error(err, "holding back operations that write static pod manifests: cannot read their images")
		return slices.DeleteFunc(slices.Clone(ops), writesTemplate), true, nil
	}

	pods := &corev1.PodList{}
	if err := r.client.List(ctx, pods,
		client.InNamespace(constants.KubeSystemNamespace),
		client.MatchingLabels{constants.StaticPodComponentLabelKey: constants.ControlPlaneManagerComponentLabelValue},
	); err != nil {
		return nil, false, err
	}
	held := heldImages(pods.Items)

	kept := make([]controlplanev1alpha1.ControlPlaneOperation, 0, len(ops))
	heldBack := false
	for _, op := range ops {
		if !writesTemplate(op) {
			kept = append(kept, op)
			continue
		}

		running, err := r.getStaticPodImages(ctx, op)
		if err != nil {
			return nil, false, err
		}
		key := op.Spec.Component.SecretKey()
		images := manifestImages(secret.Data[key])
		missing := slices.DeleteFunc(slices.Clone(images), func(image string) bool {
			digest := imageDigest(image)
			return held[op.Spec.NodeName][digest] || running[digest]
		})
		switch {
		case len(images) == 0:
			logger.Error(nil, "holding back operation: component template has no images",
				"operation", op.Name, "secret", constants.ControlPlaneManagerConfigSecretName, "key", key)
			heldBack = true
		case len(missing) > 0:
			logger.Info("holding back operation until its images are held on its node",
				"operation", op.Name, "node", op.Spec.NodeName, "images", missing)
			heldBack = true
		default:
			kept = append(kept, op)
		}
	}
	return kept, heldBack, nil
}

// writesTemplate reports whether op writes the static pod manifest from its template.
// SyncManifests without a config checksum only updates the checksum annotations.
func writesTemplate(op controlplanev1alpha1.ControlPlaneOperation) bool {
	return slices.Contains(op.Spec.Steps, controlplanev1alpha1.StepJoinEtcdCluster) ||
		op.Spec.DesiredConfigChecksum != "" && slices.Contains(op.Spec.Steps, controlplanev1alpha1.StepSyncManifests)
}

// heldImages maps node name to the image digests of the running containers of pods.
func heldImages(pods []corev1.Pod) map[string]map[string]bool {
	held := make(map[string]map[string]bool)
	for i := range pods {
		node := pods[i].Spec.NodeName
		if held[node] == nil {
			held[node] = make(map[string]bool)
		}
		maps.Copy(held[node], runningImages(&pods[i]))
	}
	return held
}

// runningImages returns the image digests of the running containers of pod. A terminating pod has
// none: its images become garbage collectable once it stops. Images are taken from the spec because
// the container status may omit the digest.
func runningImages(pod *corev1.Pod) map[string]bool {
	images := make(map[string]bool)
	if pod.DeletionTimestamp != nil {
		return images
	}
	running := make(map[string]bool, len(pod.Status.ContainerStatuses))
	for _, status := range pod.Status.ContainerStatuses {
		running[status.Name] = status.State.Running != nil
	}
	for _, container := range pod.Spec.Containers {
		if running[container.Name] {
			images[imageDigest(container.Image)] = true
		}
	}
	return images
}

// manifestImages returns the container images of a static pod manifest template, or nil if it
// cannot be parsed. Only images are decoded: other fields of an unexpanded template may not parse.
func manifestImages(manifest []byte) []string {
	var pod struct {
		Spec struct {
			InitContainers []struct {
				Image string `json:"image"`
			} `json:"initContainers"`
			Containers []struct {
				Image string `json:"image"`
			} `json:"containers"`
		} `json:"spec"`
	}
	if err := yaml.Unmarshal(manifest, &pod); err != nil {
		return nil
	}
	var images []string
	for _, c := range slices.Concat(pod.Spec.InitContainers, pod.Spec.Containers) {
		if c.Image != "" {
			images = append(images, c.Image)
		}
	}
	return images
}

// imageDigest returns the digest of an image reference, or the reference itself if it has none.
// Holders and templates reference the same digest from different repositories.
func imageDigest(image string) string {
	if i := strings.LastIndex(image, "@"); i >= 0 {
		return image[i+1:]
	}
	return image
}

// getStaticPodImages returns the image digests run by the static pod of op's component on its node,
// or none if the node runs no such pod. A template that keeps its image is held by this pod.
func (r *reconciler) getStaticPodImages(ctx context.Context, op controlplanev1alpha1.ControlPlaneOperation) (map[string]bool, error) {
	pod := &corev1.Pod{}
	key := client.ObjectKey{Namespace: constants.KubeSystemNamespace, Name: op.Spec.Component.MirrorPodName(op.Spec.NodeName)}
	if err := r.client.Get(ctx, key, pod); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return runningImages(pod), nil
}
