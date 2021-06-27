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
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeclient "k8s.io/client-go/kubernetes"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// DaemonSetPodsReady is a checker constructor and configurator
type DaemonSetPodsReady struct {
	Access    kubernetes.Access
	Timeout   time.Duration
	Namespace string
	Name      string
}

func (c DaemonSetPodsReady) Checker() check.Checker {
	dsChecker := &dsPodsReadinessChecker{
		access:        c.Access,
		namespace:     c.Namespace,
		daemonSetName: c.Name,
	}

	checker := sequence(
		&controlPlaneChecker{c.Access},
		dsChecker,
	)

	return withTimeout(checker, c.Timeout)
}

// dsPodsReadinessChecker checks that all daemonset pods are ready
type dsPodsReadinessChecker struct {
	access        kubernetes.Access
	namespace     string
	daemonSetName string
	timeout       time.Duration
}

func (c *dsPodsReadinessChecker) Check() check.Error {
	reqTimeout := int64(c.timeout.Seconds())

	// Get nodes
	nodes, err := listNodes(c.access.Kubernetes(), reqTimeout)
	if err != nil {
		return check.ErrUnknown("cannot get nodes in API: %v", err)
	}

	// Get daemonset
	ds, err := c.access.Kubernetes().AppsV1().DaemonSets(c.namespace).Get(c.daemonSetName, metav1.GetOptions{})
	if err != nil {
		return check.ErrUnknown("cannot get daemonset in API %s/%s: %v", c.namespace, c.daemonSetName, err)
	}

	// Filter node names of interest
	nodeNames := make(map[string]struct{})
	for _, node := range nodes {
		// Filter by status
		if !isNodeReady(&node) {
			continue
		}

		if node.Spec.Unschedulable {
			// If cordoned, something is happening, and we don't rely on this node
			continue
		}

		// Filter by DS tolerations
		if !isTolerated(node.Spec.Taints, ds.Spec.Template.Spec.Tolerations) {
			continue
		}

		nodeNames[node.Name] = struct{}{}
	}

	// Get daemonset pods
	pods, err := listDaemonSetPods(c.access.Kubernetes(), ds, reqTimeout)
	if err != nil {
		return check.ErrUnknown("cannot get pods of daemonset %s/%s: %v", c.namespace, c.daemonSetName, err)
	}

	// Check that all pods from desired nodes are ok
	for _, pod := range pods {
		_, ok := nodeNames[pod.Spec.NodeName]
		if !ok {
			// pod is not from a node of interest
			continue
		}

		// The node is ok, so the pod should be ok too.
		if !isPodReady(&pod) {
			return check.ErrFail("not all pods are running in daemonset %s/%s", c.namespace, c.daemonSetName)
		}

		// Seen nodes are of no interest anymore, but we want to know about unseen ones.
		delete(nodeNames, pod.Spec.NodeName)
	}

	// Check that there are no ready nodes without a pod
	if len(nodeNames) > 0 {
		names := make([]string, 0)
		for k := range nodeNames {
			names = append(names, k)
		}
		nodeNamesCommaList := strings.Join(names, ", ")
		return check.ErrFail("not all pods of daemonset %s/%s are running on desired nodes: %s",
			c.namespace, c.daemonSetName, nodeNamesCommaList)
	}

	return nil
}

func (c *dsPodsReadinessChecker) BusyWith() string {
	return fmt.Sprintf("getting daemonset %s/%s", c.namespace, c.daemonSetName)
}

func isNodeReady(node *v1.Node) bool {
	// if node.Status.Phase != v1.NodeRunning {
	// 	return false
	// }

	for _, cond := range node.Status.Conditions {
		if cond.Type != v1.NodeReady {
			// not the condition type we are looking for
			continue
		}

		if cond.Status != v1.ConditionTrue {
			// not ready? not ok
			return false
		}

		// NOTE 10 min hardcoded
		tenMinutesAgo := metav1.NewTime(time.Now().Add(-10 * time.Minute))
		isReadyLongEnough := cond.LastTransitionTime.Before(&tenMinutesAgo)
		return isReadyLongEnough
	}

	return false
}

// isTolerated checks if the given tolerations tolerates all taints
//
//      Copied from https://github.com/kubernetes/component-helpers/blob/v0.21.0/scheduling/corev1/helpers.go
//      It is not imported since k8s dependencies versions would require to rise to at least 0.20.
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
//      Copied from https://github.com/kubernetes/component-helpers/blob/v0.21.0/scheduling/corev1/helpers.go
//      It is not imported since k8s dependencies versions would require to rise to at least 0.20.
func tolerationsTolerateTaint(tolerations []v1.Toleration, taint *v1.Taint) bool {
	for i := range tolerations {
		if tolerations[i].ToleratesTaint(taint) {
			return true
		}
	}
	return false
}

func listNodes(kubernetes kubeclient.Interface, timeoutSeconds int64) ([]v1.Node, error) {
	nodeList, err := kubernetes.CoreV1().Nodes().List(metav1.ListOptions{TimeoutSeconds: &timeoutSeconds})
	if err != nil {
		return nil, err
	}

	return nodeList.Items, nil
}

func listDaemonSetPods(kubernetes kubeclient.Interface, ds *appsv1.DaemonSet, timeoutSeconds int64) ([]v1.Pod, error) {
	labelSelector := labels.FormatLabels(ds.Spec.Selector.MatchLabels)
	podList, err := kubernetes.CoreV1().Pods(ds.GetNamespace()).List(metav1.ListOptions{
		LabelSelector:  labelSelector,
		TimeoutSeconds: &timeoutSeconds,
	})
	if err != nil {
		return nil, err
	}

	return podList.Items, nil
}
