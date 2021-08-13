/*
Copyright 2021 Flant CJSC

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
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/util"
)

// PodLifecycle is a checker constructor and configurator
type PodLifecycle struct {
	Access            kubernetes.Access
	Namespace         string
	CreationTimeout   time.Duration
	SchedulingTimeout time.Duration
	DeletionTimeout   time.Duration
	Node              string

	GarbageCollectionTimeout  time.Duration
	ControlPlaneAccessTimeout time.Duration
}

func (c PodLifecycle) Checker() check.Checker {
	return &podLifecycleChecker{
		access:            c.Access,
		namespace:         c.Namespace,
		creationTimeout:   c.CreationTimeout,
		schedulingTimeout: c.SchedulingTimeout,
		deletionTimeout:   c.DeletionTimeout,
		node:              c.Node,

		garbageCollectionTimeout:  c.GarbageCollectionTimeout,
		controlPlaneAccessTimeout: c.ControlPlaneAccessTimeout,
	}
}

// podLifecycleChecker is stateful checker that wraps pod creation with the check of the pod lifecycle
type podLifecycleChecker struct {
	access            kubernetes.Access
	namespace         string
	creationTimeout   time.Duration
	schedulingTimeout time.Duration
	deletionTimeout   time.Duration
	node              string

	garbageCollectionTimeout  time.Duration
	controlPlaneAccessTimeout time.Duration

	// inner state
	checker check.Checker
}

func (c *podLifecycleChecker) BusyWith() string {
	return c.checker.BusyWith()
}

func (c *podLifecycleChecker) Check() check.Error {
	pod := createPodObject(c.node, c.access.CpSchedulerImage())
	c.checker = c.new(pod)
	return c.checker.Check()
}

/*
1. check control-plane
2. collect garbage
2. create pod                   (podCreationTimeout)
3. see the pod is scheduled     (podScheduledTimeout)
4. delete the pod               (podDeletionTimeout)
	+ensure the pod is not listed
*/
func (c *podLifecycleChecker) new(pod *v1.Pod) check.Checker {
	pingControlPlane := newControlPlaneChecker(c.access, c.controlPlaneAccessTimeout)
	collectGarbage := newGarbageCollectorCheckerByLabels(c.access, pod.Kind, c.namespace, pod.Labels, c.garbageCollectionTimeout)

	listOpts := listOptsByLabels(pod.GetLabels())

	createPod := withTimeout(
		&podCreationChecker{access: c.access, namespace: c.namespace, pod: pod},
		c.creationTimeout)

	waitForScheduling := withRetryEachSeconds(
		&podScheduledChecker{access: c.access, namespace: c.namespace, listOpts: listOpts},
		c.schedulingTimeout)

	deletePod := withTimeout(
		&podDeletionChecker{access: c.access, namespace: c.namespace, listOpts: listOpts},
		c.deletionTimeout)

	verifyNoPod := withRetryEachSeconds(
		&objectIsNotListedChecker{access: c.access, namespace: c.namespace, kind: pod.Kind, listOpts: listOpts},
		c.garbageCollectionTimeout)

	return sequence(
		pingControlPlane,
		collectGarbage,
		createPod,
		waitForScheduling,
		deletePod,
		verifyNoPod,
	)
}

type podCreationChecker struct {
	access    kubernetes.Access
	namespace string
	pod       *v1.Pod
}

func (c *podCreationChecker) BusyWith() string {
	return fmt.Sprintf("creating pod %s/%s", c.namespace, c.pod.Name)
}

func (c *podCreationChecker) Check() check.Error {
	_, err := c.access.Kubernetes().CoreV1().Pods(c.namespace).Create(c.pod)
	if err != nil {
		return check.ErrFail("cannot create pod %s/%s", c.namespace, c.pod.Name)
	}
	return nil
}

type podScheduledChecker struct {
	access    kubernetes.Access
	namespace string
	listOpts  *metav1.ListOptions
}

func (c *podScheduledChecker) BusyWith() string {
	return fmt.Sprintf("looking for scheduled pod ns=%s listOpts=%s", c.namespace, c.listOpts)
}

