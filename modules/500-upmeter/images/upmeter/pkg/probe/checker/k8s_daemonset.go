package checker

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"upmeter/pkg/check"
	"upmeter/pkg/kubernetes"
)

// AllDaemonSetPodsReady is a checker constructor and configurator
type AllDaemonSetPodsReady struct {
	Access    *kubernetes.Access
	Timeout   time.Duration
	Namespace string
	Name      string
}

func (c AllDaemonSetPodsReady) Checker() check.Checker {
	dsChecker := &dsAllPodsReadinessChecker{
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

// dsAllPodsReadinessChecker checks that all daemonset pods are ready
type dsAllPodsReadinessChecker struct {
	access        *kubernetes.Access
	namespace     string
	daemonSetName string
}

func (c *dsAllPodsReadinessChecker) Check() check.Error {
	ds, err := c.access.Kubernetes().AppsV1().DaemonSets(c.namespace).Get(c.daemonSetName, metav1.GetOptions{})
	if err != nil {
		return check.ErrUnknown("cannot get daemonset in API %s/%s: %v", c.namespace, c.daemonSetName, err)
	}
	if ds.Status.DesiredNumberScheduled == ds.Status.NumberReady {
		return nil
	}
	return check.ErrFail("not all pods are ready in daemonset %s/%s", c.namespace, c.daemonSetName)
}

func (c *dsAllPodsReadinessChecker) BusyWith() string {
	return fmt.Sprintf("getting daemonset %s/%s", c.namespace, c.daemonSetName)
}
