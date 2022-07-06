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
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/run"
)

// NamespaceLifecycle2 is a checker constructor and configurator
type NamespaceLifecycle2 struct {
	Access          kubernetes.Access
	CreationTimeout time.Duration
	DeletionTimeout time.Duration
}

func (c NamespaceLifecycle2) Checker() check.Checker {
	preflight := newK8sVersionGetter(c.Access)

	name := run.StaticIdentifier("upmeter-probe-basic")

	getter := &namespaceGetter{access: c.Access, name: name}

	creator := doWithTimeout(
		&namespaceCreator{access: c.Access, name: name},
		c.CreationTimeout,
		fmt.Errorf("creation timeout reached"),
	)

	deleter := doWithTimeout(
		&namespaceDeleter{access: c.Access, name: name, timeout: c.DeletionTimeout},
		c.DeletionTimeout,
		fmt.Errorf("deletion timeout reached"),
	)

	checker := &KubeObjectBasicLifecycle{
		preflight: preflight,
		creator:   creator,
		getter:    getter,
		deleter:   deleter,
	}

	return checker
}

type namespaceCreator struct {
	access  kubernetes.Access
	name    string
	timeout time.Duration
}

func (c *namespaceCreator) Do(_ context.Context) error {
	client := c.access.Kubernetes()
	ns := createNamespaceObject(c.name)
	_, err := client.CoreV1().Namespaces().Create(ns)
	return err
}

type namespaceGetter struct {
	access kubernetes.Access
	name   string
}

func (c *namespaceGetter) Do(_ context.Context) error {
	client := c.access.Kubernetes()
	_, err := client.CoreV1().Namespaces().Get(c.name, metav1.GetOptions{})
	return err
}

type namespaceDeleter struct {
	access  kubernetes.Access
	name    string
	timeout time.Duration
}

func (c *namespaceDeleter) Do(_ context.Context) error {
	client := c.access.Kubernetes()
	err := client.CoreV1().Namespaces().Delete(c.name, &metav1.DeleteOptions{})
	return err
}

func createNamespaceObject(name string) *v1.Namespace {
	return &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"heritage":      "upmeter",
				"upmeter-agent": run.ID(),
				"upmeter-group": "control-plane",
				"upmeter-probe": "namespace",
			},
		},
	}
}
