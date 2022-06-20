/*
Copyright 2021 Flant JSC

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
	"time"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// ControlPlaneAvailable is a checker constructor and configurator
type ControlPlaneAvailable struct {
	Access  kubernetes.Access
	Timeout time.Duration
}

func (c ControlPlaneAvailable) Checker() check.Checker {
	return failOnError(newControlPlaneChecker(c.Access, c.Timeout))
}

// controlPlaneChecker checks the availability of API server. It reports Unknown status if cannot access the
// API server. It is widely used as first step in other checkers.
type controlPlaneChecker struct {
	access kubernetes.Access
}

func (c *controlPlaneChecker) Check() check.Error {
	_, err := c.access.Kubernetes().Discovery().ServerVersion()
	if err != nil {
		return check.ErrUnknown("control plane is unavailable: %v", err)
	}
	return nil
}

// newControlPlaneChecker returns the checker wrapped with timeout to be use it in other checkers as precondition
func newControlPlaneChecker(access kubernetes.Access, timeout time.Duration) check.Checker {
	return withTimeout(&controlPlaneChecker{access}, timeout)
}
