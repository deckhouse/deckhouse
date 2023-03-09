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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Node struct {
	Name        string
	Zone        string
	Schedulable bool
}

const zoneLabelKey = "topology.kubernetes.io/zone"

func NewNode(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	node := new(v1.Node)
	err := sdk.FromUnstructured(obj, node)
	if err != nil {
		return nil, fmt.Errorf("cannot deserialize Node %q: %w", obj.GetName(), err)
	}

	zone := defaultZone
	labels := node.GetLabels()
	if z, ok := labels[zoneLabelKey]; ok {
		zone = z
	}

	var isReady bool
	for _, cond := range node.Status.Conditions {
		if cond.Type != v1.NodeReady {
			continue
		}
		isReady = cond.Status == v1.ConditionTrue
		break
	}

	var markedForScaleDown bool
	for _, taint := range node.Spec.Taints {
		if taint.Key == "ToBeDeletedByClusterAutoscaler" {
			markedForScaleDown = true
			break
		}
	}

	n := Node{
		Name:        node.GetName(), // node name and hostname always equal
		Zone:        zone,
		Schedulable: !node.Spec.Unschedulable && isReady && !markedForScaleDown,
	}
	return n, nil
}
