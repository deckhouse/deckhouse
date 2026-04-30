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

package cloud_status

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

func (s *Service) getMachinesCount(ctx context.Context, ngName string) int32 {
	var count int32

	// MCM Machines.
	mcmList := &unstructured.UnstructuredList{}
	mcmList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   common.MCMMachineGVK.Group,
		Version: common.MCMMachineGVK.Version,
		Kind:    "MachineList",
	})
	if err := s.Client.List(ctx, mcmList, client.InNamespace(common.MachineNamespace)); err == nil {
		for _, m := range mcmList.Items {
			if labels, found, _ := unstructured.NestedStringMap(m.Object, "spec", "nodeTemplate", "metadata", "labels"); found && labels[common.NodeGroupLabel] == ngName {
				count++
			}
		}
	}

	// CAPI Machines.
	capiList := &unstructured.UnstructuredList{}
	capiList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   common.CAPIMachineGVK.Group,
		Version: common.CAPIMachineGVK.Version,
		Kind:    "MachineList",
	})
	if err := s.Client.List(ctx, capiList, client.InNamespace(common.MachineNamespace), client.MatchingLabels{"node-group": ngName}); err == nil {
		count += int32(len(capiList.Items))
	}

	return count
}
