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

package processed_status

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/nodegroupfilter"
)

func ApplyNodeGroupCRDFilter(obj *unstructured.Unstructured) (interface{}, error) {
	var ng nodegroupfilter.NodeGroup
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &ng); err != nil {
		return nil, err
	}

	return common.NodeGroupCRDInfo{
		Name:            ng.GetName(),
		Spec:            ng.Spec,
		ManualRolloutID: ng.GetAnnotations()["manual-rollout-id"],
	}, nil
}