func (c *podScheduledChecker) Check() check.Error {
	client := c.access.Kubernetes()

	podList, err := client.CoreV1().Pods(c.namespace).List(*c.listOpts)
	if err != nil {
		return check.ErrFail("cannot get pod list %s/%s: %v", c.namespace, c.listOpts, err)
	}

	if len(podList.Items) == 0 {
		return check.ErrFail("pod not found %s/%s", c.namespace, c.listOpts)
	}

	for _, pod := range podList.Items {
		isScheduled := pod.Spec.NodeName != ""
		if isScheduled {
			return nil
		}
	}
	return check.ErrFail("pod not scheduled %s/%s", c.namespace, c.listOpts)
}

// Checks that at least one Pod is in "Pending" state.
// FIXME by the task, should check it is not PodUnknown ???
type pendingPodChecker struct {
	access    kubernetes.Access
	namespace string
	listOpts  *metav1.ListOptions
}

func (c *pendingPodChecker) BusyWith() string {
	return fmt.Sprintf("looking for pending pod ns=%s listOpts=%s", c.namespace, c.listOpts)
}

func (c *pendingPodChecker) Check() check.Error {
	client := c.access.Kubernetes()

	podList, err := client.CoreV1().Pods(c.namespace).List(*c.listOpts)
	if err != nil {
		return check.ErrFail("cannot get pod list %s/%s: %v", c.namespace, c.listOpts, err)
	}

	for _, pod := range podList.Items {
		if pod.Status.Phase == v1.PodPending {
			return nil
		}
	}

	return check.ErrFail("did not find pod %s/%s", c.namespace, c.listOpts)
}

type podDeletionChecker struct {
	access    kubernetes.Access
	namespace string
	listOpts  *metav1.ListOptions
}

func (c *podDeletionChecker) BusyWith() string {
	return fmt.Sprintf("deleting pod ns=%s listOpts=%s", c.namespace, c.listOpts)
}

func (c *podDeletionChecker) Check() check.Error {
	client := c.access.Kubernetes()
	// We delete a collection, not only one pod, to
	//  1. reuse generic listOptions
	//  2. eventually collect garbage left by previous runs
	err := client.CoreV1().Pods(c.namespace).DeleteCollection(&metav1.DeleteOptions{}, *c.listOpts)
	if err != nil {
		return check.ErrFail("cannot delete pod %s/%s: %v", c.namespace, c.listOpts, err)
	}
	return nil
}

func createPodObject(nodeName string, image *kubernetes.ProbeImage) *v1.Pod {
	nodeAffinity := createNodeAffinityObject(nodeName)

	podName := util.RandomIdentifier("upmeter-control-plane-scheduler")

	return &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"heritage":      "upmeter",
				"upmeter-agent": util.AgentUniqueId(),
				"upmeter-group": "control-plane",
				"upmeter-probe": "scheduler",
			},
		},
		Spec: v1.PodSpec{
			ImagePullSecrets: image.GetPullSecrets(),
			Containers: []v1.Container{
				{
					Name:            "pause",
					Image:           image.GetImageName(),
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
							Operator: "In",
							Values:   []string{nodeName},
						},
					},
				},
			},
		},
	}
}

// AtLeastOnePodReady is a checker constructor and configurator
type AtLeastOnePodReady struct {
	Access        kubernetes.Access
	Namespace     string
	LabelSelector string

	Timeout                   time.Duration
	ControlPlaneAccessTimeout time.Duration
}

func (c AtLeastOnePodReady) Checker() check.Checker {
	podsChecker := &podReadinessChecker{
		access:        c.Access,
		namespace:     c.Namespace,
		labelSelector: c.LabelSelector,
	}

	return sequence(
		newControlPlaneChecker(c.Access, c.ControlPlaneAccessTimeout),
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
	podList, err := c.access.Kubernetes().CoreV1().Pods(c.namespace).List(metav1.ListOptions{LabelSelector: c.labelSelector})
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

func (c *podReadinessChecker) BusyWith() string {
	return fmt.Sprintf("getting pods %s,%s", c.namespace, c.labelSelector)
}

func isPodReady(pod *v1.Pod) bool {
	if pod.Status.Phase != v1.PodRunning {
		return false
	}

	for _, cond := range pod.Status.Conditions {
		if cond.Type != v1.PodReady {
			// not the condition type we are looking for
			continue
		}
		return cond.Status == v1.ConditionTrue
	}

	return false
}
