// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package registry

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

const (
	// storageStatefulSetName and storageObjectName are what the module calls the store it runs in the
	// cluster: a StatefulSet in d8-system, and the one cluster-scoped RegistryStorage that reports on
	// it.
	storageStatefulSetName = "registry-storage"
	storageObjectName      = "registry"

	// bootstrapStaticPodPath is the registry the bootstrap steps put on the first master. It is the
	// same file the previous implementation manages on a legacy cluster, which is why it is removed
	// here only for an installation from a bundle.
	bootstrapStaticPodPath = "/etc/kubernetes/manifests/registry-nodeservices.yaml"

	// crictlPath is where the node's own bundle installs crictl. Used rather than a bare name because
	// the command runs through sudo, whose PATH does not include it.
	crictlPath = "/opt/deckhouse/bin/crictl"
)

// storageGVR is the RegistryStorage resource, read to find out whether the store has taken over.
var storageGVR = schema.GroupVersionResource{
	Group:    "deckhouse.io",
	Version:  "v1alpha1",
	Resource: "registrystorages",
}

// HandOverBundleStore gives the images the bootstrap left on the first master to the cluster's own
// store, for an installation from a bundle. A no-op for every other installation.
//
// Nothing moves. The bootstrap fills /opt/deckhouse/registry/local_data on the node and the store
// mounts that same directory, so what has to change hands is not the data but the address: both serve
// port 5001 on the host network, and while the bootstrap's static pod holds it the store's pod is not
// merely unable to start — the scheduler will not place it at all ("node(s) didn't have free ports for
// the requested pod ports"), which is what a first installation from a bundle actually did.
//
// The order below is the whole content of this function and it is not interchangeable. Freeing the port
// is the last step, because once it is free nothing on the node serves images, and the store's own
// containers are pulled by digest from the very registry that just stopped answering. So they are
// pulled while it still answers.
func HandOverBundleStore(ctx context.Context, kubeCl *client.KubernetesClient, config Config) error {
	if !config.BundleBootstrap {
		return nil
	}

	// Refused rather than skipped when there is no way to reach the node.
	//
	// Both things this function does are commands on the first master, and a caller that cannot run
	// them cannot hand the store over — so treating their absence as "nothing to do" reports success
	// for work that did not happen. Measured: the handover finished in 18 milliseconds, the store's pod
	// stayed Pending on the port the bootstrap registry still held, and the installation spent the next
	// thirty-three minutes waiting for a store that could never start. The wait's own message —
	// "the cluster store is Idle" — described the symptom and named nothing that caused it.
	if kubeCl.NodeInterface == nil {
		return ErrHandoverNeedsTheNode
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Hand the store over to the cluster", func(ctx context.Context) error {
		images, err := waitForStorageImages(ctx, kubeCl)
		if err != nil {
			return err
		}

		if err := prePullOnFirstMaster(ctx, kubeCl, images); err != nil {
			return err
		}

		return removeBootstrapRegistry(ctx, kubeCl)
	})
}

// waitForStorageImages waits until the module has rendered its store and returns the images its pod
// needs. Waiting rather than reading once: the StatefulSet appears when Deckhouse starts the module,
// which can be after this point.
func waitForStorageImages(ctx context.Context, kubeCl *client.KubernetesClient) ([]string, error) {
	var images []string

	err := retry.NewLoop("Waiting for the cluster store to be declared", 60, 10*time.Second).
		RunContext(ctx, func() error {
			sts, err := kubeCl.AppsV1().
				StatefulSets(secretsNamespace).
				Get(ctx, storageStatefulSetName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("get statefulset '%s/%s': %w", secretsNamespace, storageStatefulSetName, err)
			}

			images = podImages(sts.Spec.Template.Spec)
			if len(images) == 0 {
				return fmt.Errorf("statefulset '%s/%s' declares no images", secretsNamespace, storageStatefulSetName)
			}

			return nil
		})

	return images, err
}

// podImages lists every image a pod spec needs, init containers included.
//
// Read from the spec rather than assembled from digests known to dhctl, so that a container added to
// the store is pre-pulled without anybody having to remember this function exists. Forgetting would
// not fail here: it would leave the store unable to pull that one image after the port is freed, with
// nothing left to pull it from.
func podImages(spec corev1.PodSpec) []string {
	images := make([]string, 0, len(spec.InitContainers)+len(spec.Containers))

	seen := make(map[string]struct{}, cap(images))
	add := func(image string) {
		if image == "" {
			return
		}
		if _, ok := seen[image]; ok {
			return
		}
		seen[image] = struct{}{}
		images = append(images, image)
	}

	for _, container := range spec.InitContainers {
		add(container.Image)
	}
	for _, container := range spec.Containers {
		add(container.Image)
	}

	return images
}

