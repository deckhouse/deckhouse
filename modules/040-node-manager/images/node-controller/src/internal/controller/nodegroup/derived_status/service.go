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
	"encoding/json"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

func versionString(v *semver.Version) string {
	if v == nil {
		return "<nil>"
	}
	return v.String()
}

type Service struct {
	Client client.Client
	Reader client.Reader
}

func (s *Service) reader() client.Reader {
	if s.Reader != nil {
		return s.Reader
	}
	return s.Client
}

func (s *Service) ComputeEngine(ctx context.Context, ng *v1.NodeGroup) string {
	return s.computeEngine(ng, s.readCloudProviderData(ctx))
}

// Result holds the get_crds-derived fields destined for NodeGroup.status.
type Result struct {
	Engine            string
	KubernetesVersion string
	CRIType           string
	Zones             []string
	NodeCapacity      *runtime.RawExtension
	InstanceClass     *runtime.RawExtension
	SerializedLabels  string
	SerializedTaints  string
	UpdateEpoch       string
}

// Compute derives the get_crds fields for a single NodeGroup.
func (s *Service) Compute(ctx context.Context, ng *v1.NodeGroup) (Result, error) {
	logger := log.FromContext(ctx)

	cloudProvider := s.readCloudProviderData(ctx)

	result := Result{
		Engine:           s.computeEngine(ng, cloudProvider),
		SerializedLabels: serializeLabels(ng),
		SerializedTaints: serializeTaints(ng),
	}

	clusterUUID := s.readClusterUUID(ctx)
	result.UpdateEpoch = calculateUpdateEpoch(epochTimestampAccessor(), clusterUUID, ng.Name)

	targetVersion, defaultCRI := s.readClusterConfiguration(ctx)
	controlPlaneMinVersion := s.readControlPlaneMinVersion(ctx)
	effectiveKubeVer := effectiveKubernetesVersion(targetVersion, controlPlaneMinVersion)
	result.KubernetesVersion = semverMajMin(effectiveKubeVer)

	criType, err := resolveCRIType(ng, effectiveKubeVer, defaultCRI)
	if err != nil {
		return result, err
	}
	result.CRIType = criType

	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		s.computeCloudFields(ctx, ng, cloudProvider, &result)
	}

	logger.Info("derived status computed",
		"nodeGroup", ng.Name,
		"engine", result.Engine,
		"kubernetesVersion", result.KubernetesVersion,
		"targetVersion", versionString(targetVersion),
		"controlPlaneMinVersion", versionString(controlPlaneMinVersion),
		"defaultCRI", defaultCRI,
		"criType", result.CRIType,
		"updateEpoch", result.UpdateEpoch,
	)
	return result, nil
}

func (s *Service) computeCloudFields(ctx context.Context, ng *v1.NodeGroup, cloudProvider map[string]interface{}, result *Result) {
	logger := log.FromContext(ctx)

	defaultZones := s.readDefaultZones(ctx, cloudProvider)
	result.Zones = resolveZones(ng, defaultZones)

	if ng.Spec.CloudInstances == nil {
		return
	}
	kind := ng.Spec.CloudInstances.ClassReference.Kind
	name := ng.Spec.CloudInstances.ClassReference.Name
	if kind == "" || name == "" {
		return
	}

	instanceClassSpec, err := s.readInstanceClassSpec(ctx, kind, name)
	if err != nil || instanceClassSpec == nil {
		if err != nil {
			// TEMP dev observability: Info (was V(1)) while stabilising the path.
			logger.Info("instance class not found, skipping capacity/instanceClass", "nodeGroup", ng.Name, "kind", kind, "name", name, "error", err.Error())
		}
		return
	}

	// nodeCapacity is only needed for scale-from-zero (min==0 && max>0).
	if ng.Spec.CloudInstances.MinPerZone == 0 && ng.Spec.CloudInstances.MaxPerZone > 0 {
		catalog := s.readInstanceTypesCatalog(ctx)
		nodeCapacity, err := calculateNodeCapacity(kind, instanceClassSpec, catalog)
		if err != nil {
			logger.Error(err, "failed to calculate node capacity", "nodeGroup", ng.Name, "kind", kind)
		} else if nodeCapacity != nil {
			if raw, err := json.Marshal(nodeCapacity); err == nil {
				result.NodeCapacity = &runtime.RawExtension{Raw: raw}
			}
		}
	}

	resolvedSpec, err := applyCloudSpecificDefaults(cloudProvider, instanceClassSpec)
	if err != nil {
		logger.Error(err, "failed to apply cloud specific defaults", "nodeGroup", ng.Name)
		return
	}
	if raw, err := json.Marshal(resolvedSpec); err == nil {
		result.InstanceClass = &runtime.RawExtension{Raw: raw}
	}
}
