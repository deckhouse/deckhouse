/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/Masterminds/semver/v3"
	cljson "github.com/clarketm/json"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/go_lib/hooks/set_cr_statuses"
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/autoscaler/capacity"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const (
	CRITypeDocker           = "Docker"
	CRITypeContainerd       = "Containerd"
	NodeGroupDefaultCRIType = CRITypeContainerd

	errorStatusField       = "error"
	kubeVersionStatusField = "kubernetesVersion"
)

type InstanceClassCrdInfo struct {
	Name string
	Spec interface{}
}

func applyInstanceClassCrdFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return InstanceClassCrdInfo{
		Name: obj.GetName(),
		Spec: obj.Object["spec"],
	}, nil
}

type NodeGroupCrdInfo struct {
	Name            string
	Spec            ngv1.NodeGroupSpec
	ManualRolloutID string
}

// applyNodeGroupCrdFilter returns name, spec and manualRolloutID from the NodeGroup
func applyNodeGroupCrdFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var nodeGroup ngv1.NodeGroup
	err := sdk.FromUnstructured(obj, &nodeGroup)
	if err != nil {
		return nil, err
	}

	return NodeGroupCrdInfo{
		Name:            nodeGroup.GetName(),
		Spec:            nodeGroup.Spec,
		ManualRolloutID: nodeGroup.GetAnnotations()["manual-rollout-id"],
	}, nil
}

type MachineDeploymentCrdInfo struct {
	Name string
	Zone string
}

func applyMachineDeploymentCrdFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return MachineDeploymentCrdInfo{
		Name: obj.GetName(),
		Zone: obj.GetAnnotations()["zone"],
	}, nil
}

func applyCloudProviderSecretKindZonesFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secretData, err := decodeDataFromSecret(obj)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"instanceClassKind": secretData["instanceClassKind"],
		"zones":             secretData["zones"],
	}, nil
}

func applyInstanceTypesCatalog(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	c := v1alpha1.InstanceTypesCatalog{}

	err := sdk.FromUnstructured(obj, &c)
	if err != nil {
		return nil, err
	}

	return capacity.NewInstanceTypesCatalog(c.InstanceTypes), nil
}

var getCRDsHookConfig = &go_hook.HookConfig{
	Queue:        "/modules/node-manager",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		// A binding with dynamic kind has index 0 for simplicity.
		{
			Name:       "ics",
			ApiVersion: "",
			Kind:       "",
			FilterFunc: applyInstanceClassCrdFilter,
		},
		{
			Name:       "ngs",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: applyNodeGroupCrdFilter,
		},
		{
			Name:       "machine_deployments",
			ApiVersion: "machine.sapcloud.io/v1alpha1",
			Kind:       "MachineDeployment",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "heritage",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"deckhouse"},
					},
				},
			},
			FilterFunc: applyMachineDeploymentCrdFilter,
		},
		// kube-system/Secret/d8-node-manager-cloud-provider
		{
			Name:       "cloud_provider_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-node-manager-cloud-provider"},
			},
			FilterFunc: applyCloudProviderSecretKindZonesFilter,
		},
		{
			Name:       "instance_types_catalog",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "InstanceTypesCatalog",
			NameSelector: &types.NameSelector{
				MatchNames: []string{v1alpha1.CloudDiscoveryDataResourceName},
			},
			FilterFunc: applyInstanceTypesCatalog,
		},
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "sync",
			Crontab: "*/10 * * * *",
		},
	},
}

var _ = sdk.RegisterFunc(getCRDsHookConfig, getCRDsHandler)

