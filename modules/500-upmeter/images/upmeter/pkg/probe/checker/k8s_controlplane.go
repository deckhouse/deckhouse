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
	"context"
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
	return doOrFail(c.Timeout, &k8sVersionGetter{access: c.Access})
}

// newControlPlaneChecker returns common preflight checker
func newControlPlaneChecker(access kubernetes.Access, timeout time.Duration) check.Checker {
	return doOrUnknown(timeout, newK8sVersionGetter(access))
}

// k8sVersionGetter returns non-nil err of API server version request fails
type k8sVersionGetter struct {
	access kubernetes.Access
}

func newK8sVersionGetter(access kubernetes.Access) *k8sVersionGetter {
	return &k8sVersionGetter{access: access}
}

func (c *k8sVersionGetter) Do(_ context.Context) error {
	_, err := c.access.Kubernetes().Discovery().ServerVersion()
	return err
}
