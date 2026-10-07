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

package layout

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
)

const (
	// referenceScanInterval is how often a held air-gap asks again, and how long a count that
	// holds it is reused meanwhile. A count that would let the upstream go is never reused while
	// the air-gap is being decided. See upstreamReferenceCounter.
	referenceScanInterval = 30 * time.Second

	// referenceReportInterval is how long a count is reused while no air-gap is asked for, when it
	// only feeds status.safeToDropUpstream. The reconciliation runs on every change to any
	// RegistryNode or to the storage status, and a scan on each of those would be a steady load on
	// the API server for a field that changes when a rollout finishes.
	referenceReportInterval = 5 * time.Minute

	// referenceScanTimeout bounds one whole scan. It runs inside a reconciliation, and a scan
	// stuck on a slow API server would hold every other decision of that reconciliation with it.
	// A scan that times out fails, and a failed count holds the upstream: the safe answer.
	referenceScanTimeout = 30 * time.Second

	// referencePageSize bounds one list request of the scan, so that a large namespace is read in
	// pages rather than held in memory whole by a controller with a small memory limit.
	referencePageSize = 500

	// referenceExamples is how many of the references are logged. The count is the decision; the
	// names are there to say which rollout to look at, and five of them do that.
	referenceExamples = 5

	// storeAppLabel marks the store's own pods, which are counted more strictly than the rest.
	// See isStorePod.
	storeAppLabel = "registry-storage"
)

// upstreamReferences is the count the air-gap gate needs, or nil where it is not taken.
//
// Taken only while the leader is full and an upstream is still held, the one state in which it
// can change an answer: with the leader incomplete the gate is shut regardless, and with no
// upstream left there is nothing to name. A count that fails is logged and reads as nil, which
// holds the upstream — the layout itself is still applied, since a failed list must not stop
// the nodes from converging.
func (r *Reconciler) upstreamReferences(
	ctx context.Context, cfg *registryv1alpha1.RegistryConfig, held *registryv1alpha1.Upstream, leaderFull bool,
) *int32 {
	if !leaderFull || held == nil || r.Reader == nil {
		return nil
	}
	log := ctrl.LoggerFrom(ctx)

	now := time.Now
	if r.now != nil {
		now = r.now
	}
	deciding := cfg.Spec.Primary.Upstream == nil
	count, examples, err := r.references.take(ctx, r.Reader, held, deciding, now())
	if err != nil {
		log.Error(err, "cannot count what still names the upstream; it is held until the count succeeds")
		return nil
	}
	// Said when the count moves, not on every reconciliation: those come with every change to any
	// RegistryNode or to the storage status, and the same line each time buried the one that
	// changed.
	if deciding && r.references.changed(count) && count > 0 {
		log.Info("the storage leader holds the expected set, but the platform still names the upstream; holding it",
			"references", count, "examples", strings.Join(examples, "; "))
	}
	return &count
}

// upstreamReferenceCounter counts the platform workloads that still name an upstream, and keeps
// the last count for the reconciliations that follow it closely.
type upstreamReferenceCounter struct {
	mu       sync.Mutex
	address  string
	last     int32
	examples []string
	takenAt  time.Time

	// reported is the count last logged, and hasReported whether there is one.
	reported    int32
	hasReported bool
}

// changed reports whether count differs from the one last reported, and records it as reported.
func (c *upstreamReferenceCounter) changed(count int32) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.hasReported && c.reported == count {
		return false
	}
	c.reported, c.hasReported = count, true
	return true
}

// take returns how many workloads name `upstream`, reusing a recent count where that cannot
// authorize anything: while an air-gap is being decided, a nonzero count for referenceScanInterval
// and a zero one never; otherwise any count for referenceReportInterval, since then it is only
// reported.
func (c *upstreamReferenceCounter) take(
	ctx context.Context, reader client.Reader, upstream *registryv1alpha1.Upstream, deciding bool, now time.Time,
) (int32, []string, error) {
	address := upstreamAddresses(upstream)

	c.mu.Lock()
	defer c.mu.Unlock()

	age := now.Sub(c.takenAt)
	if c.address == address {
		switch {
		case deciding && c.last > 0 && age < referenceScanInterval,
			!deciding && age < referenceReportInterval:
			return c.last, c.examples, nil
		}
	}

	scanCtx, cancel := context.WithTimeout(ctx, referenceScanTimeout)
	defer cancel()
	count, examples, err := countUpstreamReferences(scanCtx, reader, upstream)
	if err != nil {
		c.address = ""
		return 0, nil, err
	}

	c.address, c.last, c.examples, c.takenAt = address, count, examples, now
	return count, examples, nil
}

// upstreamAddresses is the cache key for a count: every address the upstream can be named by.
func upstreamAddresses(upstream *registryv1alpha1.Upstream) string {
	endpoints := upstream.Endpoints()
	addresses := make([]string, 0, len(endpoints))
	for i := range endpoints {
		addresses = append(addresses, endpoints[i].Address())
	}
	return strings.Join(addresses, ",")
}