// pullArgs is the pre-pull command, as a program and its arguments rather than as a line for a shell.
//
// It matters, and it was measured on a node: the command used to be built as `bash -c "<crictl> pull
// <ref>"`, and the node interface wraps whatever it is given in a shell of its own. The inner bash then
// received the pull and the reference as $0 and $1 instead of as arguments, so what ran was crictl with
// no arguments at all — which prints its usage and exits 0.
//
//	# bash -c 'bash -c /opt/deckhouse/bin/crictl pull "<ref>"'; echo $?
//	NAME:
//	   crictl - client for CRI
//	0
//
// So the handover reported four images pulled, pulled none of them, and the store's own syncer image
// was then unobtainable: the bootstrap registry that would have served it had just been removed on the
// strength of that success. Passing argv avoids the question entirely — there is no shell to quote for.
func pullArgs(image string) []string {
	return []string{crictlPath, "pull", image}
}

// prePullOnFirstMaster puts the store's images on the node while the bootstrap registry can still
// serve them.
//
// Three of the four are already there in practice — the bootstrap's own static pod runs two of them —
// but "in practice" is exactly the kind of assumption that turns into an unrecoverable installation,
// so all of them are asked for. A pull of an image already present costs nothing.
func prePullOnFirstMaster(ctx context.Context, kubeCl *client.KubernetesClient, images []string) error {
	for _, image := range images {
		args := pullArgs(image)

		cmd := kubeCl.NodeInterface.Command(args[0], args[1:]...)
		cmd.Sudo(ctx)
		cmd.WithTimeout(10 * time.Minute)

		if _, err := cmd.CombinedOutput(ctx); err != nil {
			return fmt.Errorf(
				"pre-pull %s on the first master: %w. "+
					"The images must be on the node before the bootstrap registry stops serving them, "+
					"because afterwards there is nothing left on this node to pull them from",
				image, err,
			)
		}
	}

	return nil
}

// removeBootstrapRegistry frees the port the store needs by removing the bootstrap's static pod
// manifest. The kubelet stops the pod on its own; the data it served stays on disk.
func removeBootstrapRegistry(ctx context.Context, kubeCl *client.KubernetesClient) error {
	cmd := kubeCl.NodeInterface.Command("rm", "-f", bootstrapStaticPodPath)
	cmd.Sudo(ctx)
	cmd.WithTimeout(1 * time.Minute)

	if err := cmd.Run(ctx); err != nil {
		return fmt.Errorf("remove the bootstrap registry manifest %s: %w", bootstrapStaticPodPath, err)
	}

	return nil
}

// isStoreReady reports whether the store the module runs in the cluster has come up.
//
// Asked of the store itself, not of the previous implementation's state secret. That secret is
// written from values the current implementation clears as soon as it owns the cluster, so on every
// cluster installed from now on it is never written at all — and a wait on it is a wait on nobody.
//
// requireFull demands a complete copy of the image set in addition to the store being Ready. See the
// caller: it is required when the store is the only source of images and not otherwise.
func isStoreReady(ctx context.Context, kubeCl client.KubeClient, requireFull bool) error {
	object, err := kubeCl.Dynamic().
		Resource(storageGVR).
		Get(ctx, storageObjectName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("%w: get RegistryStorage '%s': %w", errRegistryCheckTransient, storageObjectName, err)
	}

	return storeStatusReady(object.Object, requireFull)
}

// storeStatusReady judges a RegistryStorage status.
//
// A store can be Ready while still filling, which is why fullness is a separate question. The
// message names what is missing, because for an air-gapped cluster this is the last thing an operator
// sees before nodes start joining that have nowhere else to pull from.
func storeStatusReady(object map[string]any, requireFull bool) error {
	phase, _, err := unstructured.NestedString(object, "status", "phase")
	if err != nil {
		return fmt.Errorf("read the store phase: %w", err)
	}

	full, _, err := unstructured.NestedBool(object, "status", "allReplicasFull")
	if err != nil {
		return fmt.Errorf("read whether the store replicas are full: %w", err)
	}

	switch {
	case phase != "Ready":
		if phase == "" {
			phase = "not reported yet"
		}
		return fmt.Errorf("%w: the cluster store is %s", ErrIsNotReady, phase)

	case requireFull && !full:
		filled, _, _ := unstructured.NestedInt64(object, "status", "fill", "filled")
		total, _, _ := unstructured.NestedInt64(object, "status", "fill", "total")
		return fmt.Errorf("%w: the cluster store does not hold the whole set yet (%d of %d)",
			ErrIsNotReady, filled, total)
	}

	return nil
}
