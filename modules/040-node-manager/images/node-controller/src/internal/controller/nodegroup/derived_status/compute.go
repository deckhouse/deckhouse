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
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/labels"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/capacity"
)

// engine values, kept in sync with hooks/internal/v1.NodeGroupEngine*.
const (
	engineNone = "None"
	engineMCM  = "MCM"
	engineCAPI = "CAPI"
)

const useMCMAnnotation = "node.deckhouse.io/use-mcm"

// CRI resolution constants, mirrors get_crds.go.
const (
	criTypeDocker           = "Docker"
	criTypeContainerd       = "Containerd"
	criTypeNotManaged       = "NotManaged"
	nodeGroupDefaultCRIType = criTypeContainerd
)

// epochTimestampAccessor mirrors get_crds.go; overridable in tests.
var epochTimestampAccessor = func() int64 {
	return time.Now().Unix()
}

// computeEngine mirrors get_crds.calculateNodeGroupEngine. The engine is
// sticky: once observed in status it is preserved.
func (s *Service) computeEngine(ng *v1.NodeGroup, cloudProvider map[string]interface{}) string {
	if ng.Status.Engine != "" {
		return ng.Status.Engine
	}

	useMCM := ng.GetAnnotations()[useMCMAnnotation] != ""
	defaultEngine := defaultCloudEphemeralEngine(cloudProvider, useMCM)

	switch ng.Spec.NodeType {
	case v1.NodeTypeCloudEphemeral:
		return defaultEngine
	case v1.NodeTypeStatic:
		if ng.Spec.StaticInstances != nil {
			return engineCAPI
		}
		return engineNone
	default:
		return engineNone
	}
}

// defaultCloudEphemeralEngine mirrors
// get_crds.defaultCloudEphemeralNodeGroupEngineForNewNodeGroups. It reads
// machineClassKind/capiClusterKind from the decoded cloud-provider secret,
// which correspond to internal.cloudProvider.{machineClassKind,capiClusterKind}.
func defaultCloudEphemeralEngine(cloudProvider map[string]interface{}, useMCM bool) string {
	hasMCM := nonEmptyString(cloudProvider["machineClassKind"])
	hasCAPI := nonEmptyString(cloudProvider["capiClusterKind"])

	switch {
	case hasMCM && hasCAPI:
		if useMCM {
			return engineMCM
		}
		return engineCAPI
	case hasMCM:
		return engineMCM
	case hasCAPI:
		return engineCAPI
	default:
		return engineNone
	}
}

func nonEmptyString(v interface{}) bool {
	s, ok := v.(string)
	return ok && s != ""
}

// serializeLabels mirrors get_crds.serializeLabels.
func serializeLabels(ng *v1.NodeGroup) string {
	merged := make(map[string]string)
	if ng.Spec.NodeTemplate != nil {
		for k, v := range ng.Spec.NodeTemplate.Labels {
			merged[k] = v
		}
	}
	merged["node.deckhouse.io/group"] = ng.Name
	merged["node.deckhouse.io/type"] = string(ng.Spec.NodeType)
	merged["node-role.kubernetes.io/"+ng.Name] = ""
	return labels.FormatLabels(merged)
}

// serializeTaints mirrors get_crds.serializeTaints. Order is preserved (NOT
// sorted) to match get_crds byte-for-byte.
func serializeTaints(ng *v1.NodeGroup) string {
	if ng.Spec.NodeTemplate == nil || len(ng.Spec.NodeTemplate.Taints) == 0 {
		return ""
	}
	res := make([]string, 0, len(ng.Spec.NodeTemplate.Taints))
	for _, taint := range ng.Spec.NodeTemplate.Taints {
		res = append(res, taint.ToString())
	}
	return strings.Join(res, ",")
}

const epochWindowSize int64 = 4 * 60 * 60 // 4 hours

// calculateUpdateEpoch is a verbatim port of get_crds.calculateUpdateEpoch.
func calculateUpdateEpoch(ts int64, clusterUUID string, nodeGroupName string) string {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(clusterUUID))
	_, _ = hasher.Write([]byte(nodeGroupName))
	drift := int64(hasher.Sum64() % uint64(epochWindowSize))

	if ts <= drift {
		return strconv.FormatInt(drift, 10)
	}

	absWindowStart := ((ts - drift - 1) / epochWindowSize) * epochWindowSize
	epoch := absWindowStart + epochWindowSize + drift
	return strconv.FormatInt(epoch, 10)
}

