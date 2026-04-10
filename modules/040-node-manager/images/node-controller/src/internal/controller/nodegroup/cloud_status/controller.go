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
	"sort"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

type Service struct {
	Client client.Client
}

type Result struct {
	Desired     int32
	Min         int32
	Max         int32
	Instances   int32
	Failures    []common.MachineFailure
	IsFrozen    bool
	LatestError string
}

func (s *Service) Compute(ctx context.Context, ng *v1.NodeGroup) Result {
	result := Result{}
	if ng.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		return result
	}

	zonesCount := s.getZonesCount(ctx, ng)
	if ng.Spec.CloudInstances != nil {
		result.Min = ng.Spec.CloudInstances.MinPerZone * zonesCount
		result.Max = ng.Spec.CloudInstances.MaxPerZone * zonesCount
	}

	mdInfo := s.getMachineDeploymentInfo(ctx, ng.Name)
	result.Desired = mdInfo.Desired
	result.Failures = mdInfo.Failures
	result.IsFrozen = mdInfo.IsFrozen
	if result.Min > result.Desired {
		result.Desired = result.Min
	}
	result.Instances = s.getMachinesCount(ctx, ng.Name)

	if len(result.Failures) > 0 {
		sort.Slice(result.Failures, func(i, j int) bool {
			return result.Failures[i].Time.Before(result.Failures[j].Time)
		})
		result.LatestError = result.Failures[len(result.Failures)-1].Message
	}

	return result
}
