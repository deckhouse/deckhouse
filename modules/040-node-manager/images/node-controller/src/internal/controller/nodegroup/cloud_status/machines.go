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

	"sigs.k8s.io/controller-runtime/pkg/client"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

func (s *Service) getMachinesCount(ctx context.Context, ngName string) int32 {
	var count int32

	mcmList := &mcmv1alpha1.MachineList{}
	if err := s.Client.List(ctx, mcmList, client.InNamespace(common.MachineNamespace)); err == nil {
		for i := range mcmList.Items {
			if mcmList.Items[i].Spec.NodeTemplateSpec.Labels[common.NodeGroupLabel] == ngName {
				count++
			}
		}
	}

	capiList := &capiv1beta2.MachineList{}
	if err := s.Client.List(ctx, capiList, client.InNamespace(common.MachineNamespace), client.MatchingLabels{"node-group": ngName}); err == nil {
		count += int32(len(capiList.Items))
	}

	return count
}