// effectiveKubernetesVersion mirrors get_crds: the target version capped at the
// control-plane minimum (nodes must not be above the control plane).
func effectiveKubernetesVersion(target, controlPlaneMin *semver.Version) *semver.Version {
	effective := target
	if controlPlaneMin != nil {
		if effective == nil || effective.GreaterThan(controlPlaneMin) {
			effective = controlPlaneMin
		}
	}
	return effective
}

// semverMajMin mirrors hooks/util.semverMajMin.
func semverMajMin(ver *semver.Version) string {
	if ver == nil {
		return ""
	}
	return fmt.Sprintf("%d.%d", ver.Major(), ver.Minor())
}

// resolveCRIType mirrors the CRI resolution block in get_crds.
func resolveCRIType(ng *v1.NodeGroup, effectiveKubeVer *semver.Version, defaultCRI string) (string, error) {
	v1_19_0, _ := semver.NewVersion("1.19.0")
	defaultCRIType := nodeGroupDefaultCRIType
	if effectiveKubeVer != nil && effectiveKubeVer.LessThan(v1_19_0) {
		defaultCRIType = criTypeDocker
	}
	if defaultCRI != "" {
		defaultCRIType = defaultCRI
	}

	newCRIType := ""
	if ng.Spec.CRI != nil {
		newCRIType = string(ng.Spec.CRI.Type)
	}
	if newCRIType == "" {
		newCRIType = defaultCRIType
	}

	switch newCRIType {
	case criTypeDocker:
		if ng.Spec.CRI != nil && ng.Spec.CRI.Docker != nil && ng.Spec.CRI.Docker.Manage != nil && !*ng.Spec.CRI.Docker.Manage {
			newCRIType = criTypeNotManaged
		}
	case criTypeContainerd:
		if effectiveKubeVer != nil && effectiveKubeVer.LessThan(v1_19_0) {
			return "", fmt.Errorf("cri type Containerd is allowed only for kubernetes 1.19+")
		}
	}
	return newCRIType, nil
}

// resolveZones mirrors get_crds: spec zones if set, otherwise default zones.
func resolveZones(ng *v1.NodeGroup, defaultZones []string) []string {
	if ng.Spec.CloudInstances != nil && ng.Spec.CloudInstances.Zones != nil {
		return ng.Spec.CloudInstances.Zones
	}
	return defaultZones
}

// calculateNodeCapacity mirrors get_crds check #3 (scale-from-zero capacity).
func calculateNodeCapacity(kind string, instanceClassSpec interface{}, catalog *capacity.InstanceTypesCatalog) (*capacity.InstanceType, error) {
	return capacity.CalculateNodeTemplateCapacity(kind, instanceClassSpec, catalog)
}

// applyCloudSpecificDefaults mirrors get_crds.applyCloudSpecificDefaults. The
// only registered filler is vsphere's mainNetwork.
func applyCloudSpecificDefaults(cloudProvider map[string]interface{}, instanceClassSpec interface{}) (interface{}, error) {
	specMap, ok := instanceClassSpec.(map[string]interface{})
	if !ok {
		return instanceClassSpec, nil
	}

	providerName, _ := cloudProvider["type"].(string)
	providerName = strings.ToLower(providerName)

	raw, ok := cloudProvider[providerName]
	if !ok {
		return specMap, nil
	}
	cloudVariables, ok := raw.(map[string]interface{})
	if !ok {
		return specMap, nil
	}

	for _, fillFn := range fillCloudSpecificDefaults[providerName] {
		if err := fillFn(cloudVariables, specMap); err != nil {
			return nil, fmt.Errorf("fill %s defaults: %w", providerName, err)
		}
	}
	return specMap, nil
}

type cloudFillerFunc func(cloudVariables map[string]interface{}, instanceClass map[string]interface{}) error

var fillCloudSpecificDefaults = map[string][]cloudFillerFunc{
	"vsphere": {
		fillVsphereMainNetwork,
	},
}

// fillVsphereMainNetwork is a verbatim port of get_crds.fillVsphereMainNewtork.
func fillVsphereMainNetwork(cloudVariables map[string]interface{}, instanceClass map[string]interface{}) error {
	if _, ok := instanceClass["mainNetwork"]; ok {
		return nil
	}
	instancesRaw, ok := cloudVariables["instances"]
	if !ok {
		return nil
	}
	instancesMap, ok := instancesRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("cloudVariables.instances: expected map[string]interface{}, got %T", instancesRaw)
	}
	val, ok := instancesMap["mainNetwork"]
	if !ok {
		return nil
	}
	mn, ok := val.(string)
	if !ok {
		return fmt.Errorf("instances.mainNetwork: expected string, got %T", val)
	}
	instanceClass["mainNetwork"] = mn
	return nil
}
