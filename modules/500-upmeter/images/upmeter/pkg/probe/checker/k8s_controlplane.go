package checker

import (
	"time"

	"upmeter/pkg/check"
	"upmeter/pkg/kubernetes"
)

// ControlPlaneAvailable is a checker constructor and configurator
type ControlPlaneAvailable struct {
	Access  *kubernetes.Access
	Timeout time.Duration
}

func (c ControlPlaneAvailable) Checker() check.Checker {
	checker := failOnError(&controlPlaneChecker{c.Access})
	return withTimeout(checker, c.Timeout)
}

// controlPlaneChecker checks the availability of API server. It reports Unknown status if cannot access the
// API server. It is widely used as first step in other checkers.
type controlPlaneChecker struct {
	access *kubernetes.Access
}

func (c *controlPlaneChecker) Check() check.Error {
	_, err := c.access.Kubernetes().Discovery().ServerVersion()
	if err != nil {
		return check.ErrUnknown("control plane is unavailable: %v", err)
	}
	return nil
}

func (c *controlPlaneChecker) BusyWith() string {
	return "fetching kubernetes /version"
}
