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
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

type machineDeploymentInfo struct {
	Desired  int32
	Failures []common.MachineFailure
	IsFrozen bool
}

func (s *Service) getMachineDeploymentInfo(ctx context.Context, ngName string) machineDeploymentInfo {
	var result machineDeploymentInfo

	if info, found := s.getMCMMachineDeploymentInfo(ctx, ngName); found {
		result = info
	}

	if info, found := s.getCAPIMachineDeploymentInfo(ctx, ngName); found {
		result.Desired += info.Desired
	}

	return result
}

// getMCMMachineDeploymentInfo collects replicas, frozen status, and failedMachines
// from MCM MachineDeployments. MCM stores failure details in status.failedMachines.
func (s *Service) getMCMMachineDeploymentInfo(ctx context.Context, ngName string) (machineDeploymentInfo, bool) {
	mdList := s.listMachineDeployments(ctx, common.MCMMachineDeploymentGVK, ngName)
	if mdList == nil {
		return machineDeploymentInfo{}, false
	}

	var info machineDeploymentInfo
	for _, md := range mdList.Items {
		trackMachineDeploymentNodeGroupInfo(ngName, md.GetName())

		if replicas, found, _ := unstructured.NestedInt64(md.Object, "spec", "replicas"); found {
			info.Desired += int32(replicas)
		}
		if conditions, found, _ := unstructured.NestedSlice(md.Object, "status", "conditions"); found {
			for _, c := range conditions {
				if cond, ok := c.(map[string]interface{}); ok && cond["type"] == "Frozen" && cond["status"] == "True" {
					info.IsFrozen = true
				}
			}
		}
		info.Failures = append(info.Failures, parseMCMFailedMachines(md.Object)...)
	}

	return info, true
}

// getCAPIMachineDeploymentInfo collects replicas from CAPI MachineDeployments.
// CAPI MachineDeployment does not have failedMachines in status;
// failure information must be collected from individual Machine resources (not yet implemented).
func (s *Service) getCAPIMachineDeploymentInfo(ctx context.Context, ngName string) (machineDeploymentInfo, bool) {
	mdList := s.listMachineDeployments(ctx, common.CAPIMachineDeploymentGVK, ngName)
	if mdList == nil {
		return machineDeploymentInfo{}, false
	}

	var info machineDeploymentInfo
	for _, md := range mdList.Items {
		trackMachineDeploymentNodeGroupInfo(ngName, md.GetName())

		if replicas, found, _ := unstructured.NestedInt64(md.Object, "spec", "replicas"); found {
			info.Desired += int32(replicas)
		}
	}

	return info, true
}

func (s *Service) listMachineDeployments(ctx context.Context, gvk schema.GroupVersionKind, ngName string) *unstructured.UnstructuredList {
	mdList := &unstructured.UnstructuredList{}
	mdList.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
	if err := s.Client.List(ctx, mdList, client.InNamespace(common.MachineNamespace), client.MatchingLabels{"node-group": ngName}); err != nil {
		return nil
	}
	if len(mdList.Items) == 0 {
		return nil
	}
	return mdList
}

func parseMCMFailedMachines(obj map[string]interface{}) []common.MachineFailure {
	failedMachines, found, _ := unstructured.NestedSlice(obj, "status", "failedMachines")
	if !found {
		return nil
	}

	var failures []common.MachineFailure
	for _, fm := range failedMachines {
		fmMap, ok := fm.(map[string]interface{})
		if !ok {
			continue
		}

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

	return failures
}
