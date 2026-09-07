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
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	ngcommon "github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
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
	criTypeDocker       = "Docker"
	criTypeContainerd   = "Containerd"
	criTypeContainerdV2 = "ContainerdV2"
	criTypeNotManaged   = "NotManaged"
)

func nodeGroupDefaultCRIType() string {
	if nodecommon.IsCSEEdition() {
		return criTypeContainerdV2
	}
	return criTypeContainerd
}

// epochTimestampAccessor mirrors get_crds.go; overridable in tests.
var epochTimestampAccessor = func() int64 {
	return time.Now().Unix()
}

// defaultEngine is the answer for a NodeGroup with no machines to look at: the cloud-provider
// capabilities plus the use-mcm annotation.
func defaultEngine(ng *v1.NodeGroup, reg CloudProviderRegistration) string {
	switch {
	case ng.Spec.NodeType == v1.NodeTypeCloudEphemeral:
		useMCM := ng.GetAnnotations()[useMCMAnnotation] != ""
		return defaultCloudEphemeralEngine(reg, useMCM)
	case ng.Spec.NodeType == v1.NodeTypeStatic && ng.Spec.StaticInstances != nil:
		return engineCAPI
	default:
		return engineNone
	}
}

func defaultCloudEphemeralEngine(reg CloudProviderRegistration, useMCM bool) string {
	hasMCM := reg.MachineClassKind != ""
	hasCAPI := reg.CAPIClusterKind != ""

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

// ResolveEngine returns the engine a NodeGroup runs on: status.engine once written, else the
// MachineDeployments it already has, else the provider default. The middle step keeps a group
// upgraded from before the CAPI migration on MCM instead of recreating its machines on CAPI.
func ResolveEngine(
	ctx context.Context, reader client.Reader, ng *v1.NodeGroup, reg CloudProviderRegistration,
) (string, error) {
	if !engineUndecided(ng, reg) {
		return engineFrom(ng, reg, machineDeployments{}), nil
	}

	live, err := findMachineDeployments(ctx, reader, ng.Name)
	if err != nil {
		return "", fmt.Errorf("resolve engine of NodeGroup %s: %w", ng.Name, err)
	}
	return engineFrom(ng, reg, live), nil
}

// machineDeployments reports which engines already have MachineDeployments for a NodeGroup.
type machineDeployments struct {
	MCM  bool
	CAPI bool
}

func engineFrom(ng *v1.NodeGroup, reg CloudProviderRegistration, live machineDeployments) string {
	if ng.Status.Engine != "" {
		return ng.Status.Engine
	}
	if ng.Spec.NodeType == v1.NodeTypeCloudEphemeral {
		if live.MCM {
			return engineMCM
		}
		if live.CAPI {
			return engineCAPI
		}
	}
	return defaultEngine(ng, reg)
}

// engineUndecided is true only when the default is a guess: no pin, a CloudEphemeral group, and a
// provider publishing both kinds. Everywhere else the answer follows from the spec alone.
func engineUndecided(ng *v1.NodeGroup, reg CloudProviderRegistration) bool {
	if ng.Status.Engine != "" || ng.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		return false
	}
	return reg.MachineClassKind != "" && reg.CAPIClusterKind != ""
}

// findMachineDeployments lists MCM first: a group that has any runs on MCM whatever else exists,
// so the CAPI list is paid only when MCM comes back empty.
func findMachineDeployments(ctx context.Context, reader client.Reader, ngName string) (machineDeployments, error) {
	mcm, err := ngcommon.ListMachineDeployments(ctx, reader, ngcommon.MCMMachineDeploymentGVK, ngName)
	if err != nil {
		return machineDeployments{}, err
	}
	if len(mcm.Items) > 0 {
		return machineDeployments{MCM: true}, nil
	}

	capi, err := ngcommon.ListMachineDeployments(ctx, reader, ngcommon.CAPIMachineDeploymentGVK, ngName)
	if err != nil {
		return machineDeployments{}, err
	}
	return machineDeployments{CAPI: len(capi.Items) > 0}, nil
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
	defaultCRIType := nodeGroupDefaultCRIType()
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

func applyCloudSpecificDefaults(reg CloudProviderRegistration, instanceClassSpec interface{}) (interface{}, error) {
	specMap, ok := instanceClassSpec.(map[string]interface{})
	if !ok {
		return instanceClassSpec, nil
	}
	if reg.CloudVariables == nil {
		return specMap, nil
	}

	providerName := strings.ToLower(reg.Type)
	for _, fillFn := range fillCloudSpecificDefaults[providerName] {
		if err := fillFn(reg.CloudVariables, specMap); err != nil {
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
