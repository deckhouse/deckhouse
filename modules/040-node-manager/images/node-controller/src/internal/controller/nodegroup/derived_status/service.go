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
	"fmt"

	"github.com/Masterminds/semver/v3"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/capacity"
)

func versionString(v *semver.Version) string {
	if v == nil {
		return "<nil>"
	}
	return v.String()
}

// Service reads everything through the manager cache. All of its sources live in kube-system,
// which internal/common/cache.go caches whole precisely so this path never issues a live GET —
// a live GET here used to cost hundreds of ms on every pass during a NodeGroup burst.
type Service struct {
	Client client.Client
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

// ComputeWithCloudChecks derives get_crds fields and validation diagnostics from
// the same provider snapshot, matching the old hook's single-pass behavior.
func (s *Service) ComputeWithCloudChecks(ctx context.Context, ng *v1.NodeGroup) (Result, CloudCheckResult, error) {
	cloudProvider, err := s.readCloudProviderData(ctx)
	if err != nil {
		return Result{}, CloudCheckResult{}, err
	}
	result, err := s.compute(ctx, ng, cloudProvider)
	if err != nil {
		return result, CloudCheckResult{}, err
	}
	check, err := s.runCloudChecks(ctx, ng, cloudProvider)
	if err != nil {
		return result, CloudCheckResult{}, err
	}
	return result, check, nil
}

func (s *Service) compute(ctx context.Context, ng *v1.NodeGroup, cloudProvider map[string]interface{}) (Result, error) {
	logger := log.FromContext(ctx)

	result := Result{
		Engine:           ComputeEngine(ng, cloudProvider),
		SerializedLabels: serializeLabels(ng),
		SerializedTaints: serializeTaints(ng),
	}

	clusterUUID, err := s.readClusterUUID(ctx)
	if err != nil {
		return result, err
	}
	result.UpdateEpoch = calculateUpdateEpoch(epochTimestampAccessor(), clusterUUID, ng.Name)

	targetVersion, defaultCRI, err := s.readClusterConfiguration(ctx)
	if err != nil {
		return result, err
	}
	controlPlaneMinVersion, err := s.readControlPlaneMinVersion(ctx)
	if err != nil {
		return result, err
	}
	effectiveKubeVer := effectiveKubernetesVersion(targetVersion, controlPlaneMinVersion)
	result.KubernetesVersion = semverMajMin(effectiveKubeVer)

	criType, err := resolveCRIType(ng, effectiveKubeVer, defaultCRI)
	if err != nil {
		return result, err
	}
	result.CRIType = criType

	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		if err := s.computeCloudFields(ctx, ng, cloudProvider, &result); err != nil {
			return result, err
		}
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

func (s *Service) computeCloudFields(ctx context.Context, ng *v1.NodeGroup, cloudProvider map[string]interface{}, result *Result) error {
	logger := log.FromContext(ctx)

	defaultZones := s.readDefaultZones(ctx, cloudProvider)
	result.Zones = resolveZones(ng, defaultZones)

	if ng.Spec.CloudInstances == nil {
		return nil
	}
	kind := ng.Spec.CloudInstances.ClassReference.Kind
	name := ng.Spec.CloudInstances.ClassReference.Name
	if kind == "" || name == "" {
		return nil
	}

	// Without a published version there is no version to read the InstanceClass at, and guessing
	// one is what this whole mechanism exists to prevent. Describing the NodeGroup survives it —
	// rendering does not, and runCloudChecks reports it as a validation error.
	version := instanceClassAPIVersion(cloudProvider)
	if version == "" {
		logger.V(1).Info("cloud provider published no instanceClassAPIVersion, skipping capacity/instanceClass", "nodeGroup", ng.Name, "kind", kind, "name", name)
		return nil
	}

	instanceClassSpec, err := s.readInstanceClassSpec(ctx, version, kind, name)
	if err != nil {
		// A deleted InstanceClass is not a failure: RunCloudChecks already reports it as a
		// NodeGroup validation error, and instanceClass stays unset rather than guessed.
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("read instance class of %s: %w", ng.Name, err)
		}
		logger.V(1).Info("instance class not found, skipping capacity/instanceClass", "nodeGroup", ng.Name, "kind", kind, "name", name)
		return nil
	}
	if instanceClassSpec == nil {
		return nil
	}

	// nodeCapacity is only needed for scale-from-zero (min==0 && max>0).
	if ng.Spec.CloudInstances.MinPerZone == 0 && ng.Spec.CloudInstances.MaxPerZone > 0 {
		catalog := s.readInstanceTypesCatalog(ctx)
		nodeCapacity, err := capacity.CalculateNodeTemplateCapacity(kind, instanceClassSpec, catalog)
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
		return nil
	}
	if raw, err := json.Marshal(resolvedSpec); err == nil {
		result.InstanceClass = &runtime.RawExtension{Raw: raw}
	}
	return nil
}
