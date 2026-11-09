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

package storageupdate

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// holding is what the image holder on a node runs of the images a replica needs.
//
// Compared by reference, as written in the two pod specs, and not by digest. The holder and the
// StatefulSet take their images from one define in the module, so the references are equal byte for
// byte when the images are, and a container in Running state has its image on the node. The
// control-plane-manager compares digests because its holders and its manifests name different
// repositories; here that would only add the runtime's idea of an image ID, which for containerd can
// be the digest of the index rather than of the manifest, and gain nothing.
type holding struct {
	node string

	// pod is the holder on the node, nil when it has none.
	pod *corev1.Pod

	// leaving is set when the node has no holder but one on its way out: the DaemonSet is rolling
	// it, and deletes the old pod before it creates the new one.
	leaving bool

	// missing is the images of the replica's template the holder does not run yet, in template
	// order.
	missing []string

	// notInSpec is set when some of them are not even in the holder's spec: its DaemonSet has not
	// rolled to the new images yet.
	notInSpec bool

	// waiting is the kubelet's reason for a missing image not running, empty if it gave none: that
	// of the first one, unless another one has failed to pull or to run, which is the one worth
	// reporting.
	waiting string

	// pullFailed is the missing image the kubelet failed to pull, empty while none has failed. Kept
	// apart from the order, because an image still being created ahead of it in the template would
	// otherwise hide the one failure an operator can do something about.
	pullFailed string

	// runFailed is the missing image the kubelet has pulled but cannot run, empty while none: the
	// container crashes or cannot be created. Not a wait either, and not something a push changes,
	// so it is reported apart from both — after a failed pull, which is the one an operator can fix.
	runFailed string
}

// heldOn looks at what the image holder on node runs of the images in set's pod template — the
// update revision's, since under OnDelete the template is always the newest.
//
// From the cache, unlike the replicas: the holders are pods in the module's namespace, which this
// controller already watches and may read, and a change to one wakes it up. A holder read a moment
// late cannot make a pass delete twice — the replica it gates is read past the cache, and is seen
// going — so what reading the holders from the API server would buy is a pass that notices a
// finished pull a few milliseconds sooner.
func (r *Reconciler) heldOn(ctx context.Context, node string, set *appsv1.StatefulSet) (holding, error) {
	list := &corev1.PodList{}
	err := r.Client.List(ctx, list,
		client.InNamespace(Namespace),
		client.MatchingLabels{"app": holderAppLabel})
	if err != nil {
		return holding{}, fmt.Errorf("listing the image holders: %w", err)
	}

	held := holding{node: node}
	for i := range list.Items {
		pod := &list.Items[i]
		if pod.Spec.NodeName != node {
			continue
		}
		// One that is going holds nothing worth counting: it is about to stop holding it, and the
		// image garbage collector may take it after that. It does say why the node has no other.
		if pod.DeletionTimestamp != nil {
			held.leaving = true
			continue
		}
		held.pod = pod
		held.leaving = false
		break
	}

	wanted := templateImages(set)
	if held.pod == nil {
		held.missing = wanted
		return held, nil
	}

	statuses := map[string]*corev1.ContainerStatus{}
	for i := range held.pod.Status.ContainerStatuses {
		status := &held.pod.Status.ContainerStatuses[i]
		statuses[status.Name] = status
	}

	running := map[string]bool{}
	inSpec := map[string]bool{}
	waiting := map[string]string{}
	// exited are the images whose container has stopped: between two restarts of a crash loop the
	// kubelet shows it terminated before it shows the back-off. Created from the image, so it is a
	// failure to run like the back-off, not a pull.
	exited := map[string]bool{}
	for _, container := range held.pod.Spec.Containers {
		inSpec[container.Image] = true
		status := statuses[container.Name]
		switch {
		case status == nil:
		case status.State.Running != nil:
			running[container.Image] = true
		case status.State.Waiting != nil && waiting[container.Image] == "":
			waiting[container.Image] = status.State.Waiting.Reason
		case status.State.Terminated != nil && waiting[container.Image] == "":
			waiting[container.Image] = cmp.Or(status.State.Terminated.Reason, "Terminated")
			exited[container.Image] = true
		}
	}

	for _, image := range wanted {
		if running[image] {
			continue
		}
		held.missing = append(held.missing, image)
		if !inSpec[image] {
			held.notInSpec = true
		}
		switch reason := waiting[image]; {
		case held.pullFailed == "" && pullFailure(reason):
			held.pullFailed, held.waiting = image, reason
		case held.pullFailed == "" && held.runFailed == "" && (runFailure(reason) || exited[image]):
			held.runFailed, held.waiting = image, reason
		case held.waiting == "":
			held.waiting = reason
		}
	}
	return held, nil
}

// pullFailure is whether the kubelet's waiting reason says it tried to pull the image and failed —
// rather than that it has not got round to it yet.
func pullFailure(reason string) bool {
	return reason == "ErrImagePull" || reason == "ImagePullBackOff"
}

