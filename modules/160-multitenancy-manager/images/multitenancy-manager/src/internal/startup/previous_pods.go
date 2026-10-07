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

package startup

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReasonPreviousRevisionStillRunning is the reason of the Warning event on the controller pod that
// starts its controllers while pods of another revision of its Deployment have not finished.
const ReasonPreviousRevisionStillRunning = "PreviousRevisionStillRunning"

// deploymentRevisionAnnotation is the revision number the Deployment controller writes on each of its
// ReplicaSets; a newer pod template gets a higher one.
const deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"

// PreviousPods waits for the pods of the other revisions of the controller Deployment to finish.
//
// A revision is a pod template: the ReplicaSet controller stamps its hash on every pod as the
// pod-template-hash label. During a rolling update a pod of the old revision keeps running next to
// the new one until the new one is ready. If the old revision does not take the leader election
// lease, its controllers keep writing the objects the new ones write, and what stays depends on
// which pod wrote last.
//
// A pod of an older revision counts until it finishes: while it is pending, running or terminating
// (deleted, with its containers still stopping). A pod of a newer revision does not count: every
// revision with this wait runs with leader election, so a newer pod cannot write without the lease
// this pod holds. When the revision numbers of the ReplicaSets cannot be read, a pod of any other
// revision counts. A pod in phase Succeeded or Failed, an evicted one for example, runs nothing and
// does not count, and neither does a pod deleted longer ago than Timeout: the wait for it has run
// out once already, on a lost node it may stay terminating until the Node is deleted, and each start
// of the controller would hold for the whole Timeout again. A pod without the pod-template-hash label
// belongs to no revision and does not count either.
type PreviousPods struct {
	// Reader lists the pods. The uncached API reader: a cached one would start a pod informer for a
	// few checks at startup.
	Reader client.Reader
	// Recorder receives the Warning event on this pod when the wait gives up.
	Recorder record.EventRecorder
	Logger   logr.Logger
	// Namespace and Name locate this pod; Selector matches the pods of the Deployment.
	Namespace string
	Name      string
	Selector  map[string]string
	// Interval is the pause between two checks; Timeout bounds the whole wait.
	Interval time.Duration
	Timeout  time.Duration
}

// Wait blocks until no pod of another revision is left and returns nil. After Timeout it gives up and
// returns nil as well, with an error-level log line and a Warning event on this pod, so a pod stuck
// terminating on a lost node does not keep the controllers down for good. A failed check is retried
// at the next one. Wait returns the error of ctx when ctx is done first.
//
// Without a pod name, or when this pod carries no revision, there is nothing to compare with and Wait
// returns at once.
func (p *PreviousPods) Wait(ctx context.Context) error {
	if p.Name == "" {
		p.Logger.Info("the pod name is not set, not waiting for the pods of other revisions")
		return nil
	}

	timeout := time.NewTimer(p.Timeout)
	defer timeout.Stop()
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()

	start := time.Now()
	isHolding := false
	// What the last successful check saw, for the warning when the wait gives up.
	var self *corev1.Pod
	var others []string
	var lastErr error
	for {
		checkedSelf, checkedOthers, err := p.check(ctx)
		lastErr = err
		switch {
		case err != nil:
			p.Logger.Info("cannot check the pods of other revisions, retrying", "error", err)
		case checkedSelf.Labels[appsv1.DefaultDeploymentUniqueLabelKey] == "":
			p.Logger.Info("this pod belongs to no revision, not waiting for the pods of other revisions")
			return nil
		case len(checkedOthers) == 0:
			// Every way out of the wait leaves a line, so the log tells whether the controllers started.
			if isHolding {
				p.Logger.Info("the pods of other revisions finished", "waited", time.Since(start).String())
			} else {
				p.Logger.Info("no pods of other revisions to wait for")
			}
			return nil
		default:
			if !isHolding {
				p.Logger.Info("hold the controllers until the pods of other revisions finish",
					"pods", checkedOthers, "timeout", p.Timeout.String())
				isHolding = true
			}
			self, others = checkedSelf, checkedOthers
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			p.giveUp(self, others, lastErr)
			return nil
		case <-ticker.C:
		}
	}
}

