package monitoring_and_autoscaling

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"upmeter/pkg/checks"
)

// newAnyPodReadyChecker creates checker for at least one ready pod.
// It checks that the API server is available before checking the pods.
//
// Linter detects the same timeout value all the time and suggests we put it right into this function.
//nolint:unparam
func newAnyPodReadyChecker(kubeAccessor *KubeAccessor, timeout time.Duration, namespace, labelSelector string) Checker {
	podsChecker := &podReadinessChecker{
		Namespace:     namespace,
		LabelSelector: labelSelector,
		kubeAccessor:  kubeAccessor,
	}

	checker := NewSequentialChecker(
		newControlPlaneChecker(kubeAccessor),
		podsChecker,
	)

	return withTimeout(checker, timeout)
}

// podReadinessChecker defines the information that lets check at least one ready pod
type podReadinessChecker struct {
	Namespace     string
	LabelSelector string
	kubeAccessor  *KubeAccessor
}

func (c *podReadinessChecker) Check() checks.Error {
	podList, err := c.kubeAccessor.Kubernetes().CoreV1().Pods(c.Namespace).List(metav1.ListOptions{LabelSelector: c.LabelSelector})
	if err != nil {
		return checks.ErrUnknownResult("cannot get pods in API %s: %v", podFilter(c.Namespace, c.LabelSelector), err)

	}

	for _, pod := range podList.Items {
		if isPodReady(&pod) {
			return nil
		}
	}

	return checks.ErrFail("no ready pods found %s", podFilter(c.Namespace, c.LabelSelector))
}

func (c *podReadinessChecker) BusyWith() string {
	return fmt.Sprintf("getting pods %s", podFilter(c.Namespace, c.LabelSelector))
}

func isPodReady(pod *v1.Pod) bool {
	if pod.Status.Phase != v1.PodRunning {
		return false
	}

	for _, cnd := range pod.Status.Conditions {
		if cnd.Status != v1.ConditionTrue {
			return false
		}
	}

	return true
}

func podFilter(namespace, labelSelector string) string {
	return fmt.Sprintf("%s,%s", namespace, labelSelector)
}

// Linter detects the same timeout value all the time and suggests we put it right into this function.
//nolint:unparam
func newAllDaemonsetPodsReadyChecker(kubeAccessor *KubeAccessor, timeout time.Duration, namespace, dsName string) Checker {
	dsChecker := &dsAllPodsReadinessChecker{
		Namespace:     namespace,
		DaemonSetName: dsName,
		kubeAccessor:  kubeAccessor,
	}

	checker := NewSequentialChecker(
		newControlPlaneChecker(kubeAccessor),
		dsChecker,
	)

	return withTimeout(checker, timeout)
}

// podReadinessChecker defines the information that lets check at least one ready pod
type dsAllPodsReadinessChecker struct {
	Namespace     string
	DaemonSetName string
	kubeAccessor  *KubeAccessor
}

func (c *dsAllPodsReadinessChecker) Check() checks.Error {
	ds, err := c.kubeAccessor.Kubernetes().AppsV1().DaemonSets(c.Namespace).Get(c.DaemonSetName, metav1.GetOptions{})
	if err != nil {
		return checks.ErrUnknownResult("cannot get daemonset in API %s/%s: %v", c.Namespace, c.DaemonSetName, err)
	}
	if ds.Status.DesiredNumberScheduled == ds.Status.NumberReady {
		return nil
	}
	return checks.ErrFail("not all pods are ready in daemonset %s/%s", c.Namespace, c.DaemonSetName)
}

func (c *dsAllPodsReadinessChecker) BusyWith() string {
	return fmt.Sprintf("getting daemonset %s/%s", c.Namespace, c.DaemonSetName)
}
