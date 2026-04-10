/*
Copyright 2025 Flant JSC

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
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

func (s *Service) getMachineDeploymentInfo(ctx context.Context, ngName string) (int32, []common.MachineFailure, bool) {
	var desired int32
	var failures []common.MachineFailure
	var isFrozen bool

	for _, gvk := range []schema.GroupVersionKind{common.MCMMachineDeploymentGVK, common.CAPIMachineDeploymentGVK} {
		mdList := &unstructured.UnstructuredList{}
		mdList.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
		if err := s.Client.List(ctx, mdList, client.InNamespace(common.MachineNamespace), client.MatchingLabels{"node-group": ngName}); err != nil {
			continue
		}
		for _, md := range mdList.Items {
			trackMachineDeploymentNodeGroupInfo(ngName, md.GetName())

			if replicas, found, _ := unstructured.NestedInt64(md.Object, "spec", "replicas"); found {
				desired += int32(replicas)
			}
			if conditions, found, _ := unstructured.NestedSlice(md.Object, "status", "conditions"); found {
				for _, c := range conditions {
					if cond, ok := c.(map[string]interface{}); ok && cond["type"] == "Frozen" && cond["status"] == "True" {
						isFrozen = true
					}
				}
			}
			if failedMachines, found, _ := unstructured.NestedSlice(md.Object, "status", "failedMachines"); found {
				for _, fm := range failedMachines {
					if fmMap, ok := fm.(map[string]interface{}); ok {
						mf := common.MachineFailure{Time: time.Now()}
						if name, _, _ := unstructured.NestedString(fmMap, "name"); name != "" {
							mf.MachineName = name
						}
						if providerID, _, _ := unstructured.NestedString(fmMap, "providerID"); providerID != "" {
							mf.ProviderID = providerID
						}
						if ownerRef, _, _ := unstructured.NestedString(fmMap, "ownerRef"); ownerRef != "" {
							mf.OwnerRef = ownerRef
						}
						if lastOp, _, _ := unstructured.NestedMap(fmMap, "lastOperation"); lastOp != nil {
							if msg, _, _ := unstructured.NestedString(lastOp, "description"); msg != "" {
								mf.Message = msg
							}
							if ts, _, _ := unstructured.NestedString(lastOp, "lastUpdateTime"); ts != "" {
								if t, err := time.Parse(time.RFC3339, ts); err == nil {
									mf.Time = t
								}
							}
							if state, _, _ := unstructured.NestedString(lastOp, "state"); state != "" {
								mf.State = state
							}
							if opType, _, _ := unstructured.NestedString(lastOp, "type"); opType != "" {
								mf.Type = opType
							}
						}
						if mf.Message != "" {
							failures = append(failures, mf)
						}
					}
				}
			}
		}
	}

	return desired, failures, isFrozen
}
