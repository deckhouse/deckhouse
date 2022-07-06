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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	k8s "d8.io/upmeter/pkg/kubernetes"
	"d8.io/upmeter/pkg/probe/run"
)

// NamespaceLifecycle is a checker constructor and configurator
type NamespaceLifecycle struct {
	Access                    k8s.Access
	CreationTimeout           time.Duration
	DeletionTimeout           time.Duration
	GarbageCollectionTimeout  time.Duration
	ControlPlaneAccessTimeout time.Duration
}

func (c NamespaceLifecycle) Checker() check.Checker {
	return &namespaceLifeCycleChecker{
		access:                    c.Access,
		creationTimeout:           c.CreationTimeout,
		deletionTimeout:           c.DeletionTimeout,
		garbageCollectorTimeout:   c.GarbageCollectionTimeout,
		controlPlaneAccessTimeout: c.ControlPlaneAccessTimeout,
	}
}

type namespaceLifeCycleChecker struct {
	access          k8s.Access
	creationTimeout time.Duration
	deletionTimeout time.Duration

	garbageCollectorTimeout   time.Duration
	controlPlaneAccessTimeout time.Duration

	// inner state
	checker check.Checker
}

func (c *namespaceLifeCycleChecker) Check() check.Error {
	namespace := createNamespaceObject()
	c.checker = c.new(namespace)
	return c.checker.Check()
}

/*
1. check control plane availability
2. collect the garbage of the namespace from previous runs
3. create and delete the namespace in api
4. ensure it does not exist (with retries)
*/
func (c *namespaceLifeCycleChecker) new(namespace *v1.Namespace) check.Checker {
	kind := namespace.GetObjectKind().GroupVersionKind().Kind
	name := namespace.GetName()

	pingControlPlane := newControlPlaneChecker(c.access, c.controlPlaneAccessTimeout)
	collectGarbage := newGarbageCollectorCheckerByName(c.access, kind, "", name, c.garbageCollectorTimeout)

	createNamespace := withTimeout(
		&namespaceCreationChecker{access: c.access, namespace: namespace},
		c.creationTimeout)

	deleteNamespace := withTimeout(
		&namespaceDeletionChecker{access: c.access, namespace: namespace},
		c.deletionTimeout)

	verifyDeletion := withRetryEachSeconds(
		&objectIsNotListedChecker{
			access:   c.access,
			kind:     kind,
			listOpts: listOptsByName(name),
		},
		c.garbageCollectorTimeout,
	)

	return sequence(
		pingControlPlane,
		collectGarbage,
		createNamespace,
		deleteNamespace,
		verifyDeletion,
	)
}

// namespaceCreationChecker creates namespace
type namespaceCreationChecker struct {
	access    k8s.Access
	namespace *v1.Namespace
}

func (c *namespaceCreationChecker) Check() check.Error {
	client := c.access.Kubernetes()
	_, err := client.CoreV1().Namespaces().Create(c.namespace)
	if err != nil {
		return check.ErrUnknown("cannot create namespace %q: %v", c.namespace.GetName(), err)
	}
	return nil
}

// namespaceDeletionChecker deletes namespace
type namespaceDeletionChecker struct {
	access    k8s.Access
	namespace *v1.Namespace
}

func (c *namespaceDeletionChecker) Check() check.Error {
	client := c.access.Kubernetes()
	err := client.CoreV1().Namespaces().Delete(c.namespace.GetName(), &metav1.DeleteOptions{})
	if err != nil {
		return check.ErrFail("cannot delete namespace %q: %v", c.namespace.GetName(), err)
	}
	return nil
}

func createNamespaceObject() *v1.Namespace {
	name := run.StaticIdentifier("upmeter-control-plane-namespace") // TODO check alerts

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
