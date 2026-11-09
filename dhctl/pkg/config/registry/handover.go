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

	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/retry"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

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

	// cloudUninitializedTaint is what a cloud node carries until the cloud-controller-manager has
	// initialized it. The store does not tolerate it, and must not: see waitForStoreSchedulable.
	cloudUninitializedTaint = "node.cloudprovider.kubernetes.io/uninitialized"

	// controlPlaneLabel is where the store runs when its pod spec names no nodes of its own. The
	// module's template names them with this same label once discovery has counted the masters, and
	// renders no selector before that; the fallback keeps an empty selector from meaning "every node
	// in the cluster".
	controlPlaneLabel = "node-role.kubernetes.io/control-plane"
)

// storeSchedulableAttempts and storeSchedulableWait bound waitForStoreSchedulable: ten minutes for
// the cloud-controller-manager to initialize the first master. storeSchedulableReportEvery is how many
// refused looks pass between two of its progress lines: one a minute.
const (
	storeSchedulableAttempts    = 60
	storeSchedulableReportEvery = 6
)

// storeSchedulableWait is a variable only so that a test can run the whole wait without spending
// those ten minutes.
var storeSchedulableWait = 10 * time.Second

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
// pulled while it still answers. And the port is freed only for a store that can be placed on the node
// at all: freeing it for a pod the scheduler will not place trades the only registry on the node for
// none.
func HandOverBundleStore(ctx context.Context, kubeCl *client.KubernetesClient, config Config) error {
	if !config.BundleBootstrap {
		return nil
	}

	// Refused rather than skipped when there is no way to reach the node.
	//
	// Both things this function does are commands on the first master, and a caller that cannot run
	// them cannot hand the store over — so treating their absence as "nothing to do" reports success
	// for work that did not happen, and the installation then waits on a store that can never start,
	// with only "the cluster store is Idle" to go on.
	if kubeCl.NodeInterface == nil {
		return ErrHandoverNeedsTheNode
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Hand the store over to the cluster", func(ctx context.Context) error {
		spec, err := waitForStoragePodSpec(ctx, kubeCl)
		if err != nil {
			return err
		}

		if err := waitForStoreSchedulable(ctx, kubeCl, spec); err != nil {
			return err
		}

		if err := prePullOnFirstMaster(ctx, kubeCl, podImages(spec)); err != nil {
			return err
		}

		return removeBootstrapRegistry(ctx, kubeCl)
	})
}

// waitForStoragePodSpec waits until the module has rendered its store and returns the pod spec of it:
// the images the pod needs, and where it may run. Waiting rather than reading once: the StatefulSet
// appears when Deckhouse starts the module, which can be after this point.
func waitForStoragePodSpec(ctx context.Context, kubeCl *client.KubernetesClient) (corev1.PodSpec, error) {
	var spec corev1.PodSpec

	err := retry.NewLoop("Waiting for the cluster store to be declared", 60, 10*time.Second).
		RunContext(ctx, func() error {
			sts, err := kubeCl.AppsV1().
				StatefulSets(secretsNamespace).
				Get(ctx, storageStatefulSetName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("get statefulset '%s/%s': %w", secretsNamespace, storageStatefulSetName, err)
			}

			if len(podImages(sts.Spec.Template.Spec)) == 0 {
				return fmt.Errorf("statefulset '%s/%s' declares no images", secretsNamespace, storageStatefulSetName)
			}

			spec = sts.Spec.Template.Spec
			return nil
		})

	return spec, err
}

// waitForStoreSchedulable waits until the store's pod can be placed on the first master as far as
// taints go, and refuses the handover when it cannot.
//
// In a cloud the first master starts with node.cloudprovider.kubernetes.io/uninitialized, and only
// the cloud-controller-manager removes it. The store does not tolerate that taint, and the fix is not
// to make it: before the cloud has initialized the node it may have no InternalIP, and the syncer
// refuses to start without an address to listen on. Nothing earlier in the bootstrap guarantees the
// taint is gone by now either. The readiness wait that precedes the handover is on the Deckhouse pod,
// which tolerates every taint, so a cloud-controller-manager that is slow — or one that cannot reach
// the cloud API at all, which is a real possibility in a cluster installed from a bundle precisely
// because it is cut off — lets the bootstrap get here with the taint still in place. Removing the
// bootstrap registry at that point leaves a store that stays Pending, and a node on which nothing
// serves images any more, the cloud-controller-manager's own included if it is not pulled yet. The
// bootstrap registry is the one thing on the node that can still put the cluster right, so it is kept.
//
// Asked of the store's own tolerations rather than of that one taint: any NoSchedule or NoExecute taint
// the pod does not tolerate keeps it off the node just the same, and the template is the authority on
// what it tolerates. A static cluster carries nothing the store does not tolerate, so there this
// returns on the first look.
//
// Every node the store's selector matches is asked. During a bootstrap that is the first master alone:
// the other masters join after the handover.
//
// The reason for the wait is put in front of the operator on the first refused look and once a minute
// after that. The loop itself reports each attempt only to the debug log, so without these lines the
// terminal would show the loop's name alone for up to ten minutes and say why only at the timeout.
func waitForStoreSchedulable(ctx context.Context, kubeCl client.KubeClient, spec corev1.PodSpec) error {
	logger := dhlog.FromContext(ctx)
	limit := time.Duration(storeSchedulableAttempts) * storeSchedulableWait
	attempt := 0

	return retry.NewLoop("Waiting for the first master to accept the cluster store", storeSchedulableAttempts, storeSchedulableWait).
		RunContext(ctx, func() error {
			attempt++

			err := storeSchedulable(ctx, kubeCl, spec)
			if err == nil || (attempt-1)%storeSchedulableReportEvery != 0 {
				return err
			}

			msg := fmt.Sprintf("Waiting up to %s for the first master to accept the cluster store: %v", limit, err)
			if attempt > 1 {
				waited := time.Duration(attempt-1) * storeSchedulableWait
				msg = fmt.Sprintf("Still waiting for the first master to accept the cluster store (%s of %s): %v",
					waited, limit, err)
			}
			logger.InfoContext(ctx, msg, dhlog.ShowInCompacted())

			return err
		})
}

