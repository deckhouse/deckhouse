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
	"errors"
	"sync"
	"time"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// ControlPlaneAvailable is a checker constructor and configurator
type ControlPlaneAvailable struct {
	VersionGetter Doer
	Timeout       time.Duration
}

func (c ControlPlaneAvailable) Checker() check.Checker {
	return doOrFail(c.Timeout, c.VersionGetter)
}

// k8sVersionGetter returns non-nil err of API server version request fails
type k8sVersionGetter struct {
	access   kubernetes.Access
	err      error
	mu       sync.RWMutex
	interval time.Duration
}

func NewK8sVersionGetter(access kubernetes.Access, interval time.Duration) *k8sVersionGetter {
	return &k8sVersionGetter{
		access:   access,
		interval: interval,
	}
}

func (c *k8sVersionGetter) Do(_ context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.err
}

func (c *k8sVersionGetter) Start() {
	ticker := time.NewTicker(c.interval)

	go func() {
		c.fetch() // force initiall call

		for range ticker.C {
			c.fetch()
		}
	}()
}

func (c *k8sVersionGetter) fetch() {
	_, err := c.access.Kubernetes().Discovery().ServerVersion()

	c.mu.RLock()
	prevErr := c.err
	c.mu.RUnlock()

	if errors.Is(prevErr, err) {
		// The error did not change, no need to block
		return
	}

	c.mu.Lock()
	c.err = err
	c.mu.Unlock()
}