// check lists the pods of the Deployment and returns this pod and the names of the pods of other
// revisions that have not finished, sorted.
func (p *PreviousPods) check(ctx context.Context) (*corev1.Pod, []string, error) {
	pods := new(corev1.PodList)
	if err := p.Reader.List(ctx, pods, client.InNamespace(p.Namespace), client.MatchingLabels(p.Selector)); err != nil {
		return nil, nil, fmt.Errorf("list the pods of the deployment: %w", err)
	}

	selfIndex := slices.IndexFunc(pods.Items, func(pod corev1.Pod) bool { return pod.Name == p.Name })
	if selfIndex < 0 {
		return nil, nil, fmt.Errorf("pod %q is not among the pods of the deployment", p.Name)
	}
	self := &pods.Items[selfIndex]

	revisions, err := p.revisions(ctx)
	if err != nil {
		return nil, nil, err
	}

	hash := self.Labels[appsv1.DefaultDeploymentUniqueLabelKey]
	var others []string
	for i := range pods.Items {
		if pod := &pods.Items[i]; p.waitsFor(pod, hash, revisions, time.Now()) {
			others = append(others, pod.Name)
		}
	}
	slices.Sort(others)

	return self, others, nil
}

// revisions maps the pod-template-hash of every ReplicaSet of the Deployment to its revision number.
// A ReplicaSet without a readable number is left out.
func (p *PreviousPods) revisions(ctx context.Context) (map[string]int64, error) {
	sets := new(appsv1.ReplicaSetList)
	if err := p.Reader.List(ctx, sets, client.InNamespace(p.Namespace), client.MatchingLabels(p.Selector)); err != nil {
		return nil, fmt.Errorf("list the replica sets of the deployment: %w", err)
	}
	revisions := make(map[string]int64, len(sets.Items))
	for i := range sets.Items {
		hash := sets.Items[i].Labels[appsv1.DefaultDeploymentUniqueLabelKey]
		revision, err := strconv.ParseInt(sets.Items[i].Annotations[deploymentRevisionAnnotation], 10, 64)
		if hash == "" || err != nil {
			continue
		}
		revisions[hash] = revision
	}
	return revisions, nil
}

// waitsFor reports whether this pod, of revision hash, waits for pod: pod belongs to another revision
// that is not known to be newer, may still run its containers, and was not deleted longer ago than
// Timeout.
func (p *PreviousPods) waitsFor(pod *corev1.Pod, hash string, revisions map[string]int64, now time.Time) bool {
	podHash := pod.Labels[appsv1.DefaultDeploymentUniqueLabelKey]
	if podHash == "" || podHash == hash {
		return false
	}
	if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		return false
	}
	if pod.DeletionTimestamp != nil && now.Sub(pod.DeletionTimestamp.Time) > p.Timeout {
		return false
	}
	own, ownKnown := revisions[hash]
	other, otherKnown := revisions[podHash]
	return !ownKnown || !otherKnown || other < own
}

// giveUp reports that the controllers start while the pods of other revisions have not finished. The
// event needs this pod, which is unknown when no check has succeeded; the log line is written anyway.
// logr has no warning level, so the line goes out at error level.
func (p *PreviousPods) giveUp(self *corev1.Pod, others []string, lastErr error) {
	p.Logger.Error(lastErr, "the pods of other revisions did not finish in time, starting the controllers anyway",
		"pods", others, "timeout", p.Timeout.String())
	if self == nil {
		return
	}
	p.Recorder.Eventf(self, corev1.EventTypeWarning, ReasonPreviousRevisionStillRunning,
		"Pods of another revision of the Deployment did not finish in %s: %s. The controllers start anyway; "+
			"until those pods finish, both revisions may write the same objects.",
		p.Timeout, strings.Join(others, ", "))
}
