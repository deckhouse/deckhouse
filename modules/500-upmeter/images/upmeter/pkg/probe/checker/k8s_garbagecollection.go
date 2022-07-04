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
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"d8.io/upmeter/pkg/check"
	k8s "d8.io/upmeter/pkg/kubernetes"
)

// garbageCollectorChecker ensures objects can be listed and their deletion is complete
type garbageCollectorChecker struct {
	access    k8s.Access
	namespace string
	kind      string
	listOpts  *metav1.ListOptions
	timeout   time.Duration
}

func newGarbageCollectorCheckerByName(access k8s.Access, kind, namespace, name string, timeout time.Duration) check.Checker {
	return &garbageCollectorChecker{
		access:    access,
		namespace: namespace,
		kind:      kind,
		listOpts:  listOptsByName(name),
		timeout:   timeout,
	}
}

func newGarbageCollectorCheckerByLabels(access k8s.Access, kind, namespace string, labels map[string]string, timeout time.Duration) check.Checker {
	return &garbageCollectorChecker{
		access:    access,
		namespace: namespace,
		kind:      kind,
		listOpts:  listOptsByLabels(labels),
		timeout:   timeout,
	}
}

func (c *garbageCollectorChecker) Check() check.Error {
	if err := c.collect(); err != nil {
		return check.ErrUnknown(err.Error())
	}
	return nil
}

func (c *garbageCollectorChecker) collect() error {
	var err error
	var list []string

	client := c.access.Kubernetes()

	list, err = listObjects(client, c.kind, c.namespace, *c.listOpts)
	if err != nil {
		return fmt.Errorf("listing %s/%s: %v", c.namespace, c.kind, err)
	}
	if len(list) == 0 {
		return nil
	}

	err = deleteObjects(client, c.kind, c.namespace, list)
	if err != nil {
		return fmt.Errorf("cannot clean garbage %s/%s: %v", c.namespace, c.kind, err)
	}

	// Take some time to ensure objects are deleted
	tries := int(c.timeout.Seconds())
	for ; tries > 0; tries-- {
		time.Sleep(time.Second)

		list, err = listObjects(client, c.kind, c.namespace, *c.listOpts)
		if err == nil && len(list) == 0 {
			return nil
		}
	}

	if err != nil {
		return fmt.Errorf("garbage still present in %ss (names=%s): %v", c.kind, dumpNames(list), err)
	}
	return fmt.Errorf("garbage still present in %ss (names=%s)", c.kind, dumpNames(list))
}