func getCRDsHandler(input *go_hook.HookInput) error {
	// Detect InstanceClass kind and change binding if needed.
	kindInUse, kindFromSecret := detectInstanceClassKind(input, getCRDsHookConfig)

	// Kind is changed, so objects in "dynamic-kind" can be ignored. Update kind and stop the hook.
	if kindInUse != kindFromSecret {
		if kindFromSecret == "" {
			input.LogEntry.Infof("InstanceClassKind has changed from '%s' to '': disable binding 'ics'", kindInUse)
			*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
				Name:       "ics",
				Action:     "Disable",
				Kind:       "",
				ApiVersion: "",
			})
		} else {
			input.LogEntry.Infof("InstanceClassKind has changed from '%s' to '%s': update kind for binding 'ics'", kindInUse, kindFromSecret)
			*input.BindingActions = append(*input.BindingActions, go_hook.BindingAction{
				Name:   "ics",
				Action: "UpdateKind",
				Kind:   kindFromSecret,
				// TODO Set apiVersion to exact value? Should it be in a Secret?
				// ApiVersion: "deckhouse.io/v1alpha1",
				ApiVersion: "",
			})
		}
		// Save new kind as current kind.
		getCRDsHookConfig.Kubernetes[0].Kind = kindFromSecret
		// Binding changed, hook will be restarted with new objects in "ics" snapshot.
		return nil
	}

	// TODO What should we do with a broken semver?
	// Read kubernetes version either from clusterConfiguration or from discovery.
	var globalTargetKubernetesVersion *semver.Version
	var err error

	versionValue, has := input.Values.GetOk("global.discovery.kubernetesVersion")
	if has {
		globalTargetKubernetesVersion, err = semver.NewVersion(versionValue.String())
		if err != nil {
			return fmt.Errorf("global.discovery.kubernetesVersion contains a malformed semver: %s: %v", versionValue.String(), err)
		}
	}

	versionValue, has = input.Values.GetOk("global.clusterConfiguration.kubernetesVersion")
	if has {
		globalTargetKubernetesVersion, err = semver.NewVersion(versionValue.String())
		if err != nil {
			return fmt.Errorf("global.clusterConfiguration.kubernetesVersion contains a malformed semver: %s: %v", versionValue.String(), err)
		}
	}

	controlPlaneKubeVersions := make([]*semver.Version, 0)
	if input.Values.Exists("global.discovery.kubernetesVersions") {
		for _, verItem := range input.Values.Get("global.discovery.kubernetesVersions").Array() {
			ver, _ := semver.NewVersion(verItem.String())
			controlPlaneKubeVersions = append(controlPlaneKubeVersions, ver)
		}
	}

	controlPlaneMinVersion := semverMin(controlPlaneKubeVersions)

	// Default zones. Take them from input.Snapshots["machine_deployments"]
	// and from input.Snapshots["cloud_provider_secret"].zones
	defaultZones := set.New()
	for _, machineInfoItem := range input.Snapshots["machine_deployments"] {
		machineInfo := machineInfoItem.(MachineDeploymentCrdInfo)
		defaultZones.Add(machineInfo.Zone)
	}
	if len(input.Snapshots["cloud_provider_secret"]) > 0 {
		secretInfo := input.Snapshots["cloud_provider_secret"][0].(map[string]interface{})
		zonesUntyped := secretInfo["zones"]

		switch v := zonesUntyped.(type) {
		case []string:
			defaultZones.Add(v...)
		case []interface{}:
			for _, zoneUntyped := range v {
				if s, ok := zoneUntyped.(string); ok {
					defaultZones.Add(s)
				}
			}
		case string:
			defaultZones.Add(v)
		}
	}

	// Save timestamp for updateEpoch.
	timestamp := epochTimestampAccessor()

	finalNodeGroups := make([]interface{}, 0)

	// Expire node_group_info metric.
	input.MetricsCollector.Expire("")

	iCatalogRaw := input.Snapshots["instance_types_catalog"]
	var instanceTypeCatalog *capacity.InstanceTypesCatalog

	if len(iCatalogRaw) == 1 {
		instanceTypeCatalog = iCatalogRaw[0].(*capacity.InstanceTypesCatalog)
	} else {
		instanceTypeCatalog = capacity.NewInstanceTypesCatalog(nil)
	}

	for _, v := range input.Snapshots["ngs"] {
		nodeGroup := v.(NodeGroupCrdInfo)
		ngForValues := nodeGroupForValues(nodeGroup.Spec.DeepCopy())
		// set observed status fields
		input.PatchCollector.Filter(set_cr_statuses.SetObservedStatus(v, applyNodeGroupCrdFilter), "deckhouse.io/v1", "nodegroup", "", nodeGroup.Name, object_patch.WithSubresource("/status"), object_patch.IgnoreHookError())
		// Copy manualRolloutID and name.
		ngForValues["name"] = nodeGroup.Name
		ngForValues["manualRolloutID"] = nodeGroup.ManualRolloutID

		if nodeGroup.Spec.NodeType == ngv1.NodeTypeStatic {
			if staticValue, has := input.Values.GetOk("nodeManager.internal.static"); has {
				if len(staticValue.Map()) > 0 {
					ngForValues["static"] = staticValue.Value()
				}
			}
		}

		if nodeGroup.Spec.NodeType == ngv1.NodeTypeCloudEphemeral && kindInUse != "" {
			instanceClasses := make(map[string]interface{})

			input.LogEntry.Errorf("ICSSSSS  %+v", input.Snapshots["ics"])
			for _, icsItem := range input.Snapshots["ics"] {
				ic := icsItem.(InstanceClassCrdInfo)
				instanceClasses[ic.Name] = ic.Spec
			}

			// check #1 — .spec.cloudInstances.classReference.kind should be allowed in our cluster
			nodeGroupInstanceClassKind := nodeGroup.Spec.CloudInstances.ClassReference.Kind
			if nodeGroupInstanceClassKind != kindInUse {
				errorMsg := fmt.Sprintf("Wrong classReference: Kind %s is not allowed, the only allowed kind is %s.", nodeGroupInstanceClassKind, kindInUse)

				if input.Values.Exists("nodeManager.internal.nodeGroups") {
					savedNodeGroups := input.Values.Get("nodeManager.internal.nodeGroups").Array()
					for _, savedNodeGroup := range savedNodeGroups {
						ng := savedNodeGroup.Map()
						if ng["name"].String() == nodeGroup.Name {
							finalNodeGroups = append(finalNodeGroups, savedNodeGroup.Value().(map[string]interface{}))
							errorMsg += " Earlier stored version of NG is in use now!"
						}
					}
				}

				input.LogEntry.Errorf("Bad NodeGroup '%s': %s", nodeGroup.Name, errorMsg)
				setNodeGroupStatus(input.PatchCollector, nodeGroup.Name, errorStatusField, errorMsg)
				continue
			}

			// check #2 — .spec.cloudInstances.classReference should be valid
			nodeGroupInstanceClassName := nodeGroup.Spec.CloudInstances.ClassReference.Name
			isKnownClassName := false
			input.LogEntry.Errorf("ICSSS %+v", instanceClasses)
			for className := range instanceClasses {
				if className == nodeGroupInstanceClassName {
					isKnownClassName = true
					break
				}
			}
			if !isKnownClassName {
				errorMsg := fmt.Sprintf("Wrong classReference: There is no valid instance class %s of type %s.", nodeGroupInstanceClassName, nodeGroupInstanceClassKind)

				if input.Values.Exists("nodeManager.internal.nodeGroups") {
					savedNodeGroups := input.Values.Get("nodeManager.internal.nodeGroups").Array()
					for _, savedNodeGroup := range savedNodeGroups {
						ng := savedNodeGroup.Map()
						if ng["name"].String() == nodeGroup.Name {
							finalNodeGroups = append(finalNodeGroups, savedNodeGroup.Value().(map[string]interface{}))
							errorMsg += " Earlier stored version of NG is in use now!"
						}
					}
				}

				input.LogEntry.Errorf("Bad NodeGroup '%s': %s", nodeGroup.Name, errorMsg)
				setNodeGroupStatus(input.PatchCollector, nodeGroup.Name, errorStatusField, errorMsg)
				continue
			}

			// check #3 - node capacity planning: scale from zero check
			instanceClassSpec := instanceClasses[nodeGroupInstanceClassName]
			if nodeGroup.Spec.CloudInstances.MinPerZone != nil && nodeGroup.Spec.CloudInstances.MaxPerZone != nil {
				if *nodeGroup.Spec.CloudInstances.MinPerZone == 0 && *nodeGroup.Spec.CloudInstances.MaxPerZone > 0 {
					// capacity calculation required only for scaling from zero, we can save some time in the other cases
					nodeCapacity, err := capacity.CalculateNodeTemplateCapacity(nodeGroupInstanceClassKind, instanceClassSpec, instanceTypeCatalog)
					if err != nil {
						input.LogEntry.Errorf("Calculate capacity failed for: %s with spec: %v. Error: %s", nodeGroupInstanceClassKind, instanceClassSpec, err)
						setNodeGroupStatus(input.PatchCollector, nodeGroup.Name, errorStatusField, fmt.Sprintf("%s capacity is not set and instance type could not be found in the built-it types. ScaleFromZero would not work until you set a capacity spec into the %s/%s", nodeGroupInstanceClassKind, nodeGroupInstanceClassKind, nodeGroup.Spec.CloudInstances.ClassReference.Name))
						continue
					}

					ngForValues["nodeCapacity"] = nodeCapacity
				}
			}

			// check #4 — zones should be valid
			if len(defaultZones) > 0 {
				// All elements in nodeGroup.Spec.CloudInstances.Zones
				// should contain in defaultZonesMap.
				containCount := 0
				unknownZones := make([]string, 0)
				for _, zone := range nodeGroup.Spec.CloudInstances.Zones {
					if defaultZones.Has(zone) {
						containCount++
					} else {
						unknownZones = append(unknownZones, zone)
					}
				}
				if containCount != len(nodeGroup.Spec.CloudInstances.Zones) {
					errorMsg := fmt.Sprintf("unknown cloudInstances.zones: %v", unknownZones)
					input.LogEntry.Errorf("Bad NodeGroup '%s': %s", nodeGroup.Name, errorMsg)

					setNodeGroupStatus(input.PatchCollector, nodeGroup.Name, errorStatusField, errorMsg)
					continue
				}
			}

			// Put instanceClass.spec into values.
			ngForValues["instanceClass"] = instanceClassSpec

			var zones []string
			if nodeGroup.Spec.CloudInstances.Zones != nil {
				zones = nodeGroup.Spec.CloudInstances.Zones
			}
			if zones == nil {
				zones = defaultZones.Slice()
			}

			if ngForValues["cloudInstances"] == nil {
				ngForValues["cloudInstances"] = ngv1.CloudInstances{}
			}
			cloudInstances := ngForValues["cloudInstances"].(ngv1.CloudInstances)
			cloudInstances.Zones = zones
			ngForValues["cloudInstances"] = cloudInstances
		}

		// Determine effective Kubernetes version.
		effectiveKubeVer := globalTargetKubernetesVersion
		if controlPlaneMinVersion != nil {
			if effectiveKubeVer == nil || effectiveKubeVer.GreaterThan(controlPlaneMinVersion) {
				// Nodes should not be above control plane
				effectiveKubeVer = controlPlaneMinVersion
			}
		}
		effectiveKubeVerMajMin := semverMajMin(effectiveKubeVer)
		ngForValues[kubeVersionStatusField] = effectiveKubeVerMajMin

		setNodeGroupStatus(input.PatchCollector, nodeGroup.Name, kubeVersionStatusField, effectiveKubeVerMajMin)

		// Detect CRI type. Default CRI type is 'Docker' for Kubernetes version less than 1.19.
		v1_19_0, _ := semver.NewVersion("1.19.0")
		defaultCRIType := NodeGroupDefaultCRIType
		if effectiveKubeVer.LessThan(v1_19_0) {
			defaultCRIType = CRITypeDocker
		}

		if criValue, has := input.Values.GetOk("global.clusterConfiguration.defaultCRI"); has {
			defaultCRIType = criValue.String()
		}

		newCRIType := nodeGroup.Spec.CRI.Type
		if newCRIType == "" {
			newCRIType = defaultCRIType
		}

		switch newCRIType {
		case CRITypeDocker:
			// cri is NotManaged if .spec.cri.docker.manage is explicitly set to false.
			if nodeGroup.Spec.CRI.Docker != nil && nodeGroup.Spec.CRI.Docker.Manage != nil && !*nodeGroup.Spec.CRI.Docker.Manage {
				newCRIType = "NotManaged"
			}
		case CRITypeContainerd:
			// Containerd requires Kubernetes version 1.19+.
			if effectiveKubeVer.LessThan(v1_19_0) {
				return fmt.Errorf("cri type Containerd is allowed only for kubernetes 1.19+")
			}
		}

		if ngForValues["cri"] == nil {
			ngForValues["cri"] = ngv1.CRI{}
		}
		cri := ngForValues["cri"].(ngv1.CRI)
		cri.Type = newCRIType
		ngForValues["cri"] = cri

		// Calculate update epoch
		// updateEpoch is a value that changes every 4 hour for a particular NodeGroup in the cluster.
		// Values are spread over 4 hour window to update nodes at different times.
		// Also, updateEpoch value is a unix time of the next update.
		updateEpoch := calculateUpdateEpoch(timestamp,
			input.Values.Get("global.discovery.clusterUUID").String(),
			nodeGroup.Name)
		ngForValues["updateEpoch"] = updateEpoch

		// Reset status error for current NodeGroup.
		setNodeGroupStatus(input.PatchCollector, nodeGroup.Name, errorStatusField, "")

		ngBytes, _ := cljson.Marshal(ngForValues)
		finalNodeGroups = append(finalNodeGroups, json.RawMessage(ngBytes))

		input.MetricsCollector.Set("node_group_info", 1, map[string]string{
			"name":     nodeGroup.Name,
			"cri_type": newCRIType,
		})
	}

	if !input.Values.Exists("nodeManager.internal") {
		input.Values.Set("nodeManager.internal", map[string]interface{}{})
	}
	input.Values.Set("nodeManager.internal.nodeGroups", finalNodeGroups)
	return nil
}