// storeSchedulable is one look at the nodes the store runs on. See waitForStoreSchedulable.
func storeSchedulable(ctx context.Context, kubeCl client.KubeClient, spec corev1.PodSpec) error {
	selector := labels.SelectorFromSet(spec.NodeSelector)
	if len(spec.NodeSelector) == 0 {
		selector = labels.SelectorFromSet(labels.Set{controlPlaneLabel: ""})
	}

	nodes, err := kubeCl.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return fmt.Errorf("list the nodes the cluster store runs on (%s): %w", selector, err)
	}

	if len(nodes.Items) == 0 {
		return fmt.Errorf("no node matches the cluster store's node selector %s yet", selector)
	}

	for i := range nodes.Items {
		node := &nodes.Items[i]

		taint := untoleratedTaint(node.Spec.Taints, spec.Tolerations)
		if taint == nil {
			continue
		}

		if taint.Key == cloudUninitializedTaint {
			return fmt.Errorf(
				"the cloud controller manager has not initialized node %s (taint %s is still set), "+
					"the bootstrap registry is kept: the cluster store cannot be placed on the node until it is, "+
					"and without the bootstrap registry nothing on the node would serve images. "+
					"Check that the cloud-controller-manager is running and can reach the cloud API",
				node.Name, taint.ToString())
		}

		return fmt.Errorf(
			"node %s has taint %s that the cluster store does not tolerate, the bootstrap registry is kept: "+
				"the store cannot be placed on the node while the taint is there, "+
				"and without the bootstrap registry nothing on the node would serve images",
			node.Name, taint.ToString())
	}

	return nil
}

// untoleratedTaint returns the first taint that keeps a pod with the given tolerations off a node, or
// nil when there is none. PreferNoSchedule only steers the scheduler and keeps nothing off.
func untoleratedTaint(taints []corev1.Taint, tolerations []corev1.Toleration) *corev1.Taint {
	for i := range taints {
		taint := &taints[i]

		if taint.Effect != corev1.TaintEffectNoSchedule && taint.Effect != corev1.TaintEffectNoExecute {
			continue
		}

		tolerated := false
		for j := range tolerations {
			if tolerates(&tolerations[j], taint) {
				tolerated = true
				break
			}
		}

		if !tolerated {
			return taint
		}
	}

	return nil
}

// tolerates is corev1.Toleration.ToleratesTaint as k8s.io/api v0.34 has it.
//
// Not that method itself: this package is compiled both by dhctl's own module and, through the
// replace in the root go.mod, by deckhouse-controller, and the two pin different k8s.io/api lines.
// From v0.35 the method takes a logger and a flag for the Lt/Gt operators, so no single call
// compiles in both. The rule is small and stable; the operators v0.35 adds sit behind a feature
// gate that is off, and an unknown operator tolerates nothing either way.
func tolerates(toleration *corev1.Toleration, taint *corev1.Taint) bool {
	if toleration.Effect != "" && toleration.Effect != taint.Effect {
		return false
	}
	if toleration.Key != "" && toleration.Key != taint.Key {
		return false
	}

	switch toleration.Operator {
	case "", corev1.TolerationOpEqual:
		return toleration.Value == taint.Value
	case corev1.TolerationOpExists:
		return true
	default:
		return false
	}
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
// The node interface wraps whatever it is given in a shell of its own, so a command built as
// `bash -c "<crictl> pull <ref>"` hands the inner bash the pull and the reference as $0 and $1 instead
// of as arguments: what runs is crictl with no arguments at all, which prints its usage and exits 0.
// The handover would then report images pulled, pull none of them, and leave the store's own syncer
// image unobtainable — the bootstrap registry that would have served it is removed on the strength of
// that success. Passing argv avoids the question entirely: there is no shell to quote for.
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
