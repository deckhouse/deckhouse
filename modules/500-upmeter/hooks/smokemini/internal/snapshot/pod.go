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

package snapshot

import (
	"fmt"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Pod struct {
	Index   string
	Node    string
	Ready   bool
	Created time.Time
}

func NewPod(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	pod := new(v1.Pod)
	err := sdk.FromUnstructured(obj, pod)
	if err != nil {
		return nil, fmt.Errorf("cannot deserialize Pod %q: %w", obj.GetName(), err)
	}

	var ready bool
	for _, cond := range pod.Status.Conditions {
		if cond.Type != v1.PodReady {
			continue
		}
		ready = cond.Status == v1.ConditionTrue
		break
	}

	sp := Pod{
		Index:   IndexFromPodName(pod.GetName()).String(),
		Node:    pod.Spec.NodeName, // node name and hostname are equal
		Ready:   ready,
		Created: pod.CreationTimestamp.Time,
	}

	return sp, nil
}

type PodPhase struct {
	Name      string
	IsPending bool
}

// Index returns parsed smoke-mini index
func (pp PodPhase) Index() Index {
	return IndexFromPodName(pp.Name)
}

func NewPodPhase(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	phase, found, err := unstructured.NestedString(obj.Object, "status", "phase")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("failed to find Pod phase in the unstructured")
	}

	ret := PodPhase{
		Name:      obj.GetName(),
		IsPending: v1.PodPhase(phase) == v1.PodPending,
	}
	return ret, nil
}

func NewDisruption(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	n, ok, err := unstructured.NestedInt64(obj.Object, "status", "disruptionsAllowed")
	if err != nil {
		return nil, fmt.Errorf("cannot get allowed disruptions from PDB %q: %w", obj.GetName(), err)
	}
	if !ok {
		return nil, fmt.Errorf("cannot get status.disruptionsAllowed from unstructured PDB")
	}
	return n > 0, nil
}