// countUpstreamReferences counts what on the platform would stop pulling if `upstream` went away.
//
// The workloads are counted, not their images: one Deployment that names the upstream is one
// rollout to wait for, however many containers it has.
//
// Templates count unconditionally, because every pod they make pulls. Live pods count only
// where they would still pull — see podWouldPull — so that a pod which merely ran from the
// upstream once, and will not ask it again, does not hold the cluster back. The store's own
// replicas are the exception and count while they name the upstream at all: see isStorePod.
//
// Platform namespaces only, listed first and then read one at a time in pages, as the drain counts
// them: those are what Deckhouse renders, and so
// what moves onto the in-cluster address on its own. A user's workload pulling from the same
// registry is the user's to move, and waiting on it would hold the air-gap for as long as the
// user never does.
func countUpstreamReferences(
	ctx context.Context, reader client.Reader, upstream *registryv1alpha1.Upstream,
) (int32, []string, error) {
	names := namesUpstream(upstream)
	var found []string
	note := func(kind, namespace, name string) {
		found = append(found, fmt.Sprintf("%s %s/%s", kind, namespace, name))
	}

	namespaces, err := platformNamespaces(ctx, reader)
	if err != nil {
		return 0, nil, err
	}

	for _, namespace := range namespaces {
		err := eachPage(ctx, reader, namespace, &corev1.PodList{}, func(list *corev1.PodList) {
			for i := range list.Items {
				pod := &list.Items[i]
				if isStorePod(pod) && podNames(pod, names) || podWouldPull(pod, names) {
					note("pod", pod.Namespace, pod.Name)
				}
			}
		})
		if err != nil {
			return 0, nil, fmt.Errorf("listing pods in %s: %w", namespace, err)
		}

		err = eachPage(ctx, reader, namespace, &appsv1.DeploymentList{}, func(list *appsv1.DeploymentList) {
			for i := range list.Items {
				if specNames(&list.Items[i].Spec.Template.Spec, names) {
					note("deployment", list.Items[i].Namespace, list.Items[i].Name)
				}
			}
		})
		if err != nil {
			return 0, nil, fmt.Errorf("listing deployments in %s: %w", namespace, err)
		}

		err = eachPage(ctx, reader, namespace, &appsv1.DaemonSetList{}, func(list *appsv1.DaemonSetList) {
			for i := range list.Items {
				if specNames(&list.Items[i].Spec.Template.Spec, names) {
					note("daemonset", list.Items[i].Namespace, list.Items[i].Name)
				}
			}
		})
		if err != nil {
			return 0, nil, fmt.Errorf("listing daemonsets in %s: %w", namespace, err)
		}

		err = eachPage(ctx, reader, namespace, &appsv1.StatefulSetList{}, func(list *appsv1.StatefulSetList) {
			for i := range list.Items {
				if specNames(&list.Items[i].Spec.Template.Spec, names) {
					note("statefulset", list.Items[i].Namespace, list.Items[i].Name)
				}
			}
		})
		if err != nil {
			return 0, nil, fmt.Errorf("listing statefulsets in %s: %w", namespace, err)
		}

		err = eachPage(ctx, reader, namespace, &batchv1.CronJobList{}, func(list *batchv1.CronJobList) {
			for i := range list.Items {
				if specNames(&list.Items[i].Spec.JobTemplate.Spec.Template.Spec, names) {
					note("cronjob", list.Items[i].Namespace, list.Items[i].Name)
				}
			}
		})
		if err != nil {
			return 0, nil, fmt.Errorf("listing cronjobs in %s: %w", namespace, err)
		}
	}

	examples := found
	if len(examples) > referenceExamples {
		examples = examples[:referenceExamples]
	}
	return int32(len(found)), examples, nil
}

