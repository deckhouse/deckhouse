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

package checker

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// AtLeastOnePodReady is a checker constructor and configurator
type AtLeastOnePodReady struct {
	Access        kubernetes.Access
	Namespace     string
	LabelSelector string

	Timeout time.Duration

	// PreflightChecker verifies preconditions before running the check
	PreflightChecker check.Checker
}

func (c AtLeastOnePodReady) Checker() check.Checker {
	podsChecker := &podReadinessChecker{
		access:        c.Access,
		namespace:     c.Namespace,
		labelSelector: c.LabelSelector,
	}

	return sequence(
		c.PreflightChecker,
		withTimeout(podsChecker, c.Timeout),
	)
}

// podReadinessChecker defines the information that lets check at least one ready pod
type podReadinessChecker struct {
	namespace     string
	labelSelector string
	access        kubernetes.Access
}

func (c *podReadinessChecker) Check() check.Error {
	podList, err := c.access.Kubernetes().CoreV1().Pods(c.namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: c.labelSelector})
	if err != nil {
		return check.ErrUnknown("cannot get pods %s,%s: %v", c.namespace, c.labelSelector, err)
	}

	for _, pod := range podList.Items {
		if isPodReady(&pod) {
			return nil
		}
	}

	return check.ErrFail("no ready pods found %s,%s", c.namespace, c.labelSelector)
}

// podRunningOrReadyChecker checks that there is a pod in Ready condition in reasonable time.
//   - if pod is running, but not ready in the `readinessTimeout` time, the check status is considered unknown.
//   - if pod is running, but not ready in `readinessTimeout` or more, the check status is considered failed.
//   - if pod is terminating, the status is unknown
//   - otherwise, the status is success.
type podRunningOrReadyChecker struct {
	namespace        string
	labelSelector    string
	readinessTimeout time.Duration
	access           kubernetes.Access
}

func (c *podRunningOrReadyChecker) Check() check.Error {
	podList, err := c.access.Kubernetes().CoreV1().Pods(c.namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: c.labelSelector})
	if err != nil {
		return check.ErrUnknown("cannot get pods %s,%s: %v", c.namespace, c.labelSelector, err)
	}

	var cherr check.Error
	for _, pod := range podList.Items {
		err := isPodWorking(&pod, c.readinessTimeout)
		if err == nil {
			// the pod is fine
			return nil
		}
		if cherr == nil {
			// got error, saving it for comparison with other pods
			cherr = err
			continue
		}
		if cherr.Status() == check.Unknown {
			// got at least second error, we should not make the status worse anyway
			continue
		}
		cherr = err
	}
	return cherr
}

func isPodRunning(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodRunning
}

func isPodPending(pod *v1.Pod) bool {
	return pod.Status.Phase == v1.PodPending
}

func isPodTerminating(pod *v1.Pod) bool {
	return pod.DeletionTimestamp != nil
}

func isPodReady(pod *v1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type != v1.PodReady {
			// not the condition type we are looking for
			continue
		}
		return cond.Status == v1.ConditionTrue
	}

	return false
}

func isPodWorking(pod *v1.Pod, readinessTimeout time.Duration) check.Error {
	if pod.DeletionTimestamp != nil {
		return check.ErrUnknown("pod is terminating")
	}

	if isPodReady(pod) {
		return nil
	}

	if !isPodRunning(pod) {
		return check.ErrFail("pod is not running")
	}

	// Let's see how long it is running
	readinessThreshold := pod.CreationTimestamp.Add(readinessTimeout)
	if time.Now().After(readinessThreshold) {
		return check.ErrFail("pod is not ready for too long (%s)", readinessTimeout.String())
	}

	return check.ErrUnknown("pod is running, but not ready")
}

func createPodObject(podName, nodeName, agentID string, image *kubernetes.ProbeImage) *v1.Pod {
	nodeAffinity := createNodeAffinityObject(nodeName)

	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"heritage":      "upmeter",
				agentLabelKey:   agentID,
				"upmeter-group": "control-plane",
				"upmeter-probe": "scheduler",
			},
		},
		Spec: v1.PodSpec{
			ImagePullSecrets: image.PullSecrets(),
			Containers: []v1.Container{
				{
					Name:            "pause",
					Image:           image.Name(),
					ImagePullPolicy: v1.PullIfNotPresent,
					Command: []string{
						"true",
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Tolerations: []v1.Toleration{
				{Operator: v1.TolerationOpExists},
			},
			Affinity: &v1.Affinity{
				NodeAffinity: nodeAffinity,
			},
		},
	}
}

func createNodeAffinityObject(nodeName string) *v1.NodeAffinity {
	return &v1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
			NodeSelectorTerms: []v1.NodeSelectorTerm{
				{
					MatchExpressions: []v1.NodeSelectorRequirement{
						{
							Key:      "kubernetes.io/hostname",
							Operator: v1.NodeSelectorOpIn,
							Values:   []string{nodeName},
						},
					},
				},
			},
		},
	}
}
