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

package derived_status

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// Derive computes everything a NodeGroup's own spec does not say, from the snapshot: the engine,
// the effective Kubernetes version, the CRI type, the zones, the update epoch and the cloud
// overlay.
//
// It performs no I/O. The context is carried for logging only — every value it needs was read once
// in BuildSnapshot. The only error it can return comes from the inputs disagreeing (Containerd on
// a pre-1.19 cluster), never from a source being unavailable.
func Derive(ctx context.Context, ng *v1.NodeGroup, snap Snapshot) (Result, error) {
	logger := log.FromContext(ctx)

	result := Result{
		Engine:           ComputeEngine(ng, snap.Provider),
		SerializedLabels: serializeLabels(ng),
		SerializedTaints: serializeTaints(ng),
		UpdateEpoch:      calculateUpdateEpoch(epochTimestampAccessor(), snap.ClusterUUID, ng.Name),
	}

	effectiveKubeVer := effectiveKubernetesVersion(snap.TargetVersion, snap.APIServerMin)
	result.KubernetesVersion = semverMajMin(effectiveKubeVer)

	criType, err := resolveCRIType(ng, effectiveKubeVer, snap.DefaultCRI)
	if err != nil {
		return result, err
	}
	result.CRIType = criType

	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		deriveCloudFields(logger, ng, snap, &result)
	}

	logger.Info("derived status computed",
		"nodeGroup", ng.Name,
		"engine", result.Engine,
		"kubernetesVersion", result.KubernetesVersion,
		"targetVersion", versionString(snap.TargetVersion),
		"controlPlaneMinVersion", versionString(snap.APIServerMin),
		"defaultCRI", snap.DefaultCRI,
		"criType", result.CRIType,
		"updateEpoch", result.UpdateEpoch,
	)
	return result, nil
}

func deriveCloudFields(logger logr.Logger, ng *v1.NodeGroup, snap Snapshot, result *Result) {
	result.Zones = resolveZones(ng, snap.DefaultZones)

	if snap.CapacityErr != nil {
		logger.Error(snap.CapacityErr, "failed to calculate node capacity", "nodeGroup", ng.Name)
	} else {
		result.NodeCapacity = snap.Capacity
	}

	if snap.InstanceClass == nil {
		return
	}

	resolvedSpec, err := applyCloudSpecificDefaults(snap.Provider, snap.InstanceClass)
	if err != nil {
		logger.Error(err, "failed to apply cloud specific defaults", "nodeGroup", ng.Name)
		return
	}

	// Round-tripped through JSON on purpose: the spec arrives from an unstructured object with
	// int64 numbers, and the published element has always carried them as float64. toYaml renders
	// the two differently (4 vs 4e+00), and that rendering feeds the checksum that names an
	// immutable machine template.
	result.InstanceClass = normalizeJSONMap(resolvedSpec)
}