// runFailure is whether the kubelet's waiting reason says it has the image and failed to run the
// container: each of these comes only after the image is on the node, so it is not pulling any more.
func runFailure(reason string) bool {
	switch reason {
	case "CrashLoopBackOff", "CreateContainerConfigError", "CreateContainerError", "RunContainerError":
		return true
	}
	return false
}

// complete is whether the replica could be replaced and start from what is on the node.
func (h holding) complete() bool {
	return h.pod != nil && len(h.missing) == 0 && podReady(h.pod)
}

// refusal says what is missing and what ends the wait.
//
// `d8 mirror push` is suggested in one case only: the holder failing to pull. The holder pulls from
// this very cache, so an image it cannot get is most likely not in the cache — and pushing it in is
// then what lets the holder pull it and the update continue. Everywhere else the holder is either
// not done yet or has the image and fails to run it, and telling an operator to push images that
// are already there sends them to do work that changes nothing. The failure to run is still said
// as a failure: it does not end by waiting, and reading as a pull in progress it would never be
// looked at.
//
// A node without a holder is no exception, and not by itself a fault either: every roll of the
// DaemonSet deletes the old pod before it creates the new one, and on the update that introduces
// the holder the StatefulSet may be looked at before the DaemonSet has created any pod at all. So
// that is said as a wait, with scheduling named only as what to look at if it does not end.
func (h holding) refusal(going *corev1.Pod) *refusal {
	prefix := fmt.Sprintf("Not replacing %s: it is the only replica of an air-gapped cache, so nothing could "+
		"serve its new images while it is away, and they must be on %s beforehand", going.Name, h.node)

	switch {
	case h.pod == nil && h.leaving:
		return &refusal{
			reason: BlockedHolderNotReady,
			message: fmt.Sprintf("%s. The image holder there is being replaced as DaemonSet d8-system/%s "+
				"rolls; the update continues once the new one runs every new image.", prefix, holderDaemonSet),
		}

	case h.pod == nil:
		return &refusal{
			reason: BlockedSingleReplicaWithoutHolder,
			message: fmt.Sprintf("%s. They are kept there by the image holder, and DaemonSet d8-system/%s has "+
				"no pod on %s yet. It creates one there as it rolls out, and the update continues once that "+
				"pod runs every new image; if none appears within minutes, find out why the DaemonSet does "+
				"not schedule a pod on %s.", prefix, holderDaemonSet, h.node, h.node),
		}

	case h.notInSpec:
		return &refusal{
			reason: BlockedHolderNotReady,
			message: fmt.Sprintf("%s. The image holder %s still runs the previous images; the update "+
				"continues once DaemonSet d8-system/%s has rolled it to the new ones.",
				prefix, h.pod.Name, holderDaemonSet),
		}

	case h.pullFailed != "":
		return &refusal{
			reason: BlockedHolderNotReady,
			message: fmt.Sprintf("%s. The image holder %s cannot pull %s (%s). It pulls from this cache, so "+
				"the image is most likely not in it: push the Deckhouse Platform release this cluster is "+
				"updating to with d8 mirror push, and the holder will pull it on its next retry. The pull "+
				"error itself is in the events of pod d8-system/%s.",
				prefix, h.pod.Name, h.pullFailed, h.waiting, h.pod.Name),
			// Not the kubelet's reason, which goes from ErrImagePull to ImagePullBackOff and back on every
			// retry of the pull, each change of the holder waking a pass: the same failure, said again.
			state: "cannot pull " + h.pullFailed,
		}

	case h.runFailed != "":
		return &refusal{
			reason: BlockedHolderNotReady,
			message: fmt.Sprintf("%s. The image holder %s has pulled %s but fails to run it (%s). The image "+
				"is on the node, so pushing it changes nothing: the cause is in the events and the "+
				"container statuses of pod d8-system/%s, and the update continues once the holder runs "+
				"every new image.",
				prefix, h.pod.Name, h.runFailed, h.waiting, h.pod.Name),
			// Not the kubelet's reason either: a crash loop shows the container terminated, then in
			// CrashLoopBackOff, on every restart, each change waking a pass. The same failure.
			state: "cannot run " + h.runFailed,
		}

	case len(h.missing) > 0:
		state := "not running yet"
		if h.waiting != "" {
			state = h.waiting
		}
		return &refusal{
			reason: BlockedHolderNotReady,
			message: fmt.Sprintf("%s. The image holder %s is pulling them (%s: %s); the update continues "+
				"once it runs all of them.", prefix, h.pod.Name, strings.Join(h.missing, ", "), state),
		}

	default:
		return &refusal{
			reason: BlockedHolderNotReady,
			message: fmt.Sprintf("%s. The image holder %s runs all of them and is not ready yet; the "+
				"update continues once it is.", prefix, h.pod.Name),
		}
	}
}

// templateImages is every image the replica's pod template runs, init containers included: a
// replica that cannot start one of them does not start at all.
func templateImages(set *appsv1.StatefulSet) []string {
	var images []string
	for _, containers := range [][]corev1.Container{
		set.Spec.Template.Spec.InitContainers, set.Spec.Template.Spec.Containers,
	} {
		for _, container := range containers {
			images = append(images, container.Image)
		}
	}
	return images
}
