/*
Copyright 2026 Flant JSC

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

// Package evaluator turns the objects a project quota counts into the usage they add to the project.
// Every family of keys has an evaluator; the keys are plain strings, so a family the ResourceQuota
// objects do not count (virtual machines, load balancers) plugs in without changing the ledger.
package evaluator

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Evaluator is the usage of the objects of one family.
type Evaluator interface {
	// Family names the family, the value of a metric label.
	Family() string
	// Usage is what the object adds to the project, by quota key.
	Usage(object runtime.Object) (corev1.ResourceList, error)
}

// ResourceQuota is the family the ResourceQuota objects count: the usage of a namespace is the status
// of its d8-project-quota, which the quota plugin of the API server and the resource quota controller
// keep.
type ResourceQuota struct{}

// Family is "rq".
func (ResourceQuota) Family() string { return "rq" }

// Usage is status.used of the quota.
func (ResourceQuota) Usage(object runtime.Object) (corev1.ResourceList, error) {
	rq, ok := object.(*corev1.ResourceQuota)
	if !ok {
		return nil, fmt.Errorf("the rq family evaluates ResourceQuota objects, got %T", object)
	}
	return rq.Status.Used.DeepCopy(), nil
}
