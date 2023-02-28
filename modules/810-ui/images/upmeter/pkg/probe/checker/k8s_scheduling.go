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
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// PodScheduling is a checker constructor and configurator
type PodScheduling struct {
	Access    kubernetes.Access
	Namespace string
	Preflight Doer

	Node  string
	Image *kubernetes.ProbeImage

	AgentID string
	Name    string

	CreationTimeout time.Duration
	DeletionTimeout time.Duration
	ScheduleTimeout time.Duration
}

func (c PodScheduling) Checker() check.Checker {
	pod := createPodObject(c.Name, c.Node, c.AgentID, c.Image)

	getter := &podGetter{access: c.Access, namespace: c.Namespace, name: c.Name}

	creator := doWithTimeout(
		&podCreator{access: c.Access, namespace: c.Namespace, pod: pod},
		c.CreationTimeout,
		fmt.Errorf("creation timeout reached"),
	)

	deleter := doWithTimeout(
		&podDeleter{access: c.Access, namespace: c.Namespace, name: c.Name},
		c.DeletionTimeout,
		fmt.Errorf("creation timeout reached"),
	)

	fetcher := &pollingPodNodeFetcher{
		fetcher:  &podNodeNameFetcher{access: c.Access, namespace: c.Namespace, name: c.Name},
		timeout:  c.ScheduleTimeout,
		interval: c.ScheduleTimeout / 10,
	}

	checker := &podSchedulingChecker{
		preflight:   c.Preflight,
		creator:     creator,
		getter:      getter,
		deleter:     deleter,
		nodeFetcher: fetcher,
		node:        c.Node,
	}

	return checker
}

// podSchedulingChecker checks pod node. All apiserver related errors result in undetermined status.
type podSchedulingChecker struct {
	preflight Doer
	getter    Doer
	creator   Doer
	deleter   Doer

	nodeFetcher nodeNameFetcher
	node        string
}

func (c *podSchedulingChecker) Check() check.Error {
	ctx := context.TODO()
	if err := c.preflight.Do(ctx); err != nil {
		return check.ErrUnknown("preflight: %v", err)
	}

	// Check garbage
	if getErr := c.getter.Do(ctx); getErr != nil && !apierrors.IsNotFound(getErr) {
		// Unexpected apiserver error
		return check.ErrUnknown("getting garbage: %v", getErr)
	} else if getErr == nil {
		// Garbage object exists, cleaning it and skipping this run.
		if delErr := c.deleter.Do(ctx); delErr != nil {
			return check.ErrUnknown("deleting garbage: %v", delErr)
		}
		return check.ErrUnknown("cleaned garbage")
	}

	// The actual check
	if createErr := c.creator.Do(ctx); createErr != nil {
		// Unexpected error
		return check.ErrUnknown("creating: %v", createErr)
	}
	if node, fetchErr := c.nodeFetcher.Node(ctx); fetchErr != nil {
		_ = c.deleter.Do(ctx) // Cleanup
		return check.ErrUnknown("getting: %v", fetchErr)
	} else if node != c.node {
		_ = c.deleter.Do(ctx) // Cleanup
		return check.ErrFail("verification: got node %s, expected %s", node, c.node)
	}
	if delErr := c.deleter.Do(ctx); delErr != nil {
		// Unexpected error
		return check.ErrUnknown("deleting: %v", delErr)
	}

	return nil
}

type podCreator struct {
	access    kubernetes.Access
	namespace string
	pod       *v1.Pod
}

func (c *podCreator) Do(ctx context.Context) error {
	client := c.access.Kubernetes()
	_, err := client.CoreV1().Pods(c.namespace).Create(ctx, c.pod, metav1.CreateOptions{})
	return err
}

type podGetter struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (c *podGetter) Do(ctx context.Context) error {
	client := c.access.Kubernetes()
	_, err := client.CoreV1().Pods(c.namespace).Get(ctx, c.name, metav1.GetOptions{})
	return err
}

type podDeleter struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (c *podDeleter) Do(ctx context.Context) error {
	client := c.access.Kubernetes()
	err := client.CoreV1().Pods(c.namespace).Delete(ctx, c.name, metav1.DeleteOptions{})
	return err
}

type nodeNameFetcher interface {
	Node(context.Context) (string, error)
}

type podNodeNameFetcher struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (c *podNodeNameFetcher) Node(ctx context.Context) (string, error) {
	client := c.access.Kubernetes()
	pod, err := client.CoreV1().Pods(c.namespace).Get(ctx, c.name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return pod.Spec.NodeName, nil
}

type pollingPodNodeFetcher struct {
	fetcher  nodeNameFetcher
	timeout  time.Duration
	interval time.Duration
}

func (f *pollingPodNodeFetcher) Node(ctx context.Context) (node string, err error) {
	ticker := time.NewTicker(f.interval)
	deadline := time.NewTimer(f.timeout)

	defer ticker.Stop()
	defer deadline.Stop()

	for {
		select {
		case <-ticker.C:
			node, err = f.fetcher.Node(ctx)
			if err != nil {
				// apiserver fail
				return "", err
			}
			if node != "" {
				// scheduling success
				return node, nil
			}
		case <-deadline.C:
			return "", fmt.Errorf("node polling timeout reached")
		}
	}
}