func nodeGroupForValues(nodeGroupSpec *ngv1.NodeGroupSpec) map[string]interface{} {
	res := make(map[string]interface{})

	res["nodeType"] = nodeGroupSpec.NodeType
	if !nodeGroupSpec.CRI.IsEmpty() {
		res["cri"] = nodeGroupSpec.CRI
	}
	if nodeGroupSpec.StaticInstances != nil {
		res["staticInstances"] = *nodeGroupSpec.StaticInstances
	}
	if !nodeGroupSpec.CloudInstances.IsEmpty() {
		res["cloudInstances"] = nodeGroupSpec.CloudInstances
	}
	if !nodeGroupSpec.NodeTemplate.IsEmpty() {
		res["nodeTemplate"] = nodeGroupSpec.NodeTemplate
	}
	if !nodeGroupSpec.Chaos.IsEmpty() {
		res["chaos"] = nodeGroupSpec.Chaos
	}
	if !nodeGroupSpec.OperatingSystem.IsEmpty() {
		res["operatingSystem"] = nodeGroupSpec.OperatingSystem
	}
	if !nodeGroupSpec.Disruptions.IsEmpty() {
		res["disruptions"] = nodeGroupSpec.Disruptions
	}
	if !nodeGroupSpec.Kubelet.IsEmpty() {
		res["kubelet"] = nodeGroupSpec.Kubelet
	}

	return res
}

