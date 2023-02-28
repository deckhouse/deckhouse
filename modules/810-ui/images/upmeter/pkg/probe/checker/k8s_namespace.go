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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	"d8.io/upmeter/pkg/kubernetes"
)

// NamespaceLifecycle is a checker constructor and configurator
type NamespaceLifecycle struct {
	Access    kubernetes.Access
	Preflight Doer

	AgentID string
	Name    string

	CreationTimeout time.Duration
	DeletionTimeout time.Duration
}

func (c NamespaceLifecycle) Checker() check.Checker {
	getter := &namespaceGetter{access: c.Access, name: c.Name}

	ns := createNamespaceObject(c.Name, c.AgentID)
	creator := doWithTimeout(
		&namespaceCreator{access: c.Access, ns: ns},
		c.CreationTimeout,
		fmt.Errorf("creation timeout reached"),
	)

	deleter := doWithTimeout(
		&namespaceDeleter{access: c.Access, name: c.Name, timeout: c.DeletionTimeout},
		c.DeletionTimeout,
		fmt.Errorf("deletion timeout reached"),
	)

	checker := &KubeObjectBasicLifecycle{
		preflight: c.Preflight,
		creator:   creator,
		getter:    getter,
		deleter:   deleter,
	}

	return checker
}

type namespaceCreator struct {
	access kubernetes.Access
	ns     *v1.Namespace
}

func (c *namespaceCreator) Do(ctx context.Context) error {
	client := c.access.Kubernetes()
	_, err := client.CoreV1().Namespaces().Create(ctx, c.ns, metav1.CreateOptions{})
	return err
}

type namespaceGetter struct {
	access kubernetes.Access
	name   string
}

func (c *namespaceGetter) Do(ctx context.Context) error {
	client := c.access.Kubernetes()
	_, err := client.CoreV1().Namespaces().Get(ctx, c.name, metav1.GetOptions{})
	return err
}

type namespaceDeleter struct {
	access  kubernetes.Access
	name    string
	timeout time.Duration
}

func (c *namespaceDeleter) Do(ctx context.Context) error {
	client := c.access.Kubernetes()
	err := client.CoreV1().Namespaces().Delete(ctx, c.name, metav1.DeleteOptions{})
	return err
}

func createNamespaceObject(name, agentID string) *v1.Namespace {
	return &v1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"heritage":      "upmeter",
				agentLabelKey:   agentID,
				"upmeter-group": "control-plane",
				"upmeter-probe": "namespace",
			},
		},
	}
}
