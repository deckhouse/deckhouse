package monitoring_and_autoscaling

import (
	"upmeter/pkg/checks"
)

// controlPlaneChecker checks the availability of API server.
// It is widely used as first step in other checkers.
type controlPlaneChecker struct {
	kubeAccessor *KubeAccessor
}

func newControlPlaneChecker(kubeAccessor *KubeAccessor) Checker {
	return &controlPlaneChecker{kubeAccessor}
}

func (c *controlPlaneChecker) Check() checks.Error {
	_, err := c.kubeAccessor.Kubernetes().Discovery().ServerVersion()
	if err != nil {
		return checks.ErrUnknownResult("control plane is unavailable: %v", err)
	}
	return nil
}

func (c *controlPlaneChecker) BusyWith() string {
	return "fetching kubernetes /version"
}