var epochTimestampAccessor = func() int64 {
	return time.Now().Unix()
}

var detectInstanceClassKind = func(input *go_hook.HookInput, config *go_hook.HookConfig) (inUse string, fromSecret string) {
	if len(input.Snapshots["cloud_provider_secret"]) > 0 {
		if secretInfo, ok := input.Snapshots["cloud_provider_secret"][0].(map[string]interface{}); ok {
			if kind, ok := secretInfo["instanceClassKind"].(string); ok {
				fromSecret = kind
			}
		}
	}

	return config.Kubernetes[0].Kind, fromSecret
}

const EpochWindowSize int64 = 4 * 60 * 60 // 4 hours
// calculateUpdateEpoch returns an end point of the drifted 4 hour window for given cluster and timestamp.
//
// epoch is the unix timestamp of an end time of the drifted 4 hour window.
//
//	A0---D0---------------------A1---D1---------------------A2---D2-----
//	A - points for windows in absolute time
//	D - points for drifted windows
//
// Epoch for timestamps A0 <= ts <= D0 is D0
//
// Epoch for timestamps D0 < ts <= D1 is D1
func calculateUpdateEpoch(ts int64, clusterUUID string, nodeGroupName string) string {
	hasher := fnv.New64a()
	// error is always nil here
	_, _ = hasher.Write([]byte(clusterUUID))
	_, _ = hasher.Write([]byte(nodeGroupName))
	drift := int64(hasher.Sum64() % uint64(EpochWindowSize))

	// Near zero timestamps. It should not happen, isn't it?
	if ts <= drift {
		return strconv.FormatInt(drift, 10)
	}

	// Get the start of the absolute time window (non-drifted). Correct timestamp be 1 second
	// to get correct window start when timestamp is equal to the end of the drifted window.
	absWindowStart := ((ts - drift - 1) / EpochWindowSize) * EpochWindowSize
	epoch := absWindowStart + EpochWindowSize + drift
	return strconv.FormatInt(epoch, 10)
}