// platformNamespaces are the namespaces the scan reads: the ones Deckhouse renders into.
func platformNamespaces(ctx context.Context, reader client.Reader) ([]string, error) {
	var out []string
	err := eachPage(ctx, reader, "", &corev1.NamespaceList{}, func(list *corev1.NamespaceList) {
		for i := range list.Items {
			if platformNamespace(list.Items[i].Name) {
				out = append(out, list.Items[i].Name)
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("listing namespaces: %w", err)
	}
	return out, nil
}

// eachPage reads a list in pages of referencePageSize and hands each page to visit.
func eachPage[L client.ObjectList](
	ctx context.Context, reader client.Reader, namespace string, list L, visit func(L),
) error {
	options := []client.ListOption{client.Limit(referencePageSize)}
	if namespace != "" {
		options = append(options, client.InNamespace(namespace))
	}
	for {
		if err := reader.List(ctx, list, options...); err != nil {
			return err
		}
		visit(list)
		next := list.GetContinue()
		if next == "" {
			return nil
		}
		options = append(options[:0:0], client.Limit(referencePageSize), client.Continue(next))
		if namespace != "" {
			options = append(options, client.InNamespace(namespace))
		}
	}
}

// namesUpstream returns the test for an image reference naming `upstream` by any of its addresses.
//
// A repository under the address, and the address itself as a repository: the platform's own
// image is `<host>/<path>@sha256:…`, with nothing between the path and the digest.
func namesUpstream(upstream *registryv1alpha1.Upstream) func(image string) bool {
	endpoints := upstream.Endpoints()
	addresses := make([]string, 0, len(endpoints))
	for i := range endpoints {
		if address := endpoints[i].Address(); address != "" {
			addresses = append(addresses, canonicalReference(address))
		}
	}

	return func(image string) bool {
		image = canonicalReference(image)
		for _, address := range addresses {
			rest, ok := strings.CutPrefix(image, address)
			if !ok {
				continue
			}
			switch {
			case rest == "", strings.HasPrefix(rest, "/"), strings.HasPrefix(rest, "@"):
				return true
			case strings.HasPrefix(rest, ":"):
				// A tag on the address itself, which needs the address to be a repository — a path
				// under the host. With a bare host, what follows a colon is a port, and
				// `host:5000/...` is another registry than `host`.
				if strings.Contains(address, "/") {
					return true
				}
			}
		}
		return false
	}
}

// canonicalReference spells the registry host of a reference the one way, so that the same
// registry written two ways is recognised: hostnames are case-insensitive, and 443 is the port
// an HTTPS registry is reached on when none is written.
func canonicalReference(reference string) string {
	host, rest, found := strings.Cut(reference, "/")
	host = strings.TrimSuffix(strings.ToLower(host), ":443")
	if !found {
		return host
	}
	return host + "/" + rest
}

// podWouldPull reports whether a live pod would still fetch an image that names the upstream.
//
// Three ways it would, and they are the whole of the rule:
//
//   - the container has not started yet, so its image is still to be pulled;
//   - its pull policy is `Always`, so every start is a pull;
//   - the pod has pull secrets. The kubelet records which credentials an image was pulled with
//     and, on a later start, verifies that the pod may still use them by pulling again. Air-gap is
//     exactly when those credentials stop working, so a container that has run happily for an hour
//     dies on its next restart, with its image on the disk. This is the case that was measured.
//
// A pod on its way out, or one that has finished, will not start anything again.
func podWouldPull(pod *corev1.Pod, names func(string) bool) bool {
	if pod.DeletionTimestamp != nil ||
		pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false
	}

	// Started at some point, which is what puts the image on the node. A container crash-looping
	// between restarts is Waiting, and read off its current state alone it looked unstarted and
	// held the gate for as long as it crashed.
	started := make(map[string]bool, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
	for _, statuses := range [][]corev1.ContainerStatus{
		pod.Status.ContainerStatuses,
		pod.Status.InitContainerStatuses,
	} {
		for i := range statuses {
			status := &statuses[i]
			started[status.Name] = status.State.Running != nil || status.State.Terminated != nil ||
				status.RestartCount > 0 || status.LastTerminationState.Terminated != nil
		}
	}
	verified := len(pod.Spec.ImagePullSecrets) > 0

	pulls := func(container *corev1.Container) bool {
		if !names(container.Image) {
			return false
		}
		return verified || container.ImagePullPolicy == corev1.PullAlways || !started[container.Name]
	}

	for i := range pod.Spec.Containers {
		if pulls(&pod.Spec.Containers[i]) {
			return true
		}
	}
	for i := range pod.Spec.InitContainers {
		if pulls(&pod.Spec.InitContainers[i]) {
			return true
		}
	}
	// Ephemeral containers are left out: they are someone debugging a pod, not the platform's
	// workload, and one attached with an upstream image would otherwise hold the air-gap.
	return false
}

// specNames answers whether anything in a pod template names the upstream, init containers
// included: the platform's own Deployment names its registry in one of those.
func specNames(spec *corev1.PodSpec, names func(string) bool) bool {
	for i := range spec.Containers {
		if names(spec.Containers[i].Image) {
			return true
		}
	}
	for i := range spec.InitContainers {
		if names(spec.InitContainers[i].Image) {
			return true
		}
	}
	return false
}

// platformNamespace answers whether a namespace is one Deckhouse renders into.
func platformNamespace(namespace string) bool {
	return namespace == "kube-system" || strings.HasPrefix(namespace, "d8-")
}

// isStorePod marks a replica of the store, which counts while it names the upstream at all —
// started or not, pull secrets or not.
//
// The store cannot serve its own image. A replica replaced after the upstream is gone pulls itself
// through the store it belongs to, and the one replaced first finds nothing that answers but the
// replicas it is replacing; the storage update controller replaces replicas whenever their revision
// is stale, which a store created before the in-cluster address was published always is. So the
// replicas have to be on the in-cluster address before the upstream goes, while the agent can still
// fall back to it for whatever the store cannot answer. Measured on the `scenarios` unit on
// 2026-10-05, where the upstream went before the replicas had moved: every replica in
// ImagePullBackOff on its own image, every agent answering 502.
func isStorePod(pod *corev1.Pod) bool {
	return pod.Namespace == Namespace && pod.Labels["app"] == storeAppLabel
}

// podNames answers whether any container of a pod names the upstream.
func podNames(pod *corev1.Pod, names func(string) bool) bool {
	return pod.DeletionTimestamp == nil && specNames(&pod.Spec, names)
}
