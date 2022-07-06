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
	"fmt"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/run"
)

// PodScheduling is a checker constructor and configurator
type PodScheduling struct {
	Access          kubernetes.Access
	Timeout         time.Duration
	Namespace       string
	Node            string
	Image           *kubernetes.ProbeImage
	CreationTimeout time.Duration
}

func (c PodScheduling) Checker() check.Checker {
	preflight := newK8sVersionGetter(c.Access)

	name := run.StaticIdentifier("upmeter-probe-basic")
	pod := createPodObjectWithName(name, c.Node, c.Image)

	creator := doWithTimeout(
		&podCreator{access: c.Access, namespace: c.Namespace, pod: pod},
		c.CreationTimeout,
		fmt.Errorf("cration timeout reached"),
	)

	getter := &podGetter{access: c.Access, namespace: c.Namespace, name: name}
	deleter := &podDeleter{access: c.Access, namespace: c.Namespace, name: name}
	fetcher := &podPhaseFetcherImpl{access: c.Access, namespace: c.Namespace, name: name}

	checker := &podPhaseChecker{
		preflight:   preflight,
		creator:     creator,
		getter:      getter,
		deleter:     deleter,
		nodeFetcher: fetcher,
		node:        c.Node,
	}

	return withTimeout(checker, c.Timeout)
}

type podCreator struct {
	access    kubernetes.Access
	namespace string
	pod       *v1.Pod
}

func (c *podCreator) Do(_ context.Context) error {
	client := c.access.Kubernetes()
	_, err := client.CoreV1().Pods(c.namespace).Create(c.pod)
	return err
}

type podGetter struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (c *podGetter) Do(_ context.Context) error {
	client := c.access.Kubernetes()
	_, err := client.CoreV1().Pods(c.namespace).Get(c.name, metav1.GetOptions{})
	return err
}

type podDeleter struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (c *podDeleter) Do(_ context.Context) error {
	client := c.access.Kubernetes()
	err := client.CoreV1().Pods(c.namespace).Delete(c.name, &metav1.DeleteOptions{})
	return err
}

type podNodeFetcher interface {
	Node(_ context.Context) (string, error)
}

type podPhaseFetcherImpl struct {
	access    kubernetes.Access
	namespace string
	name      string
}

func (c *podPhaseFetcherImpl) Node(_ context.Context) (string, error) {
	client := c.access.Kubernetes()
	pod, err := client.CoreV1().Pods(c.namespace).Get(c.name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return pod.Spec.NodeName, nil
}

// podPhaseChecker checks a condition within an object lifecycle.
// Hence, all errors in kube-apiserver calls result in undetermined check status.
type podPhaseChecker struct {
	preflight   doer
	getter      doer
	creator     doer
	deleter     doer
	nodeFetcher podNodeFetcher
	node        string
}

func (c *podPhaseChecker) Check() check.Error {
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
	} else {
		if node != c.node {
			_ = c.deleter.Do(ctx) // Cleanup
			return check.ErrFail("verification: got node %s, expected %s", node, c.node)
		}
	}
	if delErr := c.deleter.Do(ctx); delErr != nil {
		// Unexpected error
		return check.ErrUnknown("deleting: %v", delErr)
	}

	return nil
}
