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
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/monitor/node"
	"d8.io/upmeter/pkg/set"
)

// DaemonSetPodsReady is a checker constructor and configurator
type DaemonSetPodsReady struct {
	Access     kubernetes.Access
	NodeLister node.Lister

	// Namespace of the DaemonSet
	Namespace string
	// Name of the DaemonSet
	Name string

	// RequestTimeout is common for api operations
	RequestTimeout     time.Duration
	PodCreationTimeout time.Duration
	PodDeletionTimeout time.Duration

	// PreflightChecker verifies preconditions before running the check
	PreflightChecker check.Checker
}

func (c DaemonSetPodsReady) Checker() check.Checker {
	dsRepo := &daemonsetRepo{
		access:    c.Access,
		timeout:   c.RequestTimeout,
		name:      c.Name,
		namespace: c.Namespace,
	}

	dsChecker := &dsPodsReadinessChecker{
		dsRepo:          dsRepo,
		nodeLister:      c.NodeLister,
		creationTimeout: c.PodCreationTimeout,
		deletionTimeout: c.PodDeletionTimeout,
	}

	return sequence(
		c.PreflightChecker,
		withTimeout(dsChecker, c.RequestTimeout),
	)
}

// dsPodsReadinessChecker checks that all DaemonSet pods are ready
type dsPodsReadinessChecker struct {
	dsRepo          daemonSetRepository
	nodeLister      node.Lister
	creationTimeout time.Duration
	deletionTimeout time.Duration
}

func (c *dsPodsReadinessChecker) Check() check.Error {
	// Get nodes
	nodes, err := c.nodeLister.List()
	if err != nil {
		return check.ErrUnknown("getting nodes: %v", err)
	}

	// Get DaemonSet
	ds, err := c.dsRepo.Get()
	if err != nil {
		if apierrors.IsNotFound(err) {
			return check.ErrFail(err.Error())
		}
		return check.ErrUnknown("getting DaemonSet: %v", err)
	}

	// Get DaemonSet pods
	pods, err := c.dsRepo.Pods()
	if err != nil {
		return check.ErrUnknown("getting DaemonSet pods: %v", err)
	}

	// Filter node names of interest
	nodeNames := findDaemonSetNodeNames(nodes, ds)
	if err = c.verifyPods(pods, nodeNames); err != nil {
		return check.ErrFail(err.Error())
	}
	return nil
}

func (c *dsPodsReadinessChecker) verifyPods(pods []v1.Pod, nodeNameList []string) error {
	nodeNames := set.New(nodeNameList...)

	for _, pod := range pods {
		if !nodeNames.Has(pod.Spec.NodeName) {
			// pod is not from a node of interest
			continue
		}
		// The node is ok, so the pod should be ok too
		if err := c.verifyPodStatus(&pod); err != nil {
			return err
		}
		// Exclude seen nodes to track unseen ones.
		nodeNames.Delete(pod.Spec.NodeName)
	}

	// Check that there are no ready nodes without a pod
	if nodeNames.Size() > 0 {
		nodeNamesStr := strings.Join(nodeNames.Slice(), ", ")
		return fmt.Errorf("not all pods are running on desired nodes (%s)", nodeNamesStr)
	}
	return nil
}

// verifyPodStatus deduces the state when a pod is pending or running (but not ready), or
// terminating for reasonable time period. For that period pod is not considered down. It is useful
// for updates handling, while checking strictly for `Ready` condition is too strict.
//
// In arguments, this function accepts deadlines that divide time scale in two parts: pods are
// allowed to not be ready or to terminate before corresponding threshold, while afterwards the pod
// is considered down.
func (c *dsPodsReadinessChecker) verifyPodStatus(pod *v1.Pod) error {
	if isPodReady(pod) {
		return nil
	}

	if isPodTerminating(pod) {
		// The pod could be updating, giving it some time
		acceptableDeletionTime := metav1.NewTime(time.Now().Add(-c.deletionTimeout))
		if !pod.DeletionTimestamp.Before(&acceptableDeletionTime) {
			return nil
		}
		return fmt.Errorf("terminating for too long")
	}

	if isPodPending(pod) || isPodRunning(pod) {
		// Not ready, but started. Checking, how fresh it is.
		acceptableCreationTime := metav1.NewTime(time.Now().Add(-c.creationTimeout))
		if !pod.CreationTimestamp.Before(&acceptableCreationTime) {
			return nil
		}
		return fmt.Errorf("not ready for too long")
	}

	return fmt.Errorf("cannot deduce pod state")
}

func findDaemonSetNodeNames(nodes []*v1.Node, ds *appsv1.DaemonSet) []string {
	names := make([]string, 0)
	for _, node := range nodes {
		if !isNodeReadyLongEnough(node, 10*time.Minute) {
			// Filter by status, let the daemonset pod to be created on the node before checking
			continue
		}

		if node.Spec.Unschedulable {
			// If cordoned, something is happening, and we don't rely on this node
			continue
		}

		if !isTolerated(node.Spec.Taints, ds.Spec.Template.Spec.Tolerations) {
			// Filter by tolerations
			continue
		}

		names = append(names, node.Name)
	}
	return names
}

func isNodeReadyLongEnough(node *v1.Node, threshold time.Duration) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type != v1.NodeReady {
			// not the condition type we are looking for
			continue
		}

		if cond.Status != v1.ConditionTrue {
			// not ready? not ok
			return false
		}

		startInThePast := metav1.NewTime(time.Now().Add(-threshold))
		isReadyLongEnough := cond.LastTransitionTime.Before(&startInThePast)
		return isReadyLongEnough
	}

	return false
}

// isTolerated checks if the given tolerations tolerates all taints
//
//	Copied from https://github.com/kubernetes/component-helpers/blob/v0.21.0/scheduling/corev1/helpers.go
//	It is not imported since k8s dependencies versions would require to rise to at least 0.20.
func isTolerated(taints []v1.Taint, tolerations []v1.Toleration) bool {
	for _, taint := range taints {
		if !tolerationsTolerateTaint(tolerations, &taint) {
			return false
		}
	}
	return true
}

// tolerationsTolerateTaint checks if taint is tolerated by any of the tolerations.
//
//	Copied from https://github.com/kubernetes/component-helpers/blob/v0.21.0/scheduling/corev1/helpers.go
//	It is not imported since k8s dependencies versions would require to rise to at least 0.20.
func tolerationsTolerateTaint(tolerations []v1.Toleration, taint *v1.Taint) bool {
	for i := range tolerations {
		if tolerations[i].ToleratesTaint(taint) {
			return true
		}
	}
	return false
}

type daemonSetRepository interface {
	Get() (*appsv1.DaemonSet, error)
	Pods() ([]v1.Pod, error)
}

type daemonsetRepo struct {
	access  kubernetes.Access
	timeout time.Duration

	name      string
	namespace string
}

func (r *daemonsetRepo) Get() (*appsv1.DaemonSet, error) {
	return r.access.Kubernetes().AppsV1().DaemonSets(r.namespace).Get(context.TODO(), r.name, metav1.GetOptions{})
}

func (r *daemonsetRepo) Pods() ([]v1.Pod, error) {
	timeout := int64(r.timeout.Seconds())

	podList, err := r.access.Kubernetes().CoreV1().Pods(r.namespace).List(
		context.TODO(),
		metav1.ListOptions{
			LabelSelector:  "app=" + r.name,
			TimeoutSeconds: &timeout,
		})
	if err != nil {
		return nil, err
	}
	return podList.Items, nil
}
